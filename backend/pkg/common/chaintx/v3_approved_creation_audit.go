package chaintx

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// AuditApprovedV3Creation is a read-only, original-hash receipt check for a
// project-scoped executed Set. It does not change an outbox state, release a
// nonce, broadcast, or lock a project. A successful audit still needs the
// scanner's canonical event bridge before the project is on-chain locked.
func AuditApprovedV3Creation(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID, submissionID uint64, ownerCodeHash, governorCodeHash common.Hash) (V3ReceiptAudit, error) {
	invalid := errors.New("V3 approved creation receipt unavailable")
	if ctx == nil || database == nil || rpc == nil || projectID == 0 || submissionID == 0 ||
		ownerCodeHash == (common.Hash{}) || governorCodeHash == (common.Hash{}) {
		return V3ReceiptAudit{}, invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil {
		return V3ReceiptAudit{}, invalid
	}
	id := record.Identity
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: id.ChainID, ProjectID: projectID,
		ProxyAddress: common.HexToAddress(id.Proxy), ProjectToken: common.HexToAddress(id.ProjectToken),
		PaymentToken: common.HexToAddress(id.PaymentToken)}
	var rows []entity.ChainTxSubmission
	read := database.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE id=? AND chain_id=?
		AND project_id=? AND BINARY contract_address=BINARY ?
		AND BINARY action=BINARY 'ido_v3_create' AND BINARY LOWER(sender_address)=BINARY ? LIMIT 2`,
		submissionID, id.ChainID, projectID, id.Proxy, id.Operator).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return V3ReceiptAudit{}, invalid
	}
	row := rows[0]
	if approvedV3OriginalAttempt(row, binding, id) != nil ||
		(row.State != constant.ChainTxStateSigned && row.State != constant.ChainTxStateSubmitted &&
			row.State != constant.ChainTxStateBroadcastUnknown && row.State != constant.ChainTxStateMinedSuccess &&
			row.State != constant.ChainTxStateMinedRevert) {
		return V3ReceiptAudit{}, invalid
	}
	audit, err := AuditV3PersistedReceipt(ctx, row, binding, rpc, 15)
	if err != nil {
		return V3ReceiptAudit{}, invalid
	}
	// The confirmed receipt has its own event evidence. End-of-block fee or
	// token policy may have changed in a later transaction in the same block;
	// neither success nor revert recovery may demand that it still equals Set.
	if !audit.Reverted {
		receipt, e := rpc.TransactionReceipt(ctx, audit.Hash)
		if e != nil || approvedV3CreatedReceipt(receipt, binding, id, audit) != nil {
			return V3ReceiptAudit{}, invalid
		}
	}
	if idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, ownerCodeHash,
		governorCodeHash, audit.Block, audit.BlockHash) != nil {
		return V3ReceiptAudit{}, invalid
	}
	// The fee or token policy may change *after* a successful creation too.
	// Historical success is checked against the receipt block above; current
	// deployment identity, not current economics, must remain intact.
	head, e := rpc.HeaderByNumber(ctx, nil)
	if e != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 128805696+14 {
		return V3ReceiptAudit{}, invalid
	}
	height := head.Number.Uint64() - 14
	anchor, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if e != nil || anchor == nil || anchor.Number == nil || !anchor.Number.IsUint64() || anchor.Number.Uint64() != height ||
		idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, ownerCodeHash, governorCodeHash, height, anchor.Hash()) != nil {
		return V3ReceiptAudit{}, invalid
	}
	// The approval and exact transaction must still match after RPC reads.
	fresh, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil || fresh.Identity != id {
		return V3ReceiptAudit{}, invalid
	}
	return audit, nil
}

func approvedV3CreatedReceipt(receipt *types.Receipt, binding idov3whitelist.ExpectedProjectBinding,
	id idov3whitelist.ApprovedSetIdentity, audit V3ReceiptAudit) error {
	invalid := errors.New("V3 approved creation event unavailable")
	if id.ProjectID != binding.ProjectID || id.ChainID != binding.ChainID ||
		id.Proxy != strings.ToLower(binding.ProxyAddress.Hex()) ||
		id.ProjectToken != strings.ToLower(binding.ProjectToken.Hex()) ||
		id.PaymentToken != strings.ToLower(binding.PaymentToken.Hex()) ||
		receipt == nil || receipt.Status != types.ReceiptStatusSuccessful || receipt.TxHash != audit.Hash ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != audit.Block ||
		receipt.BlockHash != audit.BlockHash {
		return invalid
	}
	var created *basechain.V3CreatedEvent
	for _, log := range receipt.Logs {
		if log == nil || log.Address != binding.ProxyAddress || log.Removed || len(log.Topics) == 0 ||
			log.Topics[0] != crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")) {
			continue
		}
		item, e := basechain.DecodeV3CreatedLog(*log, binding.ChainID, binding.ProxyAddress)
		if e != nil || item.ProjectID != binding.ProjectID || created != nil {
			return invalid
		}
		created = item
	}
	if created == nil || basechain.MatchV3CreatedReceipt(created, binding, receipt) != nil ||
		created.ProjectOwner != common.HexToAddress(id.ProjectOwner) ||
		created.FeeReceiver != common.HexToAddress(id.FeeReceiver) ||
		created.ProjectDecimals != id.TokenDecimals || created.PaymentDecimals != id.PaymentDecimals ||
		created.FeeRate != id.FeeRate ||
		created.StartTimeRaw != new(big.Int).SetInt64(id.CreateDeadline).String() {
		return invalid
	}
	return nil
}

// No persisted row can supply its own project identity or a signing deadline.
// Both are independently reconstructed from executed four-eyes evidence.
func approvedV3OriginalAttempt(row entity.ChainTxSubmission,
	binding idov3whitelist.ExpectedProjectBinding, id idov3whitelist.ApprovedSetIdentity) error {
	invalid := errors.New("V3 approved original attempt unavailable")
	if row.Action != "ido_v3_create" || id.ProjectID != binding.ProjectID ||
		id.ChainID != binding.ChainID || id.Proxy != strings.ToLower(binding.ProxyAddress.Hex()) ||
		id.ProjectToken != strings.ToLower(binding.ProjectToken.Hex()) ||
		id.PaymentToken != strings.ToLower(binding.PaymentToken.Hex()) ||
		!v3ReconcilableRow(row, binding) || row.ActiveSlot == nil || *row.ActiveSlot != 1 ||
		!strings.EqualFold(row.SenderAddress, id.Operator) ||
		strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) != id.CreateCallHash ||
		basechain.VerifyV3ActionCall(row.Action, binding.ProjectID, row.CallData) != nil ||
		!V3SignedBeforeSet(row, id.CreateDeadline) || VerifyV3PersistedAppliedAttempt(row, binding) != nil {
		return invalid
	}
	return nil
}

// ReconcileApprovedV3CreationReceipt is restricted to an already signed,
// active original attempt. It never signs or broadcasts. A confirmed revert
// releases the slot through the existing finalizer; a success remains
// final_success until the canonical ProjectCreated bridge independently locks
// the project. No new attempt is authorized by a successful receipt alone.
func ReconcileApprovedV3CreationReceipt(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	projectID, submissionID uint64, ownerCodeHash, governorCodeHash common.Hash) (*entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 approved creation reconciliation unavailable")
	audit, err := AuditApprovedV3Creation(ctx, database, rpc, projectID, submissionID, ownerCodeHash, governorCodeHash)
	if err != nil {
		return nil, invalid
	}
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(audit.Block))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != audit.Block || header.Hash() != audit.BlockHash {
		return nil, invalid
	}
	var updated entity.ChainTxSubmission
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, projectID)
		if e != nil {
			return invalid
		}
		id := record.Identity
		binding := idov3whitelist.ExpectedProjectBinding{ChainID: id.ChainID, ProjectID: projectID,
			ProxyAddress: common.HexToAddress(id.Proxy), ProjectToken: common.HexToAddress(id.ProjectToken),
			PaymentToken: common.HexToAddress(id.PaymentToken)}
		// Lock order is reorg -> project -> submission -> shared sender nonce.
		if chainreorg.LockNoActiveIncidentTx(tx, id.ChainID, id.Proxy) != nil {
			return invalid
		}
		var projects []struct {
			ProjectID uint64 `gorm:"column:project_id"`
		}
		read := tx.Raw(`SELECT p.project_id FROM projects p JOIN ido_v3_project_binding b
			ON b.project_id=p.project_id AND b.chain_id=p.chain_id
			AND BINARY b.proxy_address=BINARY p.contract_address
			AND BINARY b.contract_version=BINARY p.contract_version
			JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
			AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
			WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.funding_mode=BINARY 'capped_presale'
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY p.status=BINARY 'Set' AND p.onchain_locked=0
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY c.operator_address=BINARY ? AND BINARY c.create_calldata_hash=BINARY ?
			LIMIT 2 FOR UPDATE`, projectID, id.ChainID, id.Proxy, id.ProjectToken, id.PaymentToken,
			id.Operator, id.CreateCallHash).Scan(&projects)
		if read.Error != nil || len(projects) != 1 || projects[0].ProjectID != projectID {
			return invalid
		}
		var rows []entity.ChainTxSubmission
		read = tx.Raw(`SELECT * FROM chain_tx_submission WHERE id=? AND chain_id=?
			AND project_id=? AND BINARY contract_address=BINARY ?
			AND BINARY action=BINARY 'ido_v3_create' AND BINARY LOWER(sender_address)=BINARY ?
			AND active_slot=1 LIMIT 2 FOR UPDATE`, submissionID, id.ChainID, projectID, id.Proxy, id.Operator).Scan(&rows)
		if read.Error != nil || len(rows) != 1 {
			return invalid
		}
		row := rows[0]
		if row.State == constant.ChainTxStateFinalSuccess || row.State == constant.ChainTxStateFinalRevert ||
			row.State == constant.ChainTxStateApplied || row.State == ChainTxStateV3ReorgSuspended ||
			approvedV3OriginalAttempt(row, binding, id) != nil ||
			row.TxHash == nil || common.HexToHash(*row.TxHash) != audit.Hash ||
			row.Nonce == nil || *row.Nonce != audit.Nonce {
			return invalid
		}
		updated, e = finalizeV3CreationReceiptLockedTx(tx, row, binding,
			common.HexToAddress(id.Operator), audit, id.CreateDeadline)
		return e
	})
	if err != nil {
		return nil, invalid
	}
	return &updated, nil
}
