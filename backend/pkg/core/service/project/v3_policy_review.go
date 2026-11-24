package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

const v3PolicyOperation = "ido_v3.policy.versioned_change"

// New reviews allow one day for a distinct Admin to approve and a further
// full day for Policy Safe owners. The immutable stored expiry remains the
// ultimate Safe deadline; old reviews retain their original stored expiry.
const v3PolicyReviewApprovalTTL = 24 * time.Hour
const v3PolicyReviewTTL = 2 * v3PolicyReviewApprovalTTL

func v3PolicyReviewApprovalOpen(created, expires, now time.Time) bool {
	return now.Before(expires) && now.Before(created.Add(v3PolicyReviewApprovalTTL))
}

var ErrV3PolicyReviewSchemaMigrationRequired = errors.New("V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED")

var v3ReviewPolicyEventTopic = crypto.Keccak256Hash([]byte("WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)"))

type V3PolicyReviewInput struct {
	ProjectID     uint64
	Wallets       []string
	Enabled       bool
	Reason        string
	AdminID       uint64
	PreviewDigest string
	BaseBlock     uint64
	BaseHash      string
}

// The preview digest covers all caller-visible fields including the base
// block, canonical full members and reason. No caller may supply a root or
// mutate a preview between the live chain read and its immutable insert.
func V3PolicyPreviewDigest(p V3PolicyPreview) (string, error) {
	p.PreviewDigest = ""
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", errors.New("V3 policy preview encoding unavailable")
	}
	sum := sha256.Sum256(append([]byte("joincare:ido-v3:policy-preview:v1\x00"), encoded...))
	return "0x" + hex.EncodeToString(sum[:]), nil
}

func v3PolicyIdentity(p V3PolicyPreview, expected uint64) (idov3whitelist.Scope, idov3whitelist.SnapshotCommitment, error) {
	invalid := errors.New("V3 policy review identity unavailable")
	chain, e1 := strconv.ParseUint(p.ChainID, 10, 64)
	project, e2 := strconv.ParseUint(p.ProjectID, 10, 64)
	old, e3 := strconv.ParseUint(p.OldVersion, 10, 64)
	newVersion, e4 := strconv.ParseUint(p.TargetVersion, 10, 64)
	block, e5 := strconv.ParseUint(p.Block, 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || chain == 0 || project != expected ||
		block == 0 || old == ^uint64(0) || newVersion != old+1 || !common.IsHexAddress(p.Proxy) ||
		p.Proxy != strings.ToLower(p.Proxy) || p.MemberCount != len(p.CanonicalMembers) ||
		len(p.CanonicalMembers) > 512 || (p.State != "pending" && p.State != "active") ||
		(p.State == "active" && (!p.OldEnabled || !p.Enabled)) ||
		p.OldEnabled == (p.OldRoot == (common.Hash{}).Hex()) ||
		!v3CanonicalHash(p.BlockHash) || strings.TrimSpace(p.Reason) == "" ||
		(p.OldEnabled && !v3CanonicalHash(p.OldRoot)) ||
		(!p.OldEnabled && p.State == "active") ||
		(p.Enabled && len(p.CanonicalMembers) == 0) || (!p.Enabled && (!p.OldEnabled || p.State != "pending" ||
		len(p.CanonicalMembers) != 0 || p.Root != (common.Hash{}).Hex() || p.MembersDigest != "" || p.SnapshotDigest != "")) {
		return idov3whitelist.Scope{}, idov3whitelist.SnapshotCommitment{}, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(chain), ProjectID: new(big.Int).SetUint64(project), ProxyAddress: common.HexToAddress(p.Proxy)}
	if !p.Enabled {
		return scope, idov3whitelist.SnapshotCommitment{}, nil
	}
	tree, err := idov3whitelist.Build(scope, p.CanonicalMembers)
	if err != nil {
		return scope, idov3whitelist.SnapshotCommitment{}, invalid
	}
	for i, member := range tree.Members() {
		if p.CanonicalMembers[i] != strings.ToLower(member.Wallet.Hex()) {
			return scope, idov3whitelist.SnapshotCommitment{}, invalid
		}
	}
	commitment, err := tree.CommitSnapshot(scope, newVersion)
	if err != nil || strings.ToLower(commitment.Root.Hex()) != strings.ToLower(p.Root) ||
		strings.ToLower(commitment.Digest.Hex()) != strings.ToLower(p.SnapshotDigest) ||
		strings.ToLower(commitment.MembersDigest.Hex()) != strings.ToLower(p.MembersDigest) {
		return scope, idov3whitelist.SnapshotCommitment{}, invalid
	}
	return scope, commitment, nil
}

func V3PolicyReviewStart(in V3PolicyReviewInput) (uint64, error) {
	return v3PolicyReviewStartWithAdmission(context.Background(), in, false)
}

// StartApprovedV3PolicyReview retains the immutable two-admin review and
// approved project admission. Admin login and live permissions authorize only
// an off-chain decision; Policy Safe execution remains independent.
func StartApprovedV3PolicyReview(in V3PolicyReviewInput) (uint64, error) {
	return StartApprovedV3PolicyReviewContext(context.Background(), in)
}

func StartApprovedV3PolicyReviewContext(ctx context.Context, in V3PolicyReviewInput) (uint64, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, errors.New("V3 policy review initiation unavailable")
	}
	if err := requireV3PolicyReviewNoMFASchema(ctx, db.Mysql()); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 160*time.Second)
	defer cancel()
	return v3PolicyReviewStartWithAdmission(ctx, in, true)
}

