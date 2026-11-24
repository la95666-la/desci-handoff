package project

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
)

const v3PolicyReviewRequestBudget = 150 * time.Second
const v3PolicyReviewFinalReserve = 35 * time.Second

// Explicitly owned by ONE review initiation. Never placed in a context, global
// cache, DB row, HTTP input or response. Approval, execution and proof readers
// cannot inherit it. The RPC and deadline are shared, not reopened per layer.
type v3PolicyReviewRequest struct {
	ctx             context.Context
	database        *gorm.DB
	rpc             *ethclient.Client
	projectID       uint64
	owner, governor common.Hash
	admitted        idov3whitelist.AdmittedProject
	snapshot        idov3whitelist.ApprovedLockedCandidate
	audited         bool
}

func (r *v3PolicyReviewRequest) admit(ctx context.Context, database *gorm.DB, rpc *ethclient.Client, id uint64, owner, governor common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	return r.admitWith(ctx, database, rpc, id, owner, governor, func() (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
		started := time.Now()
		a, s, err := chaintx.AdmitApprovedLockedProjectForPolicyReview(ctx, database, rpc, id, owner, governor)
		if l := log.GetLogger(); l != nil {
			l.WithFields(map[string]any{"phase": "project_admission", "elapsed_ms": time.Since(started).Milliseconds(), "success": err == nil}).Info("V3 policy review original audit")
		}
		return a, s, err
	})
}

func (r *v3PolicyReviewRequest) admitWith(ctx context.Context, database *gorm.DB, rpc *ethclient.Client, id uint64, owner, governor common.Hash,
	full func() (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error)) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
	if r == nil || r.ctx == nil || r.ctx.Err() != nil || ctx == nil || ctx.Err() != nil ||
		database == nil || rpc == nil || id == 0 || owner == (common.Hash{}) || governor == (common.Hash{}) || full == nil ||
		r.database != database || r.rpc != rpc || r.projectID != id || r.owner != owner || r.governor != governor {
		return idov3whitelist.AdmittedProject{}, idov3whitelist.ApprovedLockedCandidate{}, errors.New("V3 review request scope changed")
	}
	if !r.audited {
		a, s, err := full()
		if err != nil {
			return a, s, err
		}
		if a.Binding.ProjectID != id || a.Binding != s.Binding || a.Runtime != s.Runtime || s.SubmissionID == 0 {
			return a, s, errors.New("V3 review audit identity mismatch")
		}
		r.admitted, r.snapshot, r.audited = a, s, true
	}
	return r.admitted, r.snapshot, nil
}

// Do not lose cancellation behind an intentionally sanitized inner error. Keep
// an already precise phase, and never serialize the underlying RPC/SQL cause.
func v3PolicyContextDiagnostic(ctx context.Context, err error, phase string) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		if stage := v3diagnostic.Stage(err); stage != "unavailable" {
			phase = stage
		}
		return v3ReadDiagnostic(ctx.Err(), phase)
	}
	return v3ReadDiagnostic(err, phase)
}

func requireV3ReviewFinalBudget(ctx context.Context) error {
	if ctx == nil {
		return v3diagnostic.New("UNAVAILABLE", "final_recheck", nil)
	}
	if ctx.Err() != nil {
		return v3PolicyContextDiagnostic(ctx, ctx.Err(), "final_recheck")
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < v3PolicyReviewFinalReserve {
		return v3diagnostic.New("CHECK_TIMEOUT", "final_recheck", context.DeadlineExceeded)
	}
	return nil
}

// One current, hash-pinned observation replaces the nested full-admission
// status reader plus a second copy of the same live-policy reads. Canonical
// creation/cursor fences, governance and exact-call simulation remain mandatory.
func (r *v3PolicyReviewRequest) finalRecheck(p V3PolicyPreview) (resultErr error) {
	if r == nil {
		return errors.New("V3 review request unavailable")
	}
	phase, started := "final_recheck", time.Now()
	defer func() {
		resultErr = v3PolicyContextDiagnostic(r.ctx, resultErr, phase)
		if l := log.GetLogger(); l != nil {
			l.WithFields(map[string]any{"phase": phase, "elapsed_ms": time.Since(started).Milliseconds(), "success": resultErr == nil}).Info("V3 policy review final check")
		}
	}()
	invalid := errors.New("V3 review final check unavailable")
	if !r.audited || p.ProjectID != new(big.Int).SetUint64(r.projectID).String() || p.ChainID != "56" || p.Proxy != strings.ToLower(r.admitted.Binding.ProxyAddress.Hex()) {
		return invalid
	}
	if err := requireV3ReviewFinalBudget(r.ctx); err != nil {
		return err
	}
	phase = "final_project_admission"
	if err := chaintx.RecheckApprovedPolicyObservation(r.ctx, r.database, r.rpc, r.projectID, r.snapshot); err != nil {
		return err
	}
	if err := requireV3PolicyEvidenceStore(r.ctx, r.database, r.projectID); err != nil {
		return err
	}
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: new(big.Int).SetUint64(r.projectID), ProxyAddress: r.admitted.Binding.ProxyAddress}
	phase = "latest_chain_state"
	return verifyV3ReviewCurrent(r.ctx, r.rpc, scope, r.admitted, p, r.owner, r.governor)
}

