package chainevent

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func epochV3(value int64) string { return strconv.FormatInt(value, 10) }

// lockV3CreatedProjectTx runs only inside the confirmed creation bootstrap
// transaction. The event, receipt, pinned project getter and signed admission
// have already been checked by its caller. A project row is locked before the
// cursor/journal can become visible; neither a preflight nor a mined receipt
// alone changes its status. The Set commitment cannot be rewritten.
func lockV3CreatedProjectTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, created *commonchain.V3CreatedEvent, historicalSetExpiry ...int64) error {
	return lockV3CreatedProjectWithAdmissionTx(tx, binding, operator, created, false, historicalSetExpiry...)
}

func lockApprovedV3CreatedProjectTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, created *commonchain.V3CreatedEvent, setExpiry int64) error {
	return lockV3CreatedProjectWithAdmissionTx(tx, binding, operator, created, true, setExpiry)
}

func lockV3CreatedProjectWithAdmissionTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, created *commonchain.V3CreatedEvent, approved bool, historicalSetExpiry ...int64) error {
	invalid := errors.New("V3 creation lock unavailable")
	if tx == nil || operator == (common.Address{}) || operator == binding.ProxyAddress ||
		commonchain.MatchV3CreatedBinding(created, binding) != nil || created.ProjectOwner == (common.Address{}) ||
		created.FeeReceiver == (common.Address{}) || created.TxHash == (common.Hash{}) {
		return invalid
	}
	proxy := strings.ToLower(binding.ProxyAddress.Hex())
	// Follow the shared chain-write order: reorg gate, project, outbox,
	// sender nonce. Taking the submission/nonce locks before the project
	// deadlocks against Core's project-first chain-write transactions.
	if chainreorg.LockNoActiveIncidentTx(tx, binding.ChainID, proxy) != nil {
		return invalid
	}
	var rows []struct {
		ProjectID       uint64    `gorm:"column:project_id"`
		TokenDecimals   uint8     `gorm:"column:token_decimals"`
		PaymentDecimals uint8     `gorm:"column:payment_token_decimals"`
		FeeRate         uint16    `gorm:"column:success_fee_rate"`
		Price           string    `gorm:"column:token_price_raw"`
		Hard            string    `gorm:"column:hard_cap_raw"`
		Soft            string    `gorm:"column:soft_cap_raw"`
		Max             string    `gorm:"column:max_per_user_raw"`
		Start           time.Time `gorm:"column:start_time"`
		End             time.Time `gorm:"column:end_time"`
	}
	read := tx.Raw(`SELECT p.project_id,p.token_decimals,p.payment_token_decimals,p.success_fee_rate,
		p.token_price_raw,p.hard_cap_raw,p.soft_cap_raw,p.max_per_user_raw,p.start_time,p.end_time
		FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
		 AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=p.project_id AND c.chain_id=p.chain_id
		 AND BINARY c.proxy_address=BINARY p.contract_address AND BINARY c.contract_version=BINARY p.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		 AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		 AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		 AND BINARY p.owner_address=BINARY ? AND BINARY p.fee_receiver=BINARY ?
		 AND BINARY p.status=BINARY 'Set' AND p.onchain_locked=0 LIMIT 2 FOR UPDATE`,
		binding.ProjectID, binding.ChainID, proxy, strings.ToLower(binding.ProjectToken.Hex()),
		strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(created.ProjectOwner.Hex()),
		strings.ToLower(created.FeeReceiver.Hex())).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].ProjectID != binding.ProjectID ||
		rows[0].TokenDecimals != created.ProjectDecimals || rows[0].PaymentDecimals != created.PaymentDecimals ||
		rows[0].FeeRate != created.FeeRate || rows[0].Price != created.TokenPriceRaw ||
		rows[0].Hard != created.HardCapRaw || rows[0].Soft != created.SoftCapRaw ||
		rows[0].Max != created.MaxPerUserRaw {
		return invalid
	}
	startSecond, endSecond := rows[0].Start.Unix(), rows[0].End.Unix()
	if approved && len(historicalSetExpiry) == 1 && binding.ChainID == 56 &&
		strings.EqualFold(proxy, "0x0d4040875742975269c55e16c8884d3ee9510b8e") {
		// The approval-backed path may have a pre-fix DATETIME(0) rounding
		// difference. Reconstruct only the exact executed Set's reviewed seconds;
		// the original transaction/hash/nonce fence remains mandatory below.
		record, err := idov3whitelist.ReadApprovedSetRecord(tx.Statement.Context, tx, binding.ProjectID)
		if err != nil {
			return invalid
		}
		startSecond, endSecond, err = idov3whitelist.ApprovedStoredSaleSeconds(record, rows[0].Start, rows[0].End)
		if err != nil || startSecond != historicalSetExpiry[0] {
			return invalid
		}
	}
	if created.StartTimeRaw != epochV3(startSecond) || created.EndTimeRaw != epochV3(endSecond) {
		return invalid
	}
	// A matching ProjectCreated topic and economics do not prove that the
	// signed Set was executed by the reviewed operator. Require the exact
	// persisted create attempt and its original signed hash, never a replayed
	// or caller-supplied receipt. The scanner already checks that receipt.
	var attempts []entity.ChainTxSubmission
	if tx.Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY action=BINARY 'ido_v3_create'
		AND BINARY LOWER(sender_address)=BINARY ? AND tx_hash=? LIMIT 2 FOR UPDATE`,
		binding.ChainID, binding.ProjectID, proxy, strings.ToLower(operator.Hex()), created.TxHash.Hex()).Scan(&attempts).Error != nil || len(attempts) != 1 {
		return invalid
	}
	attempt := attempts[0]
	if len(historicalSetExpiry) > 1 || (len(historicalSetExpiry) == 1 &&
		!confirmedV3Creation(attempt, operator, created.TxHash, created.BlockNumber, created.BlockHash, historicalSetExpiry[0])) {
		return invalid
	}
	if attempt.Nonce == nil || *attempt.Nonce == ^uint64(0) || attempt.TxHash == nil || attempt.ActiveSlot == nil || *attempt.ActiveSlot != 1 ||
		attempt.ValueRaw != "0" || attempt.GasLimit == nil || *attempt.GasLimit == 0 ||
		attempt.State != constant.ChainTxStateSigned && attempt.State != constant.ChainTxStateSubmitted &&
			attempt.State != constant.ChainTxStateBroadcastUnknown && attempt.State != constant.ChainTxStateMinedSuccess &&
			attempt.State != constant.ChainTxStateFinalSuccess ||
		commonchain.VerifyV3ActionCall(attempt.Action, binding.ProjectID, attempt.CallData) != nil {
		return invalid
	}
	// A previously observed receipt must not contradict this canonical
	// creation. An unknown/broadcast state may have no receipt metadata, but
	// a mined/final-success claim needs a matching successful observation.
	observedReceipt := attempt.ReceiptStatus != nil || attempt.ReceiptBlockNumber != nil || attempt.ReceiptBlockHash != ""
	if observedReceipt && (attempt.ReceiptStatus == nil || *attempt.ReceiptStatus != types.ReceiptStatusSuccessful ||
		attempt.ReceiptBlockNumber == nil || *attempt.ReceiptBlockNumber != created.BlockNumber ||
		!strings.EqualFold(attempt.ReceiptBlockHash, created.BlockHash.Hex())) {
		return invalid
	}
	if (attempt.State == constant.ChainTxStateMinedSuccess || attempt.State == constant.ChainTxStateFinalSuccess) && !observedReceipt {
		return invalid
	}
	// The shared sender fence must still refer to the original create attempt.
	// A mined-looking row without its nonce ownership is not a bridge source.
	var fences []entity.ChainRelayerNonce
	if tx.Raw(`SELECT * FROM chain_relayer_nonce WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? LIMIT 2 FOR UPDATE`,
		binding.ChainID, strings.ToLower(attempt.SenderAddress)).Scan(&fences).Error != nil || len(fences) != 1 ||
		fences[0].ActiveSubmissionID == nil || *fences[0].ActiveSubmissionID != attempt.ID ||
		fences[0].NextNonce != *attempt.Nonce+1 {
		return invalid
	}
	var signed types.Transaction
	if signed.UnmarshalBinary(attempt.RawTx) != nil || signed.To() == nil || *signed.To() != binding.ProxyAddress ||
		signed.Hash() != created.TxHash || signed.Nonce() != *attempt.Nonce ||
		signed.ChainId().Cmp(new(big.Int).SetUint64(binding.ChainID)) != 0 || signed.Value().Sign() != 0 ||
		!strings.EqualFold(signed.To().Hex(), attempt.ContractAddress) ||
		!equalV3Bytes(signed.Data(), attempt.CallData) {
		return invalid
	}
	sender, err := types.Sender(types.LatestSignerForChainID(new(big.Int).SetUint64(binding.ChainID)), &signed)
	if err != nil || sender != operator {
		return invalid
	}
	var commitments []struct {
		Hash     string `gorm:"column:create_calldata_hash"`
		Operator string `gorm:"column:operator_address"`
	}
	if tx.Raw(`SELECT create_calldata_hash,operator_address FROM ido_v3_set_create_commitment WHERE project_id=?
		AND chain_id=? AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`,
		binding.ProjectID, binding.ChainID, proxy).Scan(&commitments).Error != nil ||
		len(commitments) != 1 || commitments[0].Hash != strings.ToLower(crypto.Keccak256Hash(attempt.CallData).Hex()) ||
		commitments[0].Operator != strings.ToLower(operator.Hex()) {
		return invalid
	}
	result := tx.Exec(`UPDATE projects SET status='Pending',onchain_locked=1 WHERE project_id=? AND chain_id=?
		AND BINARY contract_address=BINARY ? AND BINARY funding_mode=BINARY 'capped_presale'
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND BINARY status=BINARY 'Set' AND onchain_locked=0`,
		binding.ProjectID, binding.ChainID, proxy)
	if result.Error != nil || result.RowsAffected != 1 {
		return invalid
	}
	return nil
}

// bridgeV3Creation is invoked ONLY after independently signed pre-lock
// admission and a fixed-block runtime check by the registered scanner.
func bridgeV3Creation(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, admitted idov3whitelist.AdmittedProject,
	operator common.Address, scanExpiresAt, setExpiresAt int64, historical bool) error {
	if scanExpiresAt <= time.Now().Unix() || (!historical && setExpiresAt <= time.Now().Unix()) {
		return errors.New("V3 creation admission expired")
	}
	return v3BootstrapCreationWithLock(ctx, db, rpc, admitted.Binding, admitted.Runtime,
		admitted.CreationBlock, admitted.Confirmations,
		func(tx *gorm.DB, event *commonchain.V3CreatedEvent) error {
			// RPC checks can run across the signed decisions' expiry. Do not
			// lock a project with an expired Set or scan authorization.
			if scanExpiresAt <= time.Now().Unix() || (!historical && setExpiresAt <= time.Now().Unix()) {
				return errors.New("V3 creation admission expired")
			}
			if historical {
				return lockV3CreatedProjectTx(tx, admitted.Binding, operator, event, setExpiresAt)
			}
			return lockV3CreatedProjectTx(tx, admitted.Binding, operator, event)
		})
}

func equalV3Bytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