// A production Backend built for two logged-in admins writes SQL NULL for
// these legacy MFA digest columns. Refuse before expensive RPC auditing when
// its additive migration is missing; never fabricate a digest or alter data.
func requireV3PolicyReviewNoMFASchema(ctx context.Context, database *gorm.DB) error {
	invalid := ErrV3PolicyReviewSchemaMigrationRequired
	if ctx == nil || ctx.Err() != nil || database == nil {
		return invalid
	}
	var count int64
	read := database.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND IS_NULLABLE='YES' AND
		((TABLE_NAME='ido_v3_policy_review' AND COLUMN_NAME='initiator_mfa_digest') OR
		 (TABLE_NAME='ido_v3_policy_review_decision' AND COLUMN_NAME='approver_mfa_digest'))`).Scan(&count)
	if read.Error != nil || count != 2 {
		return invalid
	}
	return nil
}

func v3PolicyReviewStartWithAdmission(parent context.Context, in V3PolicyReviewInput, approved bool) (reviewResult uint64, resultErr error) {
	phase := "review_recompute"
	var diagnosticCtx context.Context
	var cleanup []context.CancelFunc
	commitStarted := false
	defer func() {
		defer func() {
			for _, cancel := range cleanup {
				cancel()
			}
		}()
		if resultErr != nil {
			resultErr = v3PolicyContextDiagnostic(diagnosticCtx, resultErr, phase)
			if !commitStarted {
				f, _ := V3Failure(resultErr)
				f.Outcome, f.NextAction = "not_reserved", "refresh_and_check"
				resultErr = &v3OperationError{failure: f, cause: resultErr}
			}
		}
	}()
	invalid := errors.New("V3 policy review initiation unavailable")
	if parent == nil || parent.Err() != nil || !viper.GetBool("idoV3PolicyReview.enabled") || in.AdminID == 0 || in.ProjectID == 0 ||
		len(in.Wallets) > 512 || !v3CanonicalHash(in.PreviewDigest) || in.BaseBlock == 0 ||
		!v3CanonicalHash(in.BaseHash) || db.Mysql() == nil {
		return 0, invalid
	}
	var request *v3PolicyReviewRequest
	if approved {
		ctx, cancel := context.WithTimeout(parent, v3PolicyReviewRequestBudget)
		cleanup = append(cleanup, cancel)
		diagnosticCtx, parent = ctx, ctx
		owner, governor, err := approvedV3SafeHashes()
		if err != nil {
			return 0, err
		}
		rpc, err := v3TokenGateRPC(ctx)
		if err != nil {
			return 0, err
		}
		defer rpc.Close()
		request = &v3PolicyReviewRequest{ctx: ctx, database: db.Mysql(), rpc: rpc, projectID: in.ProjectID, owner: owner, governor: governor}
	}
	preview, err := previewV3PolicyChangeForRequest(parent, in.ProjectID, in.Wallets, in.Enabled, in.Reason, in.BaseBlock, in.BaseHash, approved, true, request)
	if err != nil {
		return 0, err
	}
	digest, err := V3PolicyPreviewDigest(preview)
	if err != nil || digest != in.PreviewDigest || preview.PreviewDigest != digest || preview.Enabled != in.Enabled {
		return 0, v3diagnostic.New("STATE_CHANGED", "review_digest", invalid)
	}
	scope, candidate, err := v3PolicyIdentity(preview, in.ProjectID)
	if err != nil {
		return 0, invalid
	}
	firstEnableReview := approved && preview.State == "pending" && !preview.OldEnabled && preview.OldVersion == "0" && preview.Enabled
	ctx := parent
	if request != nil {
		// The approved path owns one overall deadline, not a second clock.
		ctx = request.ctx
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, 12*time.Second)
		cleanup = append(cleanup, cancel)
	}
	diagnosticCtx = ctx
	var admitted idov3whitelist.AdmittedProject
	var ownerHash, governorHash common.Hash
	if approved {
		ownerHash, governorHash, err = approvedV3SafeHashes()
		if err != nil {
			return 0, invalid
		}
	} else {
		admitted, _, _, _, _, err = pinnedV3Admission(in.ProjectID, false)
		if err != nil {
			return 0, invalid
		}
	}
	var rpc *ethclient.Client
	if approved {
		rpc = request.rpc
	} else {
		rpc, err = chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	}
	if err != nil {
		return 0, invalid
	}
	if request == nil {
		defer rpc.Close()
	}
	var reviewSnapshot idov3whitelist.ApprovedLockedCandidate
	if approved {
		admitted, reviewSnapshot, err = request.admit(ctx, db.Mysql(), rpc, in.ProjectID, ownerHash, governorHash)
		if err != nil || requireV3ReviewOrStrictScan(ctx, db.Mysql(), firstEnableReview, in.ProjectID) != nil {
			if err != nil {
				return 0, v3ReadDiagnostic(err, "project_admission")
			}
			return 0, v3diagnostic.New("UNAVAILABLE", "policy_evidence_store", invalid)
		}
	}
	if admitted.Binding.ChainID != scope.ChainID.Uint64() || admitted.Binding.ProxyAddress != scope.ProxyAddress {
		return 0, invalid
	}
	block, _ := strconv.ParseUint(preview.Block, 10, 64)
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if err != nil || header == nil || strings.ToLower(header.Hash().Hex()) != preview.BlockHash {
		return 0, invalid
	}
	if !approved {
		status, statusErr := ReadV3PolicyStatus(in.ProjectID)
		if statusErr != nil || status.ProjectState != preview.State || status.ChainID != preview.ChainID || status.Proxy != preview.Proxy ||
			status.Version != preview.OldVersion || status.Enabled != preview.OldEnabled || status.Root != preview.OldRoot {
			if statusErr != nil {
				return 0, statusErr
			}
			return 0, v3diagnostic.New("STATE_CHANGED", "status", invalid)
		}
	}
	if request == nil {
		latest, err := rpc.BlockNumber(ctx)
		if err != nil {
			return 0, v3ReadDiagnostic(err, "live_state")
		}
		latestHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
		if err != nil || latestHeader == nil || latestHeader.Number == nil || !latestHeader.Number.IsUint64() || latestHeader.Number.Uint64() != latest {
			return 0, invalid
		}
		livePolicy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
		if err != nil || livePolicy.Policy.Version != mustParseV3Version(preview.OldVersion) || livePolicy.Policy.Enabled != preview.OldEnabled || strings.ToLower(livePolicy.Policy.Root.Hex()) != preview.OldRoot {
			return 0, invalid
		}
		liveProject, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
		if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, liveProject) != nil ||
			(preview.State == "pending" && liveProject.State != idov3whitelist.ProjectPending) ||
			(preview.State == "active" && liveProject.State != idov3whitelist.ProjectActive) {
			return 0, invalid
		}
		latestAgain, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
		if err != nil || latestAgain == nil || latestAgain.Hash() != latestHeader.Hash() {
			return 0, invalid
		}
		if now, e := rpc.BlockNumber(ctx); e != nil || !v3PolicyPreviewHeadAdmitted(latest, now, firstEnableReview) {
			return 0, invalid
		}
	}
	if preview.OldEnabled && v3VerifyPriorReceipt(ctx, rpc, admitted, preview, in.ProjectID) != nil {
		return 0, invalid
	}
	if approved && preview.OldEnabled && verifyApprovedV3PriorPolicy(ctx, db.Mysql(), rpc, admitted, preview, in.ProjectID) != nil {
		return 0, invalid
	}
	if request != nil {
		phase = "final_recheck"
		if err := request.finalRecheck(preview); err != nil {
			return 0, err
		}
	}
	var id uint64
	phase = "review_commit"
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if approved {
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < 5*time.Second {
			return 0, v3diagnostic.New("CHECK_TIMEOUT", phase, context.DeadlineExceeded)
		}
	}
	commitStarted = true
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, scope.ChainID.Uint64(), preview.Proxy) != nil ||
			chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		if approved {
			set, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, in.ProjectID)
			if e != nil || set.Identity != reviewSnapshot.Record.Identity || !bytes.Equal(set.Request, reviewSnapshot.Record.Request) {
				return invalid
			}
			current, e := idov3whitelist.ReadApprovedLockedCandidate(ctx, tx, in.ProjectID)
			if e != nil || chaintx.RecheckPolicyObservationCandidate(reviewSnapshot, current) != nil {
				return invalid
			}
		}
		// No draft may be based on an already observed newer policy even when
		// the confirmed status read has not yet caught up with the scanner.
		var superseding int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
			WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version>?`,
			scope.ChainID.Uint64(), preview.Proxy, in.ProjectID, mustParseV3Version(preview.OldVersion)).Scan(&superseding).Error != nil || superseding != 0 {
			return invalid
		}
		if preview.OldEnabled {
			prior, e := v3PolicyPriorMembersTx(tx, preview, in.ProjectID)
			if e != nil {
				return invalid
			}
			added, removed := v3PolicyMemberDiff(prior, preview.CanonicalMembers)
			if !v3SameMembers(added, preview.AddedMembers) || !v3SameMembers(removed, preview.RemovedMembers) {
				return invalid
			}
		}
		var projects []struct {
			ID uint64 `gorm:"column:project_id"`
		}
		read := tx.Raw(`SELECT p.project_id FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id
			AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
			WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.funding_mode=BINARY 'capped_presale'
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY p.status=BINARY ? AND p.onchain_locked=1 LIMIT 2 FOR UPDATE`, in.ProjectID, scope.ChainID.Uint64(), preview.Proxy,
			strings.ToLower(admitted.Binding.ProjectToken.Hex()), strings.ToLower(admitted.Binding.PaymentToken.Hex()), strings.ToUpper(preview.State[:1])+preview.State[1:]).Scan(&projects)
		if read.Error != nil || len(projects) != 1 || projects[0].ID != in.ProjectID {
			return invalid
		}
		if _, err := middleware.HasLivePermissionTx(tx, int64(in.AdminID), middleware.ModProject, middleware.ActWrite); err != nil {
			return invalid
		}
		// Project lock serializes identical double clicks. Reuse only a still
		// pending, immutable draft by THIS initiator with this exact digest;
		// never renew expiry or reuse an approved/executed decision.
		if approved {
			existing, err := pendingV3ReviewDuplicateTx(tx, in.ProjectID, in.AdminID, digest)
			if err != nil {
				return err
			}
			if existing != 0 {
				id = existing
				return nil
			}
		}
		// Multiple immutable drafts may exist. Approval serializes on the
		// project lock and fences unexpired/executed decisions of this version.
		expires := time.Now().UTC().Add(v3PolicyReviewTTL)
		var memberDigest, snapshotDigest any
		if preview.Enabled {
			memberDigest, snapshotDigest = strings.ToLower(candidate.MembersDigest.Hex()), strings.ToLower(candidate.Digest.Hex())
		}
		insert := tx.Exec(`INSERT INTO ido_v3_policy_review (chain_id,proxy_address,project_id,expected_version,target_version,
			base_state,base_enabled,base_root,base_block,base_hash,preview_digest,enabled,root,members_digest,snapshot_digest,
			member_count,reason,initiator_admin_id,initiator_mfa_digest,expires_at,created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6))`, scope.ChainID.Uint64(), preview.Proxy, in.ProjectID,
			mustParseV3Version(preview.OldVersion), mustParseV3Version(preview.TargetVersion), preview.State, preview.OldEnabled, preview.OldRoot,
			block, preview.BlockHash, digest, preview.Enabled, strings.ToLower(preview.Root), memberDigest, snapshotDigest,
			preview.MemberCount, preview.Reason, in.AdminID, nil, expires.UTC().Format("2006-01-02 15:04:05.999999"))
		if insert.Error != nil || insert.RowsAffected != 1 {
			return invalid
		}
		if tx.Raw(`SELECT LAST_INSERT_ID()`).Scan(&id).Error != nil || id == 0 {
			return invalid
		}
		for i, member := range treeMembersForV3(scope, preview.CanonicalMembers) {
			if tx.Exec(`INSERT INTO ido_v3_policy_review_member (review_id,leaf_index,wallet_address,leaf_hash) VALUES (?,?,?,?)`,
				id, i, member.Wallet, member.Leaf).Error != nil {
				return invalid
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

func pendingV3ReviewDuplicateTx(tx *gorm.DB, projectID, adminID uint64, digest string) (uint64, error) {
	var rows []struct{ ID uint64 }
	if tx == nil || projectID == 0 || adminID == 0 || !v3CanonicalHash(digest) {
		return 0, errors.New("V3 review duplicate identity unavailable")
	}
	err := tx.Raw(`SELECT r.id FROM ido_v3_policy_review r WHERE r.project_id=? AND r.initiator_admin_id=?
 AND BINARY r.preview_digest=BINARY ? AND r.expires_at>UTC_TIMESTAMP(6)
 AND r.created_at>UTC_TIMESTAMP(6)-INTERVAL 24 HOUR
 AND NOT EXISTS(SELECT 1 FROM ido_v3_policy_review_decision d WHERE d.review_id=r.id)
 AND NOT EXISTS(SELECT 1 FROM ido_v3_policy_review_execution e WHERE e.review_id=r.id)
 ORDER BY r.id LIMIT 2`, projectID, adminID, digest).Scan(&rows).Error
	if err != nil || len(rows) > 1 || len(rows) == 1 && rows[0].ID == 0 {
		return 0, errors.New("V3 review duplicate state unavailable")
	}
	if len(rows) == 1 {
		p, err := V3PolicyReviewReadTx(tx, projectID, rows[0].ID)
		if err != nil || p.PreviewDigest != digest {
			return 0, errors.New("V3 review duplicate content changed")
		}
		return rows[0].ID, nil
	}
	return 0, nil
}

type v3ReviewMember struct{ Wallet, Leaf string }

func treeMembersForV3(scope idov3whitelist.Scope, wallets []string) []v3ReviewMember {
	members := make([]v3ReviewMember, 0, len(wallets))
	for _, wallet := range wallets {
		leaf, _ := idov3whitelist.Leaf(scope, common.HexToAddress(wallet))
		members = append(members, v3ReviewMember{wallet, strings.ToLower(leaf.Hex())})
	}
	return members
}

func v3PolicyMemberDiff(before, after []string) ([]string, []string) {
	old, next := make(map[string]bool, len(before)), make(map[string]bool, len(after))
	for _, wallet := range before {
		old[wallet] = true
	}
	for _, wallet := range after {
		next[wallet] = true
	}
	added, removed := []string{}, []string{}
	for _, wallet := range after {
		if !old[wallet] {
			added = append(added, wallet)
		}
	}
	for _, wallet := range before {
		if !next[wallet] {
			removed = append(removed, wallet)
		}
	}
	return added, removed
}

func v3SameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The immediate previous version must have been approved, executed and
// published by the scanner; no earlier draft or orphaned observation is a
// membership baseline. The caller additionally checks the live chain root.
func v3PolicyPriorMembersTx(tx *gorm.DB, p V3PolicyPreview, projectID uint64) ([]string, error) {
	invalid := errors.New("V3 prior policy unavailable")
	if tx == nil || !p.OldEnabled || !viper.GetBool("idoV3PolicyJournal.enabled") {
		return nil, invalid
	}
	var prior []struct {
		ID             uint64 `gorm:"column:id"`
		TxHash         string `gorm:"column:tx_hash"`
		ReceiptBlock   uint64 `gorm:"column:receipt_block"`
		ReceiptHash    string `gorm:"column:receipt_hash"`
		Count          uint32 `gorm:"column:member_count"`
		MembersDigest  string `gorm:"column:members_digest"`
		SnapshotDigest string `gorm:"column:snapshot_digest"`
	}
	read := tx.Raw(`SELECT r.id,e.tx_hash,e.receipt_block,e.receipt_hash,r.member_count,r.members_digest,r.snapshot_digest FROM ido_v3_policy_review r
		JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.decision='approved'
		JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
		JOIN ido_v3_policy_event_journal j ON j.chain_id=r.chain_id AND BINARY j.proxy_address=BINARY r.proxy_address
			AND j.project_id=r.project_id AND j.version=r.target_version AND BINARY j.root=BINARY r.root
			AND BINARY j.tx_hash=BINARY e.tx_hash AND j.block_number=e.receipt_block
			AND BINARY j.block_hash=BINARY e.receipt_hash AND j.enabled=1 AND j.state='canonical'
		WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version=?
			AND r.enabled=1 AND BINARY r.root=BINARY ? LIMIT 2`, mustParseV3Version(p.ChainID), p.Proxy, projectID,
		mustParseV3Version(p.OldVersion), p.OldRoot).Scan(&prior)
	if read.Error != nil || len(prior) != 1 || prior[0].Count == 0 || prior[0].Count > 512 ||
		!v3CanonicalHash(prior[0].TxHash) || !v3CanonicalHash(prior[0].ReceiptHash) || prior[0].ReceiptBlock == 0 {
		return nil, invalid
	}
	var rows []struct {
		Index  uint32 `gorm:"column:leaf_index"`
		Wallet string `gorm:"column:wallet_address"`
		Leaf   string `gorm:"column:leaf_hash"`
	}
	read = tx.Raw(`SELECT leaf_index,wallet_address,leaf_hash FROM ido_v3_policy_review_member WHERE review_id=? ORDER BY leaf_index`, prior[0].ID).Scan(&rows)
	if read.Error != nil || len(rows) != int(prior[0].Count) {
		return nil, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(mustParseV3Version(p.ChainID)),
		ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: common.HexToAddress(p.Proxy)}
	wallets := make([]string, len(rows))
	for i, row := range rows {
		leaf, err := idov3whitelist.Leaf(scope, common.HexToAddress(row.Wallet))
		if row.Index != uint32(i) || row.Wallet != strings.ToLower(row.Wallet) || !common.IsHexAddress(row.Wallet) ||
			err != nil || row.Leaf != strings.ToLower(leaf.Hex()) {
			return nil, invalid
		}
		wallets[i] = row.Wallet
	}
	tree, err := idov3whitelist.Build(scope, wallets)
	if err != nil {
		return nil, invalid
	}
	for i, member := range tree.Members() {
		if wallets[i] != strings.ToLower(member.Wallet.Hex()) {
			return nil, invalid
		}
	}
	commitment, err := tree.CommitSnapshot(scope, mustParseV3Version(p.OldVersion))
	if err != nil || strings.ToLower(commitment.Root.Hex()) != p.OldRoot ||
		strings.ToLower(commitment.MembersDigest.Hex()) != prior[0].MembersDigest ||
		strings.ToLower(commitment.Digest.Hex()) != prior[0].SnapshotDigest {
		return nil, invalid
	}
	return wallets, nil
}

// Read the previous receipt again from the original hash, not from a
// mutable current root or the journal alone. This also gates the precise
// prior member list on canonical placement and the pinned V3 runtime.
func v3VerifyPriorReceipt(ctx context.Context, rpc *ethclient.Client, admitted idov3whitelist.AdmittedProject, p V3PolicyPreview, projectID uint64) error {
	invalid := errors.New("V3 prior policy receipt unavailable")
	var previous []struct {
		Tx    string `gorm:"column:tx_hash"`
		Block uint64 `gorm:"column:receipt_block"`
		Hash  string `gorm:"column:receipt_hash"`
	}
	read := db.Mysql().WithContext(ctx).Raw(`SELECT e.tx_hash,e.receipt_block,e.receipt_hash FROM ido_v3_policy_review r
		JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.decision='approved'
		JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
		JOIN ido_v3_policy_event_journal j ON j.chain_id=r.chain_id AND BINARY j.proxy_address=BINARY r.proxy_address
		AND j.project_id=r.project_id AND j.version=r.target_version AND BINARY j.root=BINARY r.root
		AND BINARY j.tx_hash=BINARY e.tx_hash AND j.block_number=e.receipt_block
		AND BINARY j.block_hash=BINARY e.receipt_hash AND j.enabled=1 AND j.state='canonical'
		WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version=?
		AND r.enabled=1 AND BINARY r.root=BINARY ? LIMIT 2`, admitted.Binding.ChainID, p.Proxy, projectID,
		mustParseV3Version(p.OldVersion), p.OldRoot).Scan(&previous)
	if read.Error != nil || len(previous) != 1 || !v3CanonicalHash(previous[0].Tx) ||
		!v3CanonicalHash(previous[0].Hash) || previous[0].Block == 0 {
		return invalid
	}
	anchor, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(previous[0].Block))
	if err != nil || anchor == nil || anchor.Number == nil || !anchor.Number.IsUint64() ||
		anchor.Number.Uint64() != previous[0].Block || strings.ToLower(anchor.Hash().Hex()) != previous[0].Hash {
		return invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, common.HexToHash(previous[0].Tx))
	if err != nil || receipt == nil || receipt.Status != 1 || receipt.TxHash != common.HexToHash(previous[0].Tx) ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != previous[0].Block ||
		receipt.BlockHash != anchor.Hash() {
		return invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: admitted.Binding.ProxyAddress}
	observed, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, previous[0].Block, anchor.Hash())
	if err != nil || observed.Policy.Version != mustParseV3Version(p.OldVersion) || !observed.Policy.Enabled ||
		strings.ToLower(observed.Policy.Root.Hex()) != p.OldRoot {
		return invalid
	}
	// The log in the canonical original receipt must itself encode this
	// predecessor's enabled/version/root, not merely be a successful tx.
	var events int
	for _, log := range receipt.Logs {
		if log == nil || log.Address != scope.ProxyAddress || log.Removed || log.TxHash != receipt.TxHash ||
			log.BlockHash != receipt.BlockHash || log.BlockNumber != previous[0].Block || len(log.Topics) == 0 ||
			log.Topics[0] != v3ReviewPolicyEventTopic {
			continue
		}
		if len(log.Topics) != 3 || log.Topics[1] != common.BigToHash(scope.ProjectID) {
			return invalid
		}
		events++
		if log.Topics[2] != common.BigToHash(new(big.Int).SetUint64(mustParseV3Version(p.OldVersion))) ||
			len(log.Data) != 64 || new(big.Int).SetBytes(log.Data[:32]).Cmp(big.NewInt(1)) != 0 ||
			common.BytesToHash(log.Data[32:]) != common.HexToHash(p.OldRoot) {
			return invalid
		}
	}
	if events != 1 {
		return invalid
	}
	again, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(previous[0].Block))
	if err != nil || again == nil || again.Hash() != anchor.Hash() {
		return invalid
	}
	return nil
}

