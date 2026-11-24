package chaintx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// ReconcileV3LifecycleReceipt is an INTERNAL narrow recovery path for an ALREADY signed
// operator lifecycle attempt. It cannot create an intent, sign, broadcast or
// resend. Success stays final_success pending a separately authenticated V3
// event/financial materializer. Failure becomes final_revert and releases only
// the same sender's active nonce fence. An authenticated reorg coordinator
// must later invalidate/reconcile previously finalized observations before
// treating them as durable. Unknown RPC results change NOTHING.
// A signed scanner admission is necessary but does not authorize a new send.
// The caller must supply independently release-pinned governance/operator;
// it cannot be sourced from an Admin request or candidate table. No public
// execution route may use this as authorization to send or apply finance.
type V3LifecycleRecoveryRPC interface {
	V3ReceiptSource
	idov3whitelist.PolicyRPC
}

func ReconcileV3LifecycleReceipt(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	key ed25519.PublicKey, manifest idov3whitelist.AdmissionManifest, signature []byte,
	governance idov3whitelist.GovernanceIdentity, operator common.Address, submissionID uint64) (*entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 lifecycle receipt reconciliation unavailable")
	if ctx == nil || database == nil || rpc == nil || submissionID == 0 || operator == (common.Address{}) ||
		!common.IsHexAddress(manifest.Governor) || governance.Governor != common.HexToAddress(manifest.Governor) {
		return nil, invalid
	}
	admitted, err := idov3whitelist.AdmitSignedProject(ctx, database, key, manifest, signature, time.Now())
	if err != nil {
		return nil, invalid
	}
	row := entity.ChainTxSubmission{}
	if database.WithContext(ctx).Where("id = ?", submissionID).Take(&row).Error != nil || !v3ReconcilableRow(row, admitted.Binding) ||
		row.Action == "ido_v3_create" ||
		!strings.EqualFold(row.SenderAddress, operator.Hex()) {
		return nil, invalid
	}
	if row.State == constant.ChainTxStateFinalSuccess || row.State == constant.ChainTxStateFinalRevert {
		// Previously stored finality is not everlasting canonicality; this path
		// cannot reaffirm it without a separately reviewed reorg materializer.
		return nil, invalid
	}
	audit, err := AuditV3PersistedReceipt(ctx, row, admitted.Binding, rpc, admitted.Confirmations)
	if err != nil {
		return nil, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID), ProxyAddress: admitted.Binding.ProxyAddress}
	if _, err := idov3whitelist.ReadLifecycleCallersAt(ctx, rpc, scope, admitted.Runtime,
		governance, operator, audit.Block, audit.BlockHash); err != nil {
		return nil, invalid
	}
	// Expiration or a changed binding during RPC must prevent the write. The
	// second admission is read-only; the transaction below rechecks the locked
	// row, reorg gate and exact signer fence before changing a byte.
	if _, err := idov3whitelist.AdmitSignedProject(ctx, database, key, manifest, signature, time.Now()); err != nil {
		return nil, invalid
	}
	stable, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(audit.Block))
	if err != nil || stable == nil || stable.Number == nil || !stable.Number.IsUint64() ||
		stable.Number.Uint64() != audit.Block || stable.Hash() != audit.BlockHash {
		return nil, invalid
	}
	var updated entity.ChainTxSubmission
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var finishErr error
		updated, finishErr = finalizeV3AdmittedLifecycleReceiptTx(tx, row, admitted.Binding, operator, audit)
		return finishErr
	})
	if err != nil {
		return nil, invalid
	}
	return &updated, nil
}

