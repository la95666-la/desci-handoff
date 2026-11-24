package project

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// verifyApprovedV3PriorPolicy refuses to reuse a policy baseline whose
// execution was recorded in the obsolete offline-attestation digest domain.
// The previous version must have its own executed four-eyes review and actual
// Safe original-hash receipt. This is only an extra guard, not proof authority.
func verifyApprovedV3PriorPolicy(ctx context.Context, database *gorm.DB, rpc *ethclient.Client,
	admitted idov3whitelist.AdmittedProject, next V3PolicyPreview, projectID uint64) error {
	invalid := errors.New("V3 approved prior policy unavailable")
	if ctx == nil || database == nil || rpc == nil || !next.OldEnabled || projectID == 0 ||
		!v3CanonicalHash(next.OldRoot) || mustParseV3Version(next.OldVersion) == 0 {
		return invalid
	}
	var ids []struct {
		ID uint64 `gorm:"column:id"`
	}
	read := database.WithContext(ctx).Raw(`SELECT r.id FROM ido_v3_policy_review r
		JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.chain_id=r.chain_id
		 AND BINARY d.proxy_address=BINARY r.proxy_address AND d.project_id=r.project_id
		 AND d.target_version=r.target_version AND BINARY d.decision=BINARY 'approved'
		JOIN ido_v3_policy_review_execution e ON e.review_id=d.review_id
		JOIN ido_v3_policy_event_journal j ON j.chain_id=r.chain_id AND BINARY j.proxy_address=BINARY r.proxy_address
		 AND j.project_id=r.project_id AND j.version=r.target_version AND BINARY j.root=BINARY r.root
		 AND BINARY j.tx_hash=BINARY e.tx_hash AND j.block_number=e.receipt_block
		 AND BINARY j.block_hash=BINARY e.receipt_hash AND j.enabled=1 AND BINARY j.state=BINARY 'canonical'
		WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version=?
		 AND r.enabled=1 AND BINARY r.root=BINARY ? LIMIT 2`, admitted.Binding.ChainID,
		next.Proxy, projectID, mustParseV3Version(next.OldVersion), next.OldRoot).Scan(&ids)
	if read.Error != nil || len(ids) != 1 || ids[0].ID == 0 {
		return invalid
	}
	var prior approvedV3PolicyExecutionRecord
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		prior, e = readApprovedV3PolicyExecutionTx(tx, projectID, ids[0].ID)
		return e
	}); err != nil || prior.TxHash == "" || prior.Preview.TargetVersion != next.OldVersion ||
		prior.Preview.Root != next.OldRoot || !prior.Preview.Enabled {
		return invalid
	}
	scope, commitment, err := v3PolicyIdentity(prior.Preview, projectID)
	if err != nil || scope.ProxyAddress != admitted.Binding.ProxyAddress {
		return invalid
	}
	evidence, err := idov3whitelist.ReconcilePolicyEvidence(ctx, rpc, scope, admitted.Runtime,
		commitment, common.HexToHash(prior.TxHash), admitted.Confirmations)
	if err != nil || evidence.ReceiptBlock.BlockNumber != prior.ReceiptBlock ||
		strings.ToLower(evidence.ReceiptBlock.BlockHash.Hex()) != prior.ReceiptHash {
		return invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, common.HexToHash(prior.TxHash))
	if err != nil || receipt == nil || receipt.TxHash != common.HexToHash(prior.TxHash) ||
		receipt.BlockHash != evidence.ReceiptBlock.BlockHash {
		return invalid
	}
	change := idov3whitelist.PolicyChange{ExpectedVersion: mustParseV3Version(prior.Preview.OldVersion),
		TargetVersion: commitment.Version, Enabled: true, Root: commitment.Root, Candidate: &commitment}
	if idov3whitelist.CheckSafePolicyExecution(ctx, rpc, scope, common.HexToAddress(v3DiagnosticGovernor),
		change, common.HexToHash(prior.TxHash), common.HexToHash(prior.SafeTxHash), receipt) != nil {
		return invalid
	}
	height := new(big.Int).SetUint64(prior.ReceiptBlock)
	header, err := rpc.HeaderByNumber(ctx, height)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(height) != 0 ||
		strings.ToLower(header.Hash().Hex()) != prior.ReceiptHash {
		return invalid
	}
	return nil
}