// Review reads only the append-only candidate and complete member set. It
// never treats 1014 candidate rows as authorization for a policy change.
func V3PolicyReviewReadTx(tx *gorm.DB, projectID, reviewID uint64) (V3PolicyPreview, error) {
	invalid := errors.New("V3 policy review unavailable")
	if tx == nil || projectID == 0 || reviewID == 0 {
		return V3PolicyPreview{}, invalid
	}
	var rows []struct {
		ChainID        uint64    `gorm:"column:chain_id"`
		Proxy          string    `gorm:"column:proxy_address"`
		ProjectID      uint64    `gorm:"column:project_id"`
		Expected       uint64    `gorm:"column:expected_version"`
		Target         uint64    `gorm:"column:target_version"`
		BaseEnabled    uint8     `gorm:"column:base_enabled"`
		BaseState      string    `gorm:"column:base_state"`
		BaseRoot       string    `gorm:"column:base_root"`
		BaseBlock      uint64    `gorm:"column:base_block"`
		BaseHash       string    `gorm:"column:base_hash"`
		Digest         string    `gorm:"column:preview_digest"`
		Enabled        uint8     `gorm:"column:enabled"`
		Root           string    `gorm:"column:root"`
		MembersDigest  string    `gorm:"column:members_digest"`
		SnapshotDigest string    `gorm:"column:snapshot_digest"`
		Count          uint32    `gorm:"column:member_count"`
		Reason         string    `gorm:"column:reason"`
		Initiator      uint64    `gorm:"column:initiator_admin_id"`
		Expiry         time.Time `gorm:"column:expires_at"`
	}
	read := tx.Raw(`SELECT chain_id,proxy_address,project_id,expected_version,target_version,base_state,base_enabled,base_root,
		base_block,base_hash,preview_digest,enabled,root,members_digest,snapshot_digest,member_count,reason,
		initiator_admin_id,expires_at FROM ido_v3_policy_review WHERE id=? AND project_id=? LIMIT 2`, reviewID, projectID).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].ChainID == 0 || rows[0].ProjectID != projectID || rows[0].BaseBlock == 0 ||
		rows[0].BaseEnabled > 1 || rows[0].Enabled > 1 || rows[0].Initiator == 0 || rows[0].Count > 512 ||
		rows[0].Expected == ^uint64(0) || rows[0].Target != rows[0].Expected+1 ||
		rows[0].Proxy != strings.ToLower(rows[0].Proxy) || !common.IsHexAddress(rows[0].Proxy) ||
		(rows[0].BaseState != "pending" && rows[0].BaseState != "active") ||
		(rows[0].BaseState == "active" && (rows[0].BaseEnabled != 1 || rows[0].Enabled != 1)) ||
		(rows[0].Enabled == 0 && (rows[0].Count != 0 || rows[0].SnapshotDigest != "" || rows[0].MembersDigest != "")) ||
		(rows[0].Enabled == 1 && rows[0].Count == 0) {
		return V3PolicyPreview{}, invalid
	}
	var members []struct {
		Index  uint32 `gorm:"column:leaf_index"`
		Wallet string `gorm:"column:wallet_address"`
		Leaf   string `gorm:"column:leaf_hash"`
	}
	if tx.Raw(`SELECT leaf_index,wallet_address,leaf_hash FROM ido_v3_policy_review_member WHERE review_id=? ORDER BY leaf_index`, reviewID).Scan(&members).Error != nil || len(members) != int(rows[0].Count) {
		return V3PolicyPreview{}, invalid
	}
	r := rows[0]
	p := V3PolicyPreview{ChainID: strconv.FormatUint(r.ChainID, 10), ProjectID: strconv.FormatUint(r.ProjectID, 10),
		Proxy: r.Proxy, Block: strconv.FormatUint(r.BaseBlock, 10), BlockHash: r.BaseHash, State: r.BaseState,
		OldEnabled: r.BaseEnabled == 1, OldRoot: r.BaseRoot, OldVersion: strconv.FormatUint(r.Expected, 10), Enabled: r.Enabled == 1,
		TargetVersion: strconv.FormatUint(r.Target, 10), Root: r.Root, MembersDigest: r.MembersDigest, SnapshotDigest: r.SnapshotDigest,
		MemberCount: int(r.Count), Reason: r.Reason, CanonicalMembers: make([]string, len(members)), AddedMembers: []string{}, RemovedMembers: []string{}}
	for i, m := range members {
		leaf, leafErr := idov3whitelist.Leaf(idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(r.ChainID),
			ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: common.HexToAddress(r.Proxy)}, common.HexToAddress(m.Wallet))
		if m.Index != uint32(i) || !common.IsHexAddress(m.Wallet) || m.Wallet != strings.ToLower(m.Wallet) ||
			leafErr != nil || m.Leaf != strings.ToLower(leaf.Hex()) {
			return V3PolicyPreview{}, invalid
		}
		p.CanonicalMembers[i] = m.Wallet
	}
	if !p.OldEnabled && p.Enabled {
		p.AddedMembers = append(p.AddedMembers, p.CanonicalMembers...)
	} else if p.OldEnabled {
		// An immutable previous approved snapshot, not a candidate or a
		// currently observed root, is the only diff baseline.
		baseline, err := v3PolicyPriorMembersTx(tx, p, projectID)
		if err != nil {
			return V3PolicyPreview{}, invalid
		}
		p.AddedMembers, p.RemovedMembers = v3PolicyMemberDiff(baseline, p.CanonicalMembers)
	}
	if _, _, err := v3PolicyIdentity(p, projectID); err != nil {
		return V3PolicyPreview{}, invalid
	}
	digest, err := V3PolicyPreviewDigest(p)
	if err != nil || digest != r.Digest {
		return V3PolicyPreview{}, invalid
	}
	p.PreviewDigest = digest
	return p, nil
}

