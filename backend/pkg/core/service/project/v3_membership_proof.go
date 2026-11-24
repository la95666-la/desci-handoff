package project

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/db"
)

var errV3Membership = errors.New("V3 wallet qualification unavailable")

func proveApprovedV3Membership(uid int, projectID uint64, rawWallet string, sessionVersion uint64) (V3WalletProof, error) {
	if !viper.GetBool("idoV3PolicyProof.enabled") || uid <= 0 || projectID == 0 || sessionVersion == 0 || db.Mysql() == nil || !common.IsHexAddress(rawWallet) || common.HexToAddress(rawWallet) == (common.Address{}) {
		return V3WalletProof{}, errV3Membership
	}
	ctx, cancel := context.WithTimeout(context.Background(), v3WalletProofReadBudget(true))
	defer cancel()
	return proveV3Membership(ctx, db.Mysql(), uid, projectID, strings.ToLower(common.HexToAddress(rawWallet).Hex()), sessionVersion)
}

func requireV3ProofWallet(tx *gorm.DB, uid int, wallet string, session uint64) error {
	var owned int64
	r := tx.Raw(`SELECT COUNT(*) FROM user_wallet_identity i JOIN user_wallet w
 ON w.uid=i.uid AND BINARY LOWER(w.wallet_address)=BINARY i.wallet_address AND w.chain=i.chain
 JOIN user u ON u.id=i.uid WHERE i.uid=? AND BINARY i.wallet_address=BINARY ?
 AND i.chain='evm' AND i.claim_method<>'' AND w.verified_at IS NOT NULL
 AND w.verification_method<>'' AND u.is_enable=1 AND u.session_version=?`, uid, wallet, session).Scan(&owned)
	if r.Error != nil || owned != 1 {
		return errV3Membership
	}
	return nil
}

// Select the latest EXECUTED approved version, including disabled versions:
// never fall back to an old enabled list when the current list is closed.
func readV3MembershipTx(tx *gorm.DB, uid int, projectID uint64, wallet string, session uint64) (approvedV3PolicyExecutionRecord, error) {
	var zero approvedV3PolicyExecutionRecord
	if requireV3ProofWallet(tx, uid, wallet, session) != nil {
		return zero, errV3Membership
	}
	var projects []struct {
		ID uint64 `gorm:"column:project_id"`
	}
	if r := tx.Raw(`SELECT project_id FROM projects WHERE project_id=? AND chain_id=56
 AND BINARY contract_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
 AND BINARY funding_mode=BINARY 'capped_presale' AND onchain_locked=1 AND BINARY status<>BINARY 'archived' LIMIT 2`, projectID, strings.ToLower(v3TokenProxy)).Scan(&projects); r.Error != nil || len(projects) != 1 {
		return zero, errV3Membership
	}
	var reviews []struct{ ID uint64 }
	r := tx.Raw(`SELECT r.id FROM ido_v3_policy_review r JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.decision='approved'
 JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
 WHERE r.chain_id=56 AND BINARY r.proxy_address=BINARY ? AND r.project_id=?
 ORDER BY r.target_version DESC LIMIT 1`, strings.ToLower(v3TokenProxy), projectID).Scan(&reviews)
	if r.Error != nil || len(reviews) != 1 {
		return zero, errV3Membership
	}
	record, e := readApprovedV3PolicyExecutionTx(tx, projectID, reviews[0].ID)
	if e != nil || !record.Preview.Enabled || record.InitiatorID == 0 || record.ApproverID == 0 || record.InitiatorID == record.ApproverID {
		return zero, errV3Membership
	}
	return record, nil
}

func v3MembershipTree(record approvedV3PolicyExecutionRecord, projectID uint64) (idov3whitelist.Scope, *idov3whitelist.Tree, idov3whitelist.Policy, error) {
	scope, commitment, e := v3PolicyIdentity(record.Preview, projectID)
	if e != nil || scope.ChainID.Uint64() != 56 || scope.ProxyAddress != common.HexToAddress(v3TokenProxy) || !record.Preview.Enabled {
		return scope, nil, idov3whitelist.Policy{}, errV3Membership
	}
	tree, e := idov3whitelist.Build(scope, record.Preview.CanonicalMembers)
	if e != nil {
		return scope, nil, idov3whitelist.Policy{}, errV3Membership
	}
	actual, e := tree.CommitSnapshot(scope, commitment.Version)
	if e != nil || actual != commitment {
		return scope, nil, idov3whitelist.Policy{}, errV3Membership
	}
	return scope, tree, idov3whitelist.Policy{Enabled: true, Root: commitment.Root, Version: commitment.Version}, nil
}

// Pure DB + local Merkle computation. Policy execution/chain validation belongs
// to publication; the purchase contract rejects stale versions at execution.
func proveV3Membership(ctx context.Context, database *gorm.DB, uid int, projectID uint64, wallet string, session uint64) (V3WalletProof, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || uid <= 0 || projectID == 0 || session == 0 || !common.IsHexAddress(wallet) || common.HexToAddress(wallet) == (common.Address{}) {
		return V3WalletProof{}, errV3Membership
	}
	wallet = strings.ToLower(common.HexToAddress(wallet).Hex())
	load := func() (approvedV3PolicyExecutionRecord, error) {
		var record approvedV3PolicyExecutionRecord
		e := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var e error
			record, e = readV3MembershipTx(tx, uid, projectID, wallet, session)
			return e
		})
		return record, e
	}
	return proveV3MembershipWithSource(ctx, projectID, wallet, load)
}

func proveV3MembershipWithSource(ctx context.Context, projectID uint64, wallet string, load func() (approvedV3PolicyExecutionRecord, error)) (V3WalletProof, error) {
	if ctx == nil || ctx.Err() != nil || projectID == 0 || !common.IsHexAddress(wallet) || common.HexToAddress(wallet) == (common.Address{}) || load == nil {
		return V3WalletProof{}, errV3Membership
	}
	wallet = strings.ToLower(common.HexToAddress(wallet).Hex())
	record, e := load()
	if e != nil {
		return V3WalletProof{}, errV3Membership
	}
	scope, tree, want, e := v3MembershipTree(record, projectID)
	if e != nil {
		return V3WalletProof{}, errV3Membership
	}
	version, siblings, e := tree.ProofForPolicy(scope, want, wallet)
	if e != nil {
		return V3WalletProof{}, errV3Membership
	}
	// A second fresh DB transaction closes unbinding/session/publication changes
	// during computation. No RPC, cached proof or reused RR snapshot is involved.
	fresh, e := load()
	if e != nil || fresh.ReviewID != record.ReviewID || fresh.Digest != record.Digest || fresh.Preview.PreviewDigest != record.Preview.PreviewDigest || fresh.TxHash != record.TxHash || fresh.SafeTxHash != record.SafeTxHash {
		return V3WalletProof{}, errV3Membership
	}
	// Chain state may have advanced beyond this published list. Do not pretend
	// to certify current on-chain eligibility; the client/contract enforce the
	// actual policy version/root, sale state/time/caps/payment at execution.
	if ctx.Err() != nil {
		return V3WalletProof{}, errV3Membership
	}
	proof := make([]string, len(siblings))
	for i, h := range siblings {
		proof[i] = h.Hex()
	}
	return V3WalletProof{ChainID: "56", ProjectID: strconv.FormatUint(projectID, 10), Proxy: strings.ToLower(scope.ProxyAddress.Hex()), Wallet: wallet, Version: strconv.FormatUint(version, 10), Root: want.Root.Hex(), Proof: proof}, nil
}
