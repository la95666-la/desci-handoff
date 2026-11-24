// Package idov3candidate reads inert, unapproved V3 foundation rows for
// offline audit. Nothing in this package approves a policy or serves proofs.
package idov3candidate

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

var canonicalHash = regexp.MustCompile(`^0x[0-9a-f]{64}$`)

// AuditCandidate checks the complete member set against an inert snapshot
// within one repeatable-read, read-only transaction. The runtime is deliberately
// still unverified; this is not a trusted approval, chain read, or proof source.
// A missing table or any other DB failure is reported without leaking DB errors.
func AuditCandidate(ctx context.Context, database *gorm.DB, scope idov3whitelist.Scope, snapshotID uint64) (idov3whitelist.SnapshotCommitment, error) {
	return auditCandidate(ctx, database, scope, snapshotID, nil)
}

// afterSnapshotRead is a test-only interleaving point. Production calls use
// nil; it never permits a write through the read-only transaction.
func auditCandidate(ctx context.Context, database *gorm.DB, scope idov3whitelist.Scope, snapshotID uint64, afterSnapshotRead func() error) (idov3whitelist.SnapshotCommitment, error) {
	if database == nil || snapshotID == 0 || scope.ChainID == nil || !scope.ChainID.IsUint64() || scope.ChainID.Sign() <= 0 ||
		scope.ProjectID == nil || !scope.ProjectID.IsUint64() || scope.ProjectID.Sign() <= 0 || scope.ProxyAddress == (common.Address{}) {
		return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate audit scope invalid")
	}
	tx := database.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if tx.Error != nil {
		return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate audit unavailable")
	}
	defer tx.Rollback()
	var row struct {
		ExpectedVersion uint64 `gorm:"column:expected_version"`
		Version         uint64 `gorm:"column:target_version"`
		Root            string `gorm:"column:root"`
		MembersDigest   string `gorm:"column:members_digest"`
		Digest          string `gorm:"column:snapshot_digest"`
		MemberCount     uint32 `gorm:"column:member_count"`
	}
	// A joined, exact identity is mandatory even for an offline audit. Do not
	// infer chain or proxy from the supplied candidate ID alone.
	const snapshotQuery = `SELECT s.expected_version,s.target_version,s.root,s.members_digest,s.snapshot_digest,s.member_count
		FROM ido_v3_policy_snapshot s
		JOIN ido_v3_project_binding b ON b.project_id=s.project_id AND b.chain_id=s.chain_id AND b.proxy_address=s.proxy_address AND b.contract_version=s.contract_version
		JOIN ido_v3_runtime r ON r.chain_id=b.chain_id AND r.proxy_address=b.proxy_address
		JOIN projects p ON p.project_id=b.project_id
		WHERE s.id=? AND s.chain_id=? AND s.project_id=? AND s.proxy_address=?
		AND s.contract_version='ido_v3_stablecoin' AND s.state='candidate' AND s.enabled=1
		AND s.approver_admin_id IS NULL AND s.admin_approval_id IS NULL
		AND r.contract_version='ido_v3_stablecoin' AND r.state='unverified'
		AND p.chain_id=b.chain_id AND p.contract_address=b.proxy_address
		AND p.contract_version='ido_v3_stablecoin' AND p.funding_mode='capped_presale'`
	if err := tx.Raw(snapshotQuery, snapshotID, scope.ChainID.Uint64(), scope.ProjectID.Uint64(), strings.ToLower(scope.ProxyAddress.Hex())).Scan(&row).Error; err != nil || row.Version == 0 || row.ExpectedVersion == ^uint64(0) || row.ExpectedVersion+1 != row.Version ||
		row.MemberCount == 0 || row.MemberCount > idov3whitelist.MaxWallets ||
		!canonicalHash.MatchString(row.Root) || !canonicalHash.MatchString(row.MembersDigest) || !canonicalHash.MatchString(row.Digest) {
		return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate snapshot unavailable or malformed")
	}
	candidate := idov3whitelist.SnapshotCommitment{
		Version: row.Version, Root: common.HexToHash(row.Root), MembersDigest: common.HexToHash(row.MembersDigest),
		Digest: common.HexToHash(row.Digest), MemberCount: row.MemberCount,
	}
	if afterSnapshotRead != nil && afterSnapshotRead() != nil {
		return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate audit unavailable")
	}
	var members []struct {
		Index  uint32 `gorm:"column:leaf_index"`
		Wallet string `gorm:"column:wallet_address"`
		Leaf   string `gorm:"column:leaf_hash"`
	}
	if err := tx.Raw(`SELECT leaf_index,wallet_address,leaf_hash FROM ido_v3_policy_member WHERE snapshot_id=? ORDER BY leaf_index`, snapshotID).Scan(&members).Error; err != nil || len(members) != int(row.MemberCount) {
		return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate members unavailable or incomplete")
	}
	verified := make([]idov3whitelist.CandidateMemberRow, 0, len(members))
	for _, member := range members {
		if !canonicalHash.MatchString(member.Leaf) {
			return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate member encoding invalid")
		}
		verified = append(verified, idov3whitelist.CandidateMemberRow{Index: member.Index, Wallet: member.Wallet, Leaf: common.HexToHash(member.Leaf)})
	}
	if err := idov3whitelist.CheckCandidateRows(scope, candidate, verified); err != nil {
		return idov3whitelist.SnapshotCommitment{}, errors.New("V3 candidate member commitment mismatch")
	}
	// Defer always rolls back; no caller gets the member set or a Tree/Proof.
	return candidate, nil
}

// NewScope uses numeric IDs without floating-point conversion.
func NewScope(chainID, projectID uint64, proxy common.Address) idov3whitelist.Scope {
	return idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(chainID), ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: proxy}
}
