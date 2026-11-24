package project

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/db"
)

// approvedV3SafeHashes is a deployment trust root, never loaded from an
// approval, project row, Safe proposal, or RPC response. All consumers of
// approval-backed policy evidence must use the same pinned code identities.
func approvedV3SafeHashes() (common.Hash, common.Hash, error) {
	return idov3whitelist.ParseApprovedV3SafeCodeHashes(
		strings.TrimSpace(viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash")),
		strings.TrimSpace(viper.GetString("idoV3TokenAdmission.governorSafeCodeHash")))
}

// readApprovedV3PolicyStatus is the gated Core reader. A Set approval,
// candidate runtime or observed root alone cannot disclose a wallet proof.
// This checks the original applied creation, running canonical cursor, pinned
// Safe/runtime and current project policy on every request; it never writes.
func readApprovedV3PolicyStatus(ctx context.Context, database *gorm.DB, rpc *ethclient.Client,
	projectID uint64, ownerHash, governorHash common.Hash) (V3PolicyStatus, error) {
	return readApprovedV3PolicyStatusForReview(ctx, database, rpc, projectID, ownerHash, governorHash, false)
}

// Policy changes tolerate independently authenticated forward cursor progress.
// Root/version and approved member evidence, not whole-project financial TTL,
// establish policy authority. firstEnableReview retains its narrower v0 scope.
func readApprovedV3PolicyStatusForReview(ctx context.Context, database *gorm.DB, rpc *ethclient.Client,
	projectID uint64, ownerHash, governorHash common.Hash, firstEnableReview bool) (result V3PolicyStatus, resultErr error) {
	invalid := errors.New("V3 approved policy status unavailable")
	started, phase, complete := time.Now(), "input", false
	defer func() {
		if resultErr != nil {
			resultErr = v3PolicyContextDiagnostic(ctx, resultErr, phase)
		}
	}()
	defer func() {
		if firstEnableReview && !complete {
			if logger := log.GetLogger(); logger != nil {
				logger.WithFields(map[string]interface{}{
					"phase": phase, "elapsed_ms": time.Since(started).Milliseconds(),
					"deadline": ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded),
					"canceled": ctx != nil && errors.Is(ctx.Err(), context.Canceled),
				}).Warn("V3 first-enable review status unavailable")
			}
		}
	}()
	if ctx == nil || database == nil || rpc == nil || projectID == 0 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return V3PolicyStatus{}, invalid
	}
	var admitted idov3whitelist.AdmittedProject
	var snapshot idov3whitelist.ApprovedLockedCandidate
	var err error
	phase = "admission"
	if e := v3PolicyScanDiagnostic(ctx, database, projectID); e != nil {
		return V3PolicyStatus{}, e
	}
	admitted, snapshot, err = chaintx.AdmitApprovedLockedProjectForPolicyReview(ctx, database, rpc, projectID, ownerHash, governorHash)
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	phase = "policy_evidence_store"
	if requireV3ReviewOrStrictScan(ctx, database, firstEnableReview, projectID) != nil {
		return V3PolicyStatus{}, v3diagnostic.New("UNAVAILABLE", phase, invalid)
	}
	phase = "confirmed_head"
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!idov3whitelist.HasConfirmations(head.Number.Uint64(), admitted.CreationBlock, 15) {
		return V3PolicyStatus{}, invalid
	}
	height := head.Number.Uint64() - 14
	block := new(big.Int).SetUint64(height)
	anchor, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || anchor == nil || anchor.Number == nil || anchor.Number.Cmp(block) != 0 {
		return V3PolicyStatus{}, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: admitted.Binding.ProxyAddress}
	phase = "project_policy"
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, anchor.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil {
		return V3PolicyStatus{}, invalid
	}
	policy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, height, anchor.Hash())
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	if firstEnableReview && !approvedV3UnscannedInitialCandidate(observed.State, policy.Policy, true) {
		return V3PolicyStatus{}, invalid
	}
	governance := idov3whitelist.GovernanceIdentity{
		FollowOwner: true, Governor: common.HexToAddress(v3DiagnosticGovernor),
		OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash,
	}
	phase = "governance"
	if _, err := idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime,
		governance, height, anchor.Hash()); err != nil {
		return V3PolicyStatus{}, invalid
	}
	stable, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || stable == nil || stable.Number == nil || stable.Number.Cmp(block) != 0 || stable.Hash() != anchor.Hash() {
		return V3PolicyStatus{}, invalid
	}
	phase = "final_recheck"
	// The second admission detects a replaced creation/cursor, approval row or
	// concurrent scanner halt while the multi-round-trip policy read ran.
	if chaintx.RecheckApprovedLockedPreview(ctx, database, rpc, projectID, snapshot) != nil ||
		requireV3PolicyEvidenceStore(ctx, database, projectID) != nil {
		return V3PolicyStatus{}, invalid
	}
	// Also reject a newer unconfirmed policy or project transition. A confirmed
	// base alone must not authorize an approval/handoff for an obsolete version.
	states := []string{"pending", "active", "successful", "failed", "cancelled"}
	if int(observed.State) >= len(states) {
		return V3PolicyStatus{}, invalid
	}
	if checkApprovedV3ObservationCurrentPolicy(ctx, rpc, scope, admitted, V3PolicyPreview{State: states[observed.State], Root: strings.ToLower(policy.Policy.Root.Hex()), TargetVersion: strconv.FormatUint(policy.Policy.Version, 10), Enabled: policy.Policy.Enabled}) != nil {
		return V3PolicyStatus{}, invalid
	}
	complete = true
	return V3PolicyStatus{ChainID: "56", ProjectID: strconv.FormatUint(projectID, 10),
		Proxy: strings.ToLower(admitted.Binding.ProxyAddress.Hex()), Block: strconv.FormatUint(height, 10),
		BlockHash: strings.ToLower(anchor.Hash().Hex()), ProjectState: states[observed.State],
		Enabled: policy.Policy.Enabled, Root: strings.ToLower(policy.Policy.Root.Hex()),
		Version: strconv.FormatUint(policy.Policy.Version, 10), ApprovalState: "unavailable_for_proof"}, nil
}