// V3PolicyReviewRead exposes the immutable complete proposal only to a live
// project reader. It is not a current-chain or proof eligibility endpoint.
func V3PolicyReviewRead(projectID, reviewID, adminID uint64) (V3PolicyPreview, error) {
	invalid := errors.New("V3 policy review unavailable")
	if !viper.GetBool("idoV3PolicyReview.enabled") || projectID == 0 || reviewID == 0 || adminID == 0 || db.Mysql() == nil {
		return V3PolicyPreview{}, invalid
	}
	var preview V3PolicyPreview
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if _, err := middleware.HasLivePermissionTx(tx, int64(adminID), middleware.ModProject, middleware.ActRead); err != nil {
			return invalid
		}
		var err error
		preview, err = V3PolicyReviewReadTx(tx, projectID, reviewID)
		return err
	})
	if err != nil {
		return V3PolicyPreview{}, invalid
	}
	return preview, nil
}

func V3PolicyReviewApprove(projectID, reviewID, approverID uint64, previewDigest string) error {
	return v3PolicyReviewApproveWithAdmission(context.Background(), projectID, reviewID, approverID, previewDigest, false)
}

// ApproveApprovedV3PolicyReview requires a distinct, live Admin approver;
// the decision cannot replace a Policy Safe execution or wallet proof.
func ApproveApprovedV3PolicyReview(projectID, reviewID, approverID uint64, previewDigest string) error {
	return ApproveApprovedV3PolicyReviewContext(context.Background(), projectID, reviewID, approverID, previewDigest)
}