// The admitted RPC receipt is not sufficient without these same-transaction
// locks. Keep reorg -> cursor -> project -> submission -> sender ordering.
func finalizeV3AdmittedLifecycleReceiptTx(tx *gorm.DB, row entity.ChainTxSubmission,
	binding idov3whitelist.ExpectedProjectBinding, operator common.Address, audit V3ReceiptAudit) (entity.ChainTxSubmission, error) {
	var updated entity.ChainTxSubmission
	invalid := errors.New("V3 lifecycle receipt finalization unavailable")
	if tx == nil || chainreorg.LockNoActiveIncidentTx(tx, row.ChainID, row.ContractAddress) != nil ||
		LockV3RunningCursorTx(tx, binding) != nil {
		return updated, invalid
	}
	// Hold the same project identity and Set-operator commitment as the
	// signing/broadcast path before locking the signed attempt and sender.
	var project []struct {
		Status string `gorm:"column:status"`
	}
	read := tx.Raw(`SELECT p.status FROM projects p JOIN ido_v3_project_binding b
			ON b.project_id=p.project_id AND b.chain_id=p.chain_id
			AND BINARY b.proxy_address=BINARY p.contract_address
			AND BINARY b.contract_version=BINARY p.contract_version
			JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
			AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
			WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY p.funding_mode=BINARY 'capped_presale' AND p.onchain_locked=1
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, binding.ProjectID,
		binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()),
		strings.ToLower(binding.ProjectToken.Hex()), strings.ToLower(binding.PaymentToken.Hex()),
		strings.ToLower(operator.Hex())).Scan(&project)
	expected := map[string]string{"ido_v3_activate": "Pending", "ido_v3_end": "Active",
		"ido_v3_withdraw_platform_fee": "Successful"}[row.Action]
	if read.Error != nil || len(project) != 1 || expected == "" || project[0].Status != expected {
		return updated, invalid
	}
	return finalizeV3LifecycleReceiptLockedTx(tx, row, binding, operator, audit)
}

// Caller must recheck the independent receipt and runtime outside this
// transaction. All writes below are atomic under the shared sender fence.
func finalizeV3LifecycleReceiptTx(tx *gorm.DB, row entity.ChainTxSubmission,
	binding idov3whitelist.ExpectedProjectBinding, operator common.Address, audit V3ReceiptAudit) (entity.ChainTxSubmission, error) {
	var updated entity.ChainTxSubmission
	if tx == nil || chainreorg.LockNoActiveIncidentTx(tx, row.ChainID, row.ContractAddress) != nil {
		return updated, errors.New("V3 receipt finalization unavailable")
	}
	return finalizeV3LifecycleReceiptLockedTx(tx, row, binding, operator, audit)
}

// Called only after reorg gate and (for locked V3 projects) cursor; never
// reacquires a lower-order gate after the scanner cursor lock.
func finalizeV3LifecycleReceiptLockedTx(tx *gorm.DB, row entity.ChainTxSubmission,
	binding idov3whitelist.ExpectedProjectBinding, operator common.Address, audit V3ReceiptAudit) (entity.ChainTxSubmission, error) {
	var updated entity.ChainTxSubmission
	invalid := errors.New("V3 receipt finalization unavailable")
	if tx == nil || !v3ReconcilableRow(row, binding) || operator == (common.Address{}) ||
		!strings.EqualFold(row.SenderAddress, operator.Hex()) || audit.Hash == (common.Hash{}) ||
		audit.Block == 0 || audit.BlockHash == (common.Hash{}) || audit.Nonce != *row.Nonce ||
		audit.Hash.Hex() != *row.TxHash || basechain.VerifyV3ActionCall(row.Action, row.ProjectID, row.CallData) != nil {
		return updated, invalid
	}
	var current entity.ChainTxSubmission
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", row.ID).Take(&current).Error != nil ||
		!v3ReconcilableRow(current, binding) || !sameV3SignedAttempt(row, current) ||
		!strings.EqualFold(current.SenderAddress, operator.Hex()) ||
		basechain.VerifyV3ActionCall(current.Action, current.ProjectID, current.CallData) != nil {
		return updated, invalid
	}
	if current.State != constant.ChainTxStateSigned && current.State != constant.ChainTxStateSubmitted &&
		current.State != constant.ChainTxStateBroadcastUnknown && current.State != constant.ChainTxStateMinedSuccess &&
		current.State != constant.ChainTxStateMinedRevert {
		return updated, invalid
	}
	// A previously stored receipt is not authority to overwrite a different
	// canonical observation. In particular, an old mined_revert or a replaced
	// block cannot be converted to final_success by the ordinary receipt job.
	// Only the authenticated reorg coordinator may adjudicate that history.
	if !v3StoredReceiptAgrees(current, audit) {
		return updated, invalid
	}
	var fence entity.ChainRelayerNonce
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND sender_address = ?", current.ChainID, current.SenderAddress).Take(&fence).Error != nil ||
		fence.ActiveSubmissionID == nil || *fence.ActiveSubmissionID != current.ID || current.Nonce == nil ||
		(*current.Nonce == ^uint64(0)) || fence.NextNonce != *current.Nonce+1 {
		return updated, invalid
	}
	now := time.Now()
	state := constant.ChainTxStateFinalSuccess
	status := uint64(1)
	values := map[string]interface{}{"receipt_status": status, "receipt_block_number": audit.Block,
		"receipt_block_hash": audit.BlockHash.Hex(), "receipt_gas_used": audit.GasUsed,
		"finalized_at": now, "last_error": "", "updated_at": now}
	if audit.Reverted {
		state, status = constant.ChainTxStateFinalRevert, 0
		values["receipt_status"] = status
		values["active_slot"] = nil
	}
	values["state"] = state
	result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state IN ? AND active_slot = 1 AND tx_hash = ? AND nonce = ?", current.ID,
		[]string{constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown, constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert},
		*current.TxHash, *current.Nonce).Updates(values)
	if result.Error != nil || result.RowsAffected != 1 {
		return updated, invalid
	}
	if audit.Reverted && releaseSignerTx(tx, current, current.ID) != nil {
		return updated, invalid
	}
	if tx.Where("id = ?", current.ID).Take(&updated).Error != nil || updated.State != state ||
		updated.ReceiptStatus == nil || *updated.ReceiptStatus != status ||
		updated.ReceiptBlockNumber == nil || *updated.ReceiptBlockNumber != audit.Block ||
		!strings.EqualFold(updated.ReceiptBlockHash, audit.BlockHash.Hex()) {
		return entity.ChainTxSubmission{}, invalid
	}
	return updated, nil
}

func v3StoredReceiptAgrees(row entity.ChainTxSubmission, audit V3ReceiptAudit) bool {
	observed := row.ReceiptStatus != nil || row.ReceiptBlockNumber != nil || row.ReceiptBlockHash != "" ||
		row.ReceiptGasUsed != nil
	if !observed {
		return row.State != constant.ChainTxStateMinedSuccess && row.State != constant.ChainTxStateMinedRevert
	}
	status := uint64(1)
	if audit.Reverted {
		status = 0
	}
	return (row.State != constant.ChainTxStateMinedSuccess || status == 1) &&
		(row.State != constant.ChainTxStateMinedRevert || status == 0) &&
		row.ReceiptStatus != nil && *row.ReceiptStatus == status &&
		row.ReceiptBlockNumber != nil && *row.ReceiptBlockNumber == audit.Block &&
		strings.EqualFold(row.ReceiptBlockHash, audit.BlockHash.Hex()) &&
		(row.ReceiptGasUsed == nil || *row.ReceiptGasUsed == audit.GasUsed)
}

func v3ReconcilableRow(row entity.ChainTxSubmission, binding idov3whitelist.ExpectedProjectBinding) bool {
	// Creation is admitted only by the separate signed-Set recovery wrapper.
	// Wallet and Safe operations never enter this relayer path.
	if row.Action != "ido_v3_create" && row.Action != "ido_v3_activate" && row.Action != "ido_v3_end" && row.Action != "ido_v3_withdraw_platform_fee" {
		return false
	}
	return row.ID != 0 && row.ChainID == binding.ChainID && row.ProjectID == binding.ProjectID &&
		strings.EqualFold(row.ContractAddress, binding.ProxyAddress.Hex()) && row.ActiveSlot != nil && *row.ActiveSlot == 1 &&
		row.TxHash != nil && row.Nonce != nil && len(row.RawTx) != 0 &&
		baseV3ActionValid(row)
}

func baseV3ActionValid(row entity.ChainTxSubmission) bool {
	return row.SenderAddress != "" && common.IsHexAddress(row.SenderAddress) &&
		row.SenderAddress != common.Address{}.Hex() && len(row.CallData) > 0
}

func sameV3SignedAttempt(a, b entity.ChainTxSubmission) bool {
	return a.ID == b.ID && a.Action == b.Action && a.ProjectID == b.ProjectID && a.ChainID == b.ChainID &&
		strings.EqualFold(a.ContractAddress, b.ContractAddress) && strings.EqualFold(a.SenderAddress, b.SenderAddress) &&
		a.TxHash != nil && b.TxHash != nil && *a.TxHash == *b.TxHash && a.Nonce != nil && b.Nonce != nil && *a.Nonce == *b.Nonce &&
		bytes.Equal(a.RawTx, b.RawTx) && bytes.Equal(a.CallData, b.CallData)
}
