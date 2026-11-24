package chaintx

import (
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// ApplyV3CanonicalEventTx consumes an independently authenticated, successful
// canonical V3 event in the scanner's cursor/journal transaction. It is NOT a
// generic outbox entry point: it cannot sign, broadcast, or accept a caller's
// arbitrary event ID. The caller must hold its project and cursor locks and
// must have inserted the corresponding journal row in this transaction.
// An event without the same persisted transaction hash belongs to a
// separately authenticated external wallet/Safe channel, or a synthetic
// unregistered bootstrap fixture. Only an exact signed attempt may advance.
func ApplyV3CanonicalEventTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	action string, hash, blockHash common.Hash, block uint64, journalID uint64, requireAttempt bool) error {
	invalid := errors.New("V3 canonical outbox application unavailable")
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		block == 0 || hash == (common.Hash{}) || blockHash == (common.Hash{}) || journalID == 0 {
		return invalid
	}
	switch action {
	case "ido_v3_create", "ido_v3_activate", "ido_v3_end", "ido_v3_withdraw_platform_fee":
	default:
		return invalid
	}
	proxy := strings.ToLower(binding.ProxyAddress.Hex())
	kind := map[string]string{"ido_v3_create": "created", "ido_v3_activate": "ProjectActivated",
		"ido_v3_end": "ProjectEnded", "ido_v3_withdraw_platform_fee": "PlatformFeeWithdrawn"}[action]
	var journals []struct {
		ID uint64 `gorm:"column:id"`
	}
	if read := tx.Raw(`SELECT id FROM ido_v3_event_journal WHERE id=? AND chain_id=? AND proxy_address=?
		AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND block_hash=?
		AND tx_hash=? AND event_kind=? AND status='canonical' LIMIT 2`, journalID, binding.ChainID, proxy,
		binding.ProjectID, block, strings.ToLower(blockHash.Hex()), strings.ToLower(hash.Hex()), kind).Scan(&journals); read.Error != nil || len(journals) != 1 {
		return invalid
	}
	var attempts []entity.ChainTxSubmission
	if read := tx.Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY action=BINARY ? AND tx_hash=? LIMIT 2 FOR UPDATE`,
		binding.ChainID, binding.ProjectID, proxy, action, hash.Hex()).Scan(&attempts); read.Error != nil || len(attempts) > 1 {
		return invalid
	}
	if len(attempts) == 0 {
		if requireAttempt {
			return invalid
		}
		// A Safe/wallet event can be independent of an operator, but must
		// never silently supersede a signed or still-active relayer attempt
		// for the same action. The operator intent needs explicit handling.
		var conflicting []entity.ChainTxSubmission
		read := tx.Raw(`SELECT id FROM chain_tx_submission WHERE chain_id=? AND project_id=?
			AND BINARY contract_address=BINARY ? AND BINARY action=BINARY ? AND active_slot=1
			LIMIT 2 FOR UPDATE`, binding.ChainID, binding.ProjectID, proxy, action).Scan(&conflicting)
		if read.Error != nil || len(conflicting) > 1 {
			return invalid
		}
		if len(conflicting) == 1 {
			if conflicting[0].ID == 0 {
				return invalid
			}
			// A separate Safe event can supersede only an unsigned intent.
			// Signed bytes, even if not broadcast locally, must remain fenced.
			var row entity.ChainTxSubmission
			if tx.Raw(`SELECT * FROM chain_tx_submission WHERE id=? AND chain_id=? AND project_id=?
				AND BINARY contract_address=BINARY ? AND BINARY action=BINARY ? AND active_slot=1
				FOR UPDATE`, conflicting[0].ID, binding.ChainID, binding.ProjectID, proxy, action).Scan(&row).Error != nil || row.ID != conflicting[0].ID ||
				(row.State != constant.ChainTxStateIntent && row.State != constant.ChainTxStateReady) ||
				row.Nonce != nil || row.GasLimit != nil || len(row.RawTx) != 0 || row.TxHash != nil ||
				row.ActiveSlot == nil || *row.ActiveSlot != 1 {
				return invalid
			}
			changed := tx.Model(&entity.ChainTxSubmission{}).Where(`id=? AND active_slot=1 AND state IN ?
				AND nonce IS NULL AND gas_limit IS NULL AND raw_tx IS NULL AND tx_hash IS NULL`, row.ID,
				[]string{constant.ChainTxStateIntent, constant.ChainTxStateReady}).Updates(map[string]any{
				"state": constant.ChainTxStateCancelled, "active_slot": nil,
				"last_error": "superseded by canonical V3 external lifecycle event", "finalized_at": time.Now(),
			})
			if changed.Error != nil || changed.RowsAffected != 1 {
				return invalid
			}
		}
		return nil // independently signed Safe/wallet transaction
	}
	row := attempts[0]
	if row.State == constant.ChainTxStateApplied {
		if row.FinalEventRecordID != nil && *row.FinalEventRecordID == journalID && row.ActiveSlot == nil &&
			row.ReceiptStatus != nil && *row.ReceiptStatus == types.ReceiptStatusSuccessful &&
			row.ReceiptBlockNumber != nil && *row.ReceiptBlockNumber == block &&
			strings.EqualFold(row.ReceiptBlockHash, blockHash.Hex()) {
			return nil // exact canonical fact, never release sender twice
		}
		return invalid
	}
	var commitments []struct {
		Operator string `gorm:"column:operator_address"`
		Hash     string `gorm:"column:create_calldata_hash"`
	}
	if read := tx.Raw(`SELECT operator_address,create_calldata_hash FROM ido_v3_set_create_commitment
		WHERE project_id=? AND chain_id=? AND BINARY proxy_address=BINARY ?
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`, binding.ProjectID, binding.ChainID, proxy).Scan(&commitments); read.Error != nil || len(commitments) != 1 || commitments[0].Operator != strings.ToLower(row.SenderAddress) ||
		(action == "ido_v3_create" && commitments[0].Hash != strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex())) {
		return invalid
	}
	if row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.TxHash == nil || !strings.EqualFold(*row.TxHash, hash.Hex()) || row.Nonce == nil ||
		*row.Nonce == ^uint64(0) || row.GasLimit == nil || *row.GasLimit == 0 ||
		row.ValueRaw != "0" || len(row.RawTx) == 0 ||
		(row.State != constant.ChainTxStateSigned && row.State != constant.ChainTxStateSubmitted &&
			row.State != constant.ChainTxStateBroadcastUnknown && row.State != constant.ChainTxStateMinedSuccess &&
			row.State != constant.ChainTxStateFinalSuccess) ||
		basechain.VerifyV3ActionCall(action, binding.ProjectID, row.CallData) != nil ||
		validateRaw(row, row.RawTx, *row.TxHash, *row.Nonce) != nil {
		return invalid
	}
	if row.ReceiptStatus != nil && *row.ReceiptStatus != types.ReceiptStatusSuccessful ||
		row.ReceiptBlockNumber != nil && *row.ReceiptBlockNumber != block ||
		row.ReceiptBlockHash != "" && !strings.EqualFold(row.ReceiptBlockHash, blockHash.Hex()) ||
		row.State == constant.ChainTxStateFinalSuccess &&
			(row.ReceiptStatus == nil || row.ReceiptBlockNumber == nil || row.ReceiptBlockHash == "") {
		return invalid
	}
	var fences []entity.ChainRelayerNonce
	if read := tx.Raw(`SELECT * FROM chain_relayer_nonce WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? LIMIT 2 FOR UPDATE`,
		binding.ChainID, strings.ToLower(row.SenderAddress)).Scan(&fences); read.Error != nil || len(fences) != 1 ||
		fences[0].ActiveSubmissionID == nil || *fences[0].ActiveSubmissionID != row.ID ||
		fences[0].NextNonce != *row.Nonce+1 {
		return invalid
	}
	now := time.Now()
	update := tx.Model(&entity.ChainTxSubmission{}).Where(`id=? AND chain_id=? AND project_id=? AND action=? AND active_slot=1
		AND tx_hash=? AND nonce=? AND state IN ?`, row.ID, binding.ChainID, binding.ProjectID, action,
		*row.TxHash, *row.Nonce, []string{constant.ChainTxStateSigned, constant.ChainTxStateSubmitted,
			constant.ChainTxStateBroadcastUnknown, constant.ChainTxStateMinedSuccess, constant.ChainTxStateFinalSuccess}).Updates(map[string]interface{}{
		"state": constant.ChainTxStateApplied, "active_slot": nil, "receipt_status": uint64(types.ReceiptStatusSuccessful),
		"receipt_block_number": block, "receipt_block_hash": blockHash.Hex(), "final_event_record_id": journalID,
		"finalized_at": now, "updated_at": now,
	})
	if update.Error != nil || update.RowsAffected != 1 || releaseSignerTx(tx, row, row.ID) != nil {
		return invalid
	}
	return nil
}

// VerifyV3PersistedAppliedAttempt is read-only, requiring the exact original
// signed transaction and immutable V3 calldata; it never assigns a nonce.
func VerifyV3PersistedAppliedAttempt(row entity.ChainTxSubmission, binding idov3whitelist.ExpectedProjectBinding) error {
	invalid := errors.New("V3 persisted applied attempt unavailable")
	if row.ID == 0 || row.ChainID != binding.ChainID || row.ProjectID != binding.ProjectID ||
		!strings.EqualFold(row.ContractAddress, binding.ProxyAddress.Hex()) || row.TxHash == nil ||
		row.Nonce == nil || row.GasLimit == nil || *row.GasLimit == 0 || row.ValueRaw != "0" ||
		len(row.RawTx) == 0 || basechain.VerifyV3ActionCall(row.Action, binding.ProjectID, row.CallData) != nil ||
		validateRaw(row, row.RawTx, *row.TxHash, *row.Nonce) != nil {
		return invalid
	}
	return nil
}