func ApproveApprovedV3PolicyReviewContext(ctx context.Context, projectID, reviewID, approverID uint64, previewDigest string) error {
	if err := requireV3PolicyReviewNoMFASchema(ctx, db.Mysql()); err != nil {
		return err
	}
	return v3PolicyReviewApproveWithAdmission(ctx, projectID, reviewID, approverID, previewDigest, true)
}

func v3PolicyReviewApproveWithAdmission(parent context.Context, projectID, reviewID, approverID uint64, previewDigest string, approved bool) (resultErr error) {
	phase := "review_read"
	defer func() {
		if resultErr != nil {
			resultErr = v3ReadDiagnostic(resultErr, phase)
		}
	}()
	invalid := errors.New("V3 policy review approval unavailable")
	if parent == nil || parent.Err() != nil || !viper.GetBool("idoV3PolicyReview.enabled") || projectID == 0 || reviewID == 0 || approverID == 0 ||
		!v3CanonicalHash(previewDigest) || db.Mysql() == nil {
		return invalid
	}
	// The first-enable status read can spend up to 120s on serial receipt and
	// canonical RPC checks. Bound the *whole* approval below the Admin 170s
	// client / 175s proxy deadlines; the subsequent 90s phase cannot stack
	// into an unbounded 210s request. Never continue after caller cancellation.
	if approved {
		var cancel context.CancelFunc
		parent, cancel = context.WithTimeout(parent, 165*time.Second)
		defer cancel()
	}
	// Obtain a fresh independently pinned policy before taking DB locks. A
	// failure/reorg means no decision may be inserted.
	var status V3PolicyStatus
	var err error
	var reviewed V3PolicyPreview
	if approved {
		readCtx, stop := context.WithTimeout(parent, 5*time.Second)
		err = db.Mysql().WithContext(readCtx).Transaction(func(tx *gorm.DB) error {
			var e error
			reviewed, e = V3PolicyReviewReadTx(tx, projectID, reviewID)
			return e
		})
		stop()
		if err != nil || reviewed.PreviewDigest != previewDigest {
			return invalid
		}
	}
	firstEnableReview := approved && reviewed.State == "pending" && !reviewed.OldEnabled && reviewed.OldVersion == "0" && reviewed.Enabled
	phase = "status"
	if approved {
		if firstEnableReview {
			status, err = previewApprovedV3PolicyReviewStatusContext(parent, projectID)
		} else {
			status, err = PreviewApprovedV3PolicyStatusContext(parent, projectID)
		}
	} else {
		status, err = ReadV3PolicyStatus(projectID)
	}
	if err != nil || (status.ProjectState != "pending" && status.ProjectState != "active") {
		if err != nil {
			return err
		}
		return v3diagnostic.New("STATE_CHANGED", "status", invalid)
	}
	budget := 12 * time.Second
	if approved {
		budget = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	var admitted idov3whitelist.AdmittedProject
	var ownerHash, governorHash common.Hash
	if approved {
		ownerHash, governorHash, err = approvedV3SafeHashes()
		if err != nil {
			return invalid
		}
	} else {
		admitted, _, _, _, _, err = pinnedV3Admission(projectID, false)
		if err != nil {
			return invalid
		}
	}
	var rpc *ethclient.Client
	if approved {
		rpc, err = v3TokenGateRPC(ctx)
	} else {
		rpc, err = chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	}
	if err != nil {
		return invalid
	}
	defer rpc.Close()
	var reviewSnapshot idov3whitelist.ApprovedLockedCandidate
	if approved {
		admitted, reviewSnapshot, err = chaintx.AdmitApprovedLockedProjectForPolicyReview(ctx, db.Mysql(), rpc, projectID, ownerHash, governorHash)
		if err != nil || requireV3ReviewOrStrictScan(ctx, db.Mysql(), firstEnableReview, projectID) != nil {
			if err != nil {
				return v3ReadDiagnostic(err, "project_admission")
			}
			return v3diagnostic.New("UNAVAILABLE", "policy_evidence_store", invalid)
		}
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID), ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: admitted.Binding.ProxyAddress}
	phase = "live_state"
	liveHead, err := rpc.BlockNumber(ctx)
	if err != nil {
		return v3ReadDiagnostic(err, phase)
	}
	liveHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(liveHead))
	if err != nil || liveHeader == nil || liveHeader.Number == nil || !liveHeader.Number.IsUint64() || liveHeader.Number.Uint64() != liveHead {
		return invalid
	}
	livePolicy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, liveHead, liveHeader.Hash())
	if err != nil || livePolicy.Policy.Version != mustParseV3Version(status.Version) || livePolicy.Policy.Enabled != status.Enabled ||
		strings.ToLower(livePolicy.Policy.Root.Hex()) != status.Root {
		return invalid
	}
	liveProject, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, liveHead, liveHeader.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, liveProject) != nil ||
		(status.ProjectState == "pending" && liveProject.State != idov3whitelist.ProjectPending) ||
		(status.ProjectState == "active" && liveProject.State != idov3whitelist.ProjectActive) {
		return invalid
	}
	liveAgain, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(liveHead))
	if err != nil || liveAgain == nil || liveAgain.Hash() != liveHeader.Hash() {
		return invalid
	}
	if now, e := rpc.BlockNumber(ctx); e != nil || !v3PolicyPreviewHeadAdmitted(liveHead, now, firstEnableReview) {
		return invalid
	}
	// Revalidate the previous version's original receipt after the independent
	// current-chain read, before committing a second administrator's decision.
	if db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		reviewed, e = V3PolicyReviewReadTx(tx, projectID, reviewID)
		return e
	}) != nil {
		return invalid
	}
	if reviewed.OldEnabled && v3VerifyPriorReceipt(ctx, rpc, admitted, reviewed, projectID) != nil {
		return invalid
	}
	if approved && reviewed.OldEnabled && verifyApprovedV3PriorPolicy(ctx, db.Mysql(), rpc, admitted, reviewed, projectID) != nil {
		return invalid
	}
	if reviewed.PreviewDigest != previewDigest || reviewed.State != status.ProjectState ||
		reviewed.OldVersion != status.Version || reviewed.OldRoot != status.Root || reviewed.OldEnabled != status.Enabled {
		return invalid
	}
	if approved {
		if chaintx.RecheckApprovedLockedPreview(ctx, db.Mysql(), rpc, projectID, reviewSnapshot) != nil ||
			requireV3PolicyEvidenceStore(ctx, db.Mysql(), projectID) != nil {
			return invalid
		}
	}
	phase = "approval_commit"
	return db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, status.Proxy) != nil || chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		if approved {
			set, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, projectID)
			if e != nil || set.Identity.ProjectID != projectID || set.Identity.Proxy != status.Proxy {
				return invalid
			}
		}
		var superseding int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
			WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version>?`,
			admitted.Binding.ChainID, status.Proxy, projectID, mustParseV3Version(status.Version)).Scan(&superseding).Error != nil || superseding != 0 {
			return invalid
		}
		var projects []struct {
			ID uint64 `gorm:"column:project_id"`
		}
		if read := tx.Raw(`SELECT project_id FROM projects WHERE project_id=? AND chain_id=? AND BINARY contract_address=BINARY ?
			AND funding_mode='capped_presale' AND contract_version='ido_v3_stablecoin' AND status=?
			AND onchain_locked=1 LIMIT 2 FOR UPDATE`, projectID, admitted.Binding.ChainID, status.Proxy, strings.ToUpper(status.ProjectState[:1])+status.ProjectState[1:]).Scan(&projects); read.Error != nil || len(projects) != 1 {
			return invalid
		}
		p, err := V3PolicyReviewReadTx(tx, projectID, reviewID)
		if err != nil || p.PreviewDigest != previewDigest || p.Proxy != status.Proxy || p.State != status.ProjectState || p.OldVersion != status.Version || p.OldRoot != status.Root ||
			p.ChainID != status.ChainID || p.OldEnabled != status.Enabled {
			return invalid
		}
		if p.OldEnabled {
			prior, e := v3PolicyPriorMembersTx(tx, p, projectID)
			if e != nil {
				return invalid
			}
			added, removed := v3PolicyMemberDiff(prior, p.CanonicalMembers)
			if !v3SameMembers(added, p.AddedMembers) || !v3SameMembers(removed, p.RemovedMembers) {
				return invalid
			}
		}
		base, parseErr := strconv.ParseUint(p.Block, 10, 64)
		header, headerErr := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(base))
		if parseErr != nil || base == 0 || headerErr != nil || header == nil || header.Number == nil ||
			!header.Number.IsUint64() || header.Number.Uint64() != base || strings.ToLower(header.Hash().Hex()) != p.BlockHash {
			return invalid
		}
		var initiator []struct {
			ID      uint64    `gorm:"column:initiator_admin_id"`
			Expires time.Time `gorm:"column:expires_at"`
			Created time.Time `gorm:"column:created_at"`
		}
		if read := tx.Raw(`SELECT initiator_admin_id,expires_at,created_at FROM ido_v3_policy_review WHERE id=? LIMIT 2`, reviewID).Scan(&initiator); read.Error != nil || len(initiator) != 1 {
			return invalid
		}
		if initiator[0].ID == approverID {
			return v3diagnostic.New("DISTINCT_APPROVER_REQUIRED", "approval_commit", nil)
		}
		if !v3PolicyReviewApprovalOpen(v3UTCWallTime(initiator[0].Created), v3UTCWallTime(initiator[0].Expires), time.Now().UTC()) {
			return v3diagnostic.New("REVIEW_EXPIRED", "approval_commit", nil)
		}
		if _, err := middleware.HasLivePermissionTx(tx, int64(initiator[0].ID), middleware.ModProject, middleware.ActWrite); err != nil {
			return v3diagnostic.New("PERMISSION_DENIED", "approval_commit", err)
		}
		if _, err := middleware.HasLivePermissionTx(tx, int64(approverID), middleware.ModProject, middleware.ActApprove); err != nil {
			return v3diagnostic.New("PERMISSION_DENIED", "approval_commit", err)
		}
		// The project row is locked above. An expired approved decision can be
		// superseded, but never an unexpired approval or an executed version.
		// The database trigger independently enforces the same fence for any
		// other writer; no old decision or receipt is modified/deleted.
		var competing int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r
			JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.decision='approved'
			WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=?
			AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin' AND r.target_version=?
			AND r.id<>? AND (r.expires_at>UTC_TIMESTAMP(6) OR EXISTS
			(SELECT 1 FROM ido_v3_policy_review_execution e WHERE e.review_id=r.id))`,
			admitted.Binding.ChainID, status.Proxy, projectID, mustParseV3Version(p.TargetVersion), reviewID).
			Scan(&competing).Error != nil || competing != 0 {
			return invalid
		}
		var count int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review_decision WHERE review_id=?`, reviewID).Scan(&count).Error != nil || count != 0 {
			return invalid
		}
		insert := tx.Exec(`INSERT INTO ido_v3_policy_review_decision
			(review_id,chain_id,proxy_address,project_id,target_version,approver_admin_id,approver_mfa_digest,decision)
			VALUES (?,?,?,?,?,?,?,'approved')`, reviewID, admitted.Binding.ChainID, status.Proxy, projectID,
			mustParseV3Version(p.TargetVersion), approverID, nil)
		if insert.Error != nil || insert.RowsAffected != 1 {
			return invalid
		}
		return nil
	})
}

func mustParseV3Version(s string) uint64 { v, _ := strconv.ParseUint(s, 10, 64); return v }

// MySQL's DATETIME has no zone and the shared DSN parses it in Local. The
// review migration stores UTC wall-clock fields, not local instants.
func v3UTCWallTime(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

// The only complete and canonical reviewed membership list is the newly
// reviewed immutable snapshot; old 1014 candidate rows are never consulted.
// A disabled review deliberately has no commitment and cannot authorize a proof.
func V3PolicyReviewCommitment(projectID, reviewID uint64, tx *gorm.DB) (idov3whitelist.SnapshotCommitment, error) {
	p, err := V3PolicyReviewReadTx(tx, projectID, reviewID)
	if err != nil {
		return idov3whitelist.SnapshotCommitment{}, err
	}
	_, commitment, err := v3PolicyIdentity(p, projectID)
	return commitment, err
}