func requireV3ReviewOrStrictScan(ctx context.Context, database *gorm.DB, firstEnableReview bool, projectIDs ...uint64) error {
	id := uint64(0)
	if len(projectIDs) == 1 {
		id = projectIDs[0]
	}
	return requireV3PolicyEvidenceStore(ctx, database, id)
}

// PreviewApprovedV3PolicyStatus is an internal policy-control read. It does
// not publish member proofs; publication independently audits Safe execution.
func PreviewApprovedV3PolicyStatus(projectID uint64) (V3PolicyStatus, error) {
	return PreviewApprovedV3PolicyStatusContext(context.Background(), projectID)
}

func PreviewApprovedV3PolicyStatusContext(parent context.Context, projectID uint64) (V3PolicyStatus, error) {
	return previewApprovedV3PolicyStatusContext(parent, projectID, false)
}

// First enable retains its special v0/Pending eligibility and review budget.
func previewApprovedV3PolicyReviewStatusContext(parent context.Context, projectID uint64) (V3PolicyStatus, error) {
	return previewApprovedV3PolicyStatusContext(parent, projectID, true)
}

func previewApprovedV3PolicyStatusContext(parent context.Context, projectID uint64, firstEnableReview bool) (V3PolicyStatus, error) {
	invalid := errors.New("V3 approved policy status unavailable")
	if parent == nil || parent.Err() != nil || !viper.GetBool("idoV3PolicyPreview.enabled") || projectID == 0 {
		return V3PolicyStatus{}, invalid
	}
	ownerHash, governorHash, err := approvedV3SafeHashes()
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	ctx, cancel := context.WithTimeout(parent, v3ApprovedPolicyStatusBudget(firstEnableReview))
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	defer rpc.Close()
	return readApprovedV3PolicyStatusForReview(ctx, db.Mysql(), rpc, projectID, ownerHash, governorHash, firstEnableReview)
}

// First-enable review audits a long, serial original receipt path. Its read
// remains bounded; public/strict status is intentionally unchanged. This
// never relaxes Safe execution or proof gates.
func v3ApprovedPolicyStatusBudget(firstEnableReview bool) time.Duration {
	if firstEnableReview {
		return 120 * time.Second
	}
	return 40 * time.Second
}
