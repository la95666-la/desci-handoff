package chaintx

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// ReconcileApprovedV3LifecycleReceipt finalizes only the original active
// signed operator attempt. This is never a signer, broadcast or retry path.
// A successful receipt remains final_success until its canonical event is
// independently applied by the scanner; a confirmed revert releases only
// that attempt's nonce fence via the existing locked finalizer.
func ReconcileApprovedV3LifecycleReceipt(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID, submissionID uint64, ownerHash, governorHash common.Hash) (*entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 approved lifecycle reconciliation unavailable")
	if ctx == nil || database == nil || rpc == nil || projectID == 0 || submissionID == 0 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return nil, invalid
	}
	admitted, err := AdmitApprovedLockedProject(ctx, database, rpc, projectID, ownerHash, governorHash)
	if err != nil {
		return nil, invalid
	}
	var rows []entity.ChainTxSubmission
	read := database.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE id=? AND chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY LOWER(sender_address)=BINARY ?
		AND BINARY action IN ('ido_v3_activate','ido_v3_end','ido_v3_withdraw_platform_fee')
		AND active_slot=1 LIMIT 2`, submissionID, admitted.Binding.ChainID, projectID,
		strings.ToLower(admitted.Binding.ProxyAddress.Hex()), "0xfd712061ee08e070a930ecd45e935ae5d7f5445c").Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return nil, invalid
	}
	row := rows[0]
	if row.Action == "ido_v3_create" || !v3ReconcilableRow(row, admitted.Binding) ||
		!strings.EqualFold(row.SenderAddress, "0xfd712061ee08e070a930ecd45e935ae5d7f5445c") ||
		basechain.VerifyV3ActionCall(row.Action, projectID, row.CallData) != nil ||
		VerifyV3PersistedAppliedAttempt(row, admitted.Binding) != nil ||
		(row.State != constant.ChainTxStateSigned && row.State != constant.ChainTxStateSubmitted &&
			row.State != constant.ChainTxStateBroadcastUnknown && row.State != constant.ChainTxStateMinedSuccess &&
			row.State != constant.ChainTxStateMinedRevert) {
		return nil, invalid
	}
	audit, err := AuditV3PersistedReceipt(ctx, row, admitted.Binding, rpc, admitted.Confirmations)
	if err != nil {
		return nil, invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil || idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, ownerHash,
		governorHash, audit.Block, audit.BlockHash) != nil {
		return nil, invalid
	}
	// The original attempt's receipt block must be canonical even if the
	// current project later changed status. Never infer success from a DB row.
	block := new(big.Int).SetUint64(audit.Block)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 ||
		header.Hash() != audit.BlockHash {
		return nil, invalid
	}
	// A repeat admission detects a reorg, cursor halt or changed approval
	// during the provider reads; the finalizer repeats DB locks and nonce.
	again, err := AdmitApprovedLockedProject(ctx, database, rpc, projectID, ownerHash, governorHash)
	if err != nil || again.Binding != admitted.Binding || again.Runtime != admitted.Runtime ||
		again.CreationBlock != admitted.CreationBlock || again.Confirmations != admitted.Confirmations {
		return nil, invalid
	}
	var updated entity.ChainTxSubmission
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var finishErr error
		updated, finishErr = finalizeV3AdmittedLifecycleReceiptTx(tx, row, admitted.Binding,
			common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"), audit)
		return finishErr
	}); err != nil {
		return nil, invalid
	}
	return &updated, nil
}