type v3ReviewCurrentRPC interface {
	v3ObservationCurrentRPC
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
}

func verifyV3ReviewCurrent(ctx context.Context, rpc v3ReviewCurrentRPC, scope idov3whitelist.Scope, admitted idov3whitelist.AdmittedProject, p V3PolicyPreview, owner, governor common.Hash) error {
	invalid := errors.New("V3 current review policy unavailable")
	if ctx == nil || ctx.Err() != nil || rpc == nil {
		return invalid
	}
	height, err := rpc.BlockNumber(ctx)
	if err != nil {
		return err
	}
	n := new(big.Int).SetUint64(height)
	h, err := rpc.HeaderByNumber(ctx, n)
	if err != nil {
		return err
	}
	if h == nil || h.Number == nil || h.Number.Cmp(n) != 0 {
		return invalid
	}
	policy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, height, h.Hash())
	if err != nil {
		return err
	}
	if policy.Policy.Version != mustParseV3Version(p.OldVersion) || policy.Policy.Enabled != p.OldEnabled || strings.ToLower(policy.Policy.Root.Hex()) != p.OldRoot {
		return v3diagnostic.New("STATE_CHANGED", "current_policy", invalid)
	}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, h.Hash())
	if err != nil {
		return err
	}
	if idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil ||
		(p.State != "pending" && p.State != "active") ||
		(p.State == "pending" && observed.State != idov3whitelist.ProjectPending) ||
		(p.State == "active" && (observed.State != idov3whitelist.ProjectActive || !p.OldEnabled || !p.Enabled)) {
		return v3diagnostic.New("STATE_CHANGED", "project", invalid)
	}
	gov := idov3whitelist.GovernanceIdentity{FollowOwner: true, Governor: common.HexToAddress(v3DiagnosticGovernor), OwnerCodeHash: owner, GovernorCodeHash: governor}
	if _, err := idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime, gov, height, h.Hash()); err != nil {
		return v3diagnostic.New("SAFE_CHECK_FAILED", "governance", err)
	}
	_, candidate, err := v3PolicyIdentity(p, scope.ProjectID.Uint64())
	if err != nil {
		return err
	}
	change := idov3whitelist.PolicyChange{ExpectedVersion: mustParseV3Version(p.OldVersion), TargetVersion: mustParseV3Version(p.TargetVersion), Root: common.HexToHash(p.Root), Enabled: p.Enabled}
	if p.Enabled {
		change.Candidate = &candidate
	}
	data, err := idov3whitelist.PackPolicyChangeCall(scope, change)
	if err != nil {
		return err
	}
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{From: gov.Governor, To: &admitted.Binding.ProxyAddress, Data: data}, n)
	if err != nil || len(result) != 0 {
		return v3diagnostic.New("PREFLIGHT_REJECTED", "policy_simulation", err)
	}
	again, err := rpc.HeaderByNumber(ctx, n)
	if err != nil {
		return err
	}
	if again == nil || again.Number == nil || again.Number.Cmp(n) != 0 || again.Hash() != h.Hash() {
		return v3diagnostic.New("STATE_CHANGED", "final_anchor", invalid)
	}
	return ctx.Err()
}
