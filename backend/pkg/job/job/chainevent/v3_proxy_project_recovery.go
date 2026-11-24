package chainevent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// Only unchanged-tip (unknown-source halt) recovery. Actual replaced tips stay
// closed for the existing ancestor/reorg reconciliation. Full coverage starts
// before creation; no 512-block bypass or endpoint-only certificate is used.
func recoverV3ProjectFromCoverage(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, id, expected uint64, owner, governor common.Hash) error {
	s := productionV3StreamScope()
	candidate, e := idov3whitelist.ReadApprovedHaltedCandidate(ctx, db, id)
	if e != nil {
		return e
	}
	if expected == 0 || candidate.CursorRevision != expected {
		return errV3Stream
	}
	if e = idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, candidate.Record, owner, governor, candidate.CreationBlock, candidate.CreationHash); e != nil {
		return e
	}
	if e = v3StreamAnchor(ctx, rpc, s, candidate.CursorBlock, candidate.CursorHash); e != nil {
		return fmt.Errorf("%w: replaced/unknown tip; no automatic recovery", errV3Stream)
	}
	return v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		stream, e := v3StreamCursorRead(tx, s, true)
		if e != nil || stream.State != "running" {
			return errV3Stream
		}
		var locked struct {
			Revision uint64
			State    string
		}
		if r := tx.Raw(`SELECT revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? FOR UPDATE`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Scan(&locked); r.Error != nil || r.RowsAffected != 1 || locked.Revision != expected || locked.State != "reorg_halted" {
			return errV3Stream
		}
		c, e := idov3whitelist.ReadApprovedHaltedCandidate(ctx, tx, id)
		if e != nil || c.CursorRevision != expected || c.CursorBlock != candidate.CursorBlock || c.CursorHash != candidate.CursorHash {
			return errV3Stream
		}
		return restoreV3CoveredCandidate(ctx, tx, rpc, s, c)
	})
}

// Only the authenticated original-create caller above may supply a candidate.
// Kept separate to exercise the full transaction with synthetic identities.
func restoreV3CoveredCandidate(ctx context.Context, tx *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, c idov3whitelist.ApprovedLockedCandidate) error {
	if _, e := auditV3CoveredProject(ctx, tx, rpc, s, c.Binding, c.CreationBlock, c.CursorBlock, c.CursorHash, true); e != nil {
		return e
	}
	// Audit every original signed suspended attempt independently, never
	// assigning/releasing a nonce and never querying a signing key.
	var attempts []entity.ChainTxSubmission
	if tx.Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND project_id=? AND contract_address=? AND state='v3_reorg_suspended' ORDER BY id FOR UPDATE`, s.Chain, c.Binding.ProjectID, v3Lower(s.Proxy)).Scan(&attempts).Error != nil {
		return errV3Stream
	}
	for _, attempt := range attempts {
		if chaintx.VerifyV3PersistedAppliedAttempt(attempt, c.Binding) != nil || attempt.TxHash == nil || attempt.ReceiptBlockNumber == nil || *attempt.ReceiptBlockNumber > c.CursorBlock {
			return errV3Stream
		}
		receipt, e := rpc.TransactionReceipt(ctx, common.HexToHash(*attempt.TxHash))
		if e != nil || receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.TxHash != common.HexToHash(*attempt.TxHash) || receipt.Status != 1 || receipt.BlockNumber.Uint64() != *attempt.ReceiptBlockNumber || !strings.EqualFold(receipt.BlockHash.Hex(), attempt.ReceiptBlockHash) {
			return errV3Stream
		}
		if e = v3StreamAnchor(ctx, rpc, s, *attempt.ReceiptBlockNumber, receipt.BlockHash); e != nil {
			return e
		}
	}
	if e := (v3Journal{db: tx}).resumeAtAncestor(ctx, c.Binding, c.CursorBlock, c.CursorHash, c.CursorRevision); e != nil {
		return e
	}
	if e := v3StreamAnchor(ctx, rpc, s, c.CursorBlock, c.CursorHash); e != nil {
		return e
	}
	return nil
}
