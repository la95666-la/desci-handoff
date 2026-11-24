package project

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/db"
)

// V3PolicyPreview is diagnostic, never an approval or Safe proposal. No
// member from a Warmup registration is consulted. A new decision must bind
// this exact digest, two live administrators and an independent chain receipt.
type V3PolicyPreview struct {
	ChainID          string   `json:"chain_id"`
	ProjectID        string   `json:"project_id"`
	Proxy            string   `json:"proxy"`
	Block            string   `json:"block"`
	BlockHash        string   `json:"block_hash"`
	State            string   `json:"state"`
	OldEnabled       bool     `json:"old_enabled"`
	OldRoot          string   `json:"old_root"`
	OldVersion       string   `json:"old_version"`
	TargetVersion    string   `json:"target_version"`
	Enabled          bool     `json:"enabled"`
	Root             string   `json:"root"`
	MembersDigest    string   `json:"members_digest"`
	SnapshotDigest   string   `json:"snapshot_digest"`
	CanonicalMembers []string `json:"canonical_members"`
	AddedMembers     []string `json:"added_members"`
	RemovedMembers   []string `json:"removed_members"`
	MemberCount      int      `json:"member_count"`
	Reason           string   `json:"reason"`
	PreviewDigest    string   `json:"preview_digest"`
}

// PreviewV3InitialPolicy retains the initial-enable entry point.
func PreviewV3InitialPolicy(projectID uint64, wallets []string, reason string) (V3PolicyPreview, error) {
	return PreviewV3PolicyChange(projectID, wallets, true, reason)
}

// PreviewV3PolicyChange accepts a complete replacement list, or an empty
// list for Pending disable. Enabled predecessors must have a complete,
// independently approved and scanned immutable member snapshot. Nothing
// from the inert 1014 candidates can become a diff baseline.
func PreviewV3PolicyChange(projectID uint64, wallets []string, enabled bool, reason string) (V3PolicyPreview, error) {
	return previewV3PolicyChangeAt(projectID, wallets, enabled, reason, 0, "")
}

// A submitted review recomputes the exact preview at its original confirmed
// block, rather than silently substituting a newer block and digest.
func previewV3PolicyChangeAt(projectID uint64, wallets []string, enabled bool, reason string, baseBlock uint64, baseHash string) (V3PolicyPreview, error) {
	return previewV3PolicyChangeWithAdmission(projectID, wallets, enabled, reason, baseBlock, baseHash, false, true)
}

// PreviewApprovedV3PolicyChange is not routed until review, execution and
// proof consumers all share this approval-backed admission. A preview is
// never a Safe signature or authority to publish a member root.
func PreviewApprovedV3PolicyChange(projectID uint64, wallets []string, enabled bool, reason string) (V3PolicyPreview, error) {
	return PreviewApprovedV3PolicyChangeContext(context.Background(), projectID, wallets, enabled, reason)
}

// The Admin request context cancels queued RPC reads when its caller leaves.
// The background wrapper above retains the existing internal call contract.
func PreviewApprovedV3PolicyChangeContext(parent context.Context, projectID uint64, wallets []string, enabled bool, reason string) (V3PolicyPreview, error) {
	return previewV3PolicyChangeWithContext(parent, projectID, wallets, enabled, reason, 0, "", true, false)
}

func approvedV3PolicyPreviewScope(requireFreshScan bool, baseBlock uint64) bool {
	return requireFreshScan == (baseBlock != 0)
}

func previewV3PolicyChangeWithAdmission(projectID uint64, wallets []string, enabled bool, reason string, baseBlock uint64, baseHash string, approved, requireFreshScan bool) (V3PolicyPreview, error) {
	return previewV3PolicyChangeWithContext(context.Background(), projectID, wallets, enabled, reason, baseBlock, baseHash, approved, requireFreshScan)
}

func previewV3PolicyChangeWithContext(parent context.Context, projectID uint64, wallets []string, enabled bool, reason string, baseBlock uint64, baseHash string, approved, requireFreshScan bool) (previewResult V3PolicyPreview, resultErr error) {
	return previewV3PolicyChangeForRequest(parent, projectID, wallets, enabled, reason, baseBlock, baseHash, approved, requireFreshScan, nil)
}

func previewV3PolicyChangeForRequest(parent context.Context, projectID uint64, wallets []string, enabled bool, reason string, baseBlock uint64, baseHash string, approved, requireFreshScan bool, request *v3PolicyReviewRequest) (previewResult V3PolicyPreview, resultErr error) {
	invalid := errors.New("V3 policy preview unavailable")
	// The Admin route collapses all failures to one safe error. This diagnostic
	// records only a finite phase and timing, never wallet data, the RPC URL,
	// provider errors or transaction contents. Review recomputation is included.
	phase, started, complete := "input", time.Now(), false
	var diagnosticCtx context.Context
	var cancelTrace context.CancelFunc
	defer func() {
		if cancelTrace != nil {
			defer cancelTrace()
		}
		if resultErr != nil {
			resultErr = v3PolicyContextDiagnostic(diagnosticCtx, resultErr, phase)
		}
	}()
	phaseStarted := started
	advance := func(next string) {
		if elapsed := time.Since(phaseStarted); approved && elapsed >= 2*time.Second {
			if logger := log.GetLogger(); logger != nil {
				logger.WithFields(map[string]interface{}{
					"phase": phase, "phase_ms": elapsed.Milliseconds(), "review": requireFreshScan,
				}).Warn("V3 policy preview slow phase")
			}
		}
		phase, phaseStarted = next, time.Now()
	}
	var traceCtx context.Context
	defer func() {
		// Capture the outcome before normal cleanup cancels the context;
		// otherwise every completed preview appears to be client-canceled.
		deadline := traceCtx != nil && errors.Is(traceCtx.Err(), context.DeadlineExceeded)
		canceled := traceCtx != nil && errors.Is(traceCtx.Err(), context.Canceled)
		if logger := log.GetLogger(); logger != nil && approved {
			fields := map[string]interface{}{
				"phase": phase, "elapsed_ms": time.Since(started).Milliseconds(), "phase_ms": time.Since(phaseStarted).Milliseconds(),
				"review": requireFreshScan, "complete": complete,
				"deadline": deadline, "canceled": canceled,
			}
			if !complete || time.Since(started) >= 10*time.Second {
				logger.WithFields(fields).Warn("V3 policy preview trace")
			}
		}
	}()
	if parent == nil || parent.Err() != nil || !viper.GetBool("idoV3PolicyPreview.enabled") || projectID == 0 || db.Mysql() == nil ||
		(approved && !approvedV3PolicyPreviewScope(requireFreshScan, baseBlock)) ||
		len(wallets) > 512 || enabled && len(wallets) == 0 || !enabled && len(wallets) != 0 ||
		strings.TrimSpace(reason) == "" || len(reason) > 500 ||
		(baseBlock == 0 && baseHash != "") || (baseBlock != 0 && !v3CanonicalHash(baseHash)) {
		return V3PolicyPreview{}, invalid
	}
	var admitted idov3whitelist.AdmittedProject
	var previewAdmission idov3whitelist.ApprovedLockedCandidate
	var set idov3whitelist.SetManifest
	var ownerHash, governorHash common.Hash
	var err error
	if approved {
		ownerHash, governorHash, err = approvedV3SafeHashes()
		if err != nil {
			return V3PolicyPreview{}, invalid
		}
	} else {
		admitted, set, _, _, _, err = pinnedV3Admission(projectID, false)
		if err != nil {
			return V3PolicyPreview{}, invalid
		}
	}
	// Candidate display authenticates the DB identity and live policy without
	// repeating historical transaction or governance audit. Submitted review
	// recomputation performs one full audit inside its request-owned deadline.
	budget := v3PolicyPreviewRPCBudget(approved, requireFreshScan)
	advance("rpc_setup")
	ctx, cancel := context.WithTimeout(parent, budget)
	diagnosticCtx = ctx
	cancelTrace = cancel
	traceCtx = ctx
	var rpc *ethclient.Client
	if request != nil {
		if !approved || !requireFreshScan || request.ctx != parent {
			return V3PolicyPreview{}, invalid
		}
		rpc = request.rpc
	} else if approved {
		rpc, err = v3TokenGateRPC(ctx)
	} else {
		rpc, err = chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	}
	if err != nil {
		return V3PolicyPreview{}, invalid
	}
	if request == nil {
		defer rpc.Close()
	}
	advance("project_admission")
	if approved {
		if e := v3PolicyScanDiagnostic(ctx, db.Mysql(), projectID); e != nil {
			return V3PolicyPreview{}, e
		}
		if request != nil {
			admitted, previewAdmission, err = request.admit(ctx, db.Mysql(), rpc, projectID, ownerHash, governorHash)
		} else if requireFreshScan {
			admitted, previewAdmission, err = chaintx.AdmitApprovedLockedProjectForPolicyReview(ctx, db.Mysql(), rpc, projectID, ownerHash, governorHash)
		} else {
			// Display only: authenticate the canonical executed-Set/create join.
			// A review independently audits the complete original transaction.
			previewAdmission, err = idov3whitelist.ReadApprovedLockedCandidate(ctx, db.Mysql(), projectID)
			if err == nil {
				admitted = idov3whitelist.AdmittedProject{Binding: previewAdmission.Binding, Runtime: previewAdmission.Runtime, CreationBlock: previewAdmission.CreationBlock, StartBlock: 128805696, Confirmations: 15}
			}
		}
		if err != nil {
			return V3PolicyPreview{}, invalid
		}
	}
	advance("confirmed_snapshot")
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != admitted.Binding.ChainID {
		return V3PolicyPreview{}, invalid
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, admitted.CreationBlock, admitted.Confirmations) {
		return V3PolicyPreview{}, invalid
	}
	height := head - (admitted.Confirmations - 1)
	if baseBlock != 0 {
		// A reviewer needs time to read a full 512-wallet candidate. Keep the
		// original 120-block bound for other scopes; only an approved-mode,
		// first-enable review may pin an older confirmed snapshot. Its current
		// policy/project, original receipt and canonical base hash are still
		// rechecked below before an immutable draft can be written.
		maxAge := v3PolicyReviewBaseMaxAge(approved, requireFreshScan, enabled)
		if baseBlock > height || height-baseBlock > maxAge || !v3CanonicalHash(baseHash) {
			return V3PolicyPreview{}, invalid
		}
		height = baseBlock
	} else if baseHash != "" {
		return V3PolicyPreview{}, invalid
	}
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height {
		return V3PolicyPreview{}, invalid
	}
	if baseBlock != 0 && strings.ToLower(header.Hash().Hex()) != baseHash {
		return V3PolicyPreview{}, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: admitted.Binding.ProxyAddress}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, header.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil ||
		(observed.State != idov3whitelist.ProjectPending && observed.State != idov3whitelist.ProjectActive) {
		return V3PolicyPreview{}, invalid
	}
	policy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, height, header.Hash())
	if err != nil {
		return V3PolicyPreview{}, invalid
	}
	// First enable has no predecessor. Changes/closure require the exact
	// approved prior snapshot and canonical Safe receipt for an honest diff;
	// unrelated financial consumption/certificate publication grants nothing.
	initialCandidate := approved && approvedV3UnscannedInitialCandidate(
		observed.State, policy.Policy, enabled)
	if approved && requireFreshScan && !initialCandidate && baseBlock != 0 && head-(admitted.Confirmations-1)-baseBlock > 120 {
		// The longer human-review window is not for Active projects, policy
		// updates or closure. These retain the original 120-block limit.
		return V3PolicyPreview{}, invalid
	}
	if approved && requireV3PolicyEvidenceStore(ctx, db.Mysql(), projectID) != nil {
		return V3PolicyPreview{}, v3diagnostic.New("UNAVAILABLE", "policy_evidence_store", invalid)
	}
	advance("governance")
	governance := idov3whitelist.GovernanceIdentity{
		Owner: common.HexToAddress(set.OwnerSafe), Governor: common.HexToAddress(set.Governor),
		OwnerCodeHash: common.HexToHash(set.OwnerCodeHash), GovernorCodeHash: common.HexToHash(set.GovernorCodeHash),
	}
	if approved {
		governance = idov3whitelist.GovernanceIdentity{FollowOwner: true,
			Governor: common.HexToAddress(v3DiagnosticGovernor), OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash}
	}
	if !approved || requireFreshScan {
		if _, err := idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime,
			governance, height, header.Hash()); err != nil {
			return V3PolicyPreview{}, invalid
		}
	}
	advance("cursor_canonical")
	if approved {
		if err := chaintx.RecheckApprovedLockedPreview(ctx, db.Mysql(), rpc, projectID, previewAdmission); err != nil {
			return V3PolicyPreview{}, invalid
		}
	} else {
		if err := checkV3ExecutionCursor(ctx, rpc, admitted); err != nil {
			return V3PolicyPreview{}, invalid
		}
	}
	var tree *idov3whitelist.Tree
	if enabled {
		tree, err = idov3whitelist.Build(scope, wallets)
		if err != nil {
			return V3PolicyPreview{}, invalid
		}
	}
	change, err := idov3whitelist.PreviewPolicyChange(scope, observed.State, policy.Policy, tree)
	if err != nil {
		return V3PolicyPreview{}, invalid
	}
	advance("policy_simulation")
	// The only proposed caller is the independently pinned policy Safe, not
	// the operator or the Admin. Simulate its exact V3 method at this block;
	// this is still only a read, never a Safe proposal or execution.
	calldata, err := idov3whitelist.PackPolicyChangeCall(scope, change)
	if err != nil {
		return V3PolicyPreview{}, invalid
	}
	if !approved || requireFreshScan {
		result, err := rpc.CallContract(ctx, ethereum.CallMsg{From: governance.Governor,
			To: &admitted.Binding.ProxyAddress, Data: calldata}, new(big.Int).SetUint64(height))
		if err != nil || len(result) != 0 {
			return V3PolicyPreview{}, invalid
		}
	}
	// The policy and project must still be at the same canonical height. A
	// later tick may reorg, so this response cannot authorize a write.
	again, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil || again == nil || again.Hash() != header.Hash() {
		return V3PolicyPreview{}, invalid
	}
	// A newer, unconfirmed policy must not be ignored just because the
	// confirmed snapshot is still old. Approval remains independently gated.
	advance("latest_chain_state")
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || latest < height {
		return V3PolicyPreview{}, invalid
	}
	latestHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
	if err != nil || latestHeader == nil || latestHeader.Number == nil || !latestHeader.Number.IsUint64() || latestHeader.Number.Uint64() != latest {
		return V3PolicyPreview{}, invalid
	}
	latestPolicy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
	if err != nil || latestPolicy.Policy != policy.Policy {
		return V3PolicyPreview{}, invalid
	}
	latestProject, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, latestProject) != nil || latestProject.State != observed.State {
		return V3PolicyPreview{}, invalid
	}
	latestAgain, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
	if err != nil || latestAgain == nil || latestAgain.Hash() != latestHeader.Hash() {
		return V3PolicyPreview{}, invalid
	}
	if live, e := rpc.BlockNumber(ctx); e != nil || !v3PolicyPreviewHeadAdmitted(latest, live, initialCandidate) {
		return V3PolicyPreview{}, invalid
	}
	advance("cursor_transaction")
	if err := db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID,
			strings.ToLower(admitted.Binding.ProxyAddress.Hex())); err != nil {
			return err
		}
		return chaintx.LockV3RunningCursorTx(tx, admitted.Binding)
	}); err != nil {
		return V3PolicyPreview{}, invalid
	}
	canonical := []string{}
	if tree != nil {
		for _, member := range tree.Members() {
			canonical = append(canonical, strings.ToLower(member.Wallet.Hex()))
		}
	}
	state := "pending"
	if observed.State == idov3whitelist.ProjectActive {
		state = "active"
	}
	preview := V3PolicyPreview{ChainID: strconv.FormatUint(admitted.Binding.ChainID, 10), ProjectID: strconv.FormatUint(projectID, 10),
		Proxy: strings.ToLower(admitted.Binding.ProxyAddress.Hex()), Block: strconv.FormatUint(height, 10),
		BlockHash: strings.ToLower(header.Hash().Hex()), State: state, OldEnabled: policy.Policy.Enabled,
		OldRoot: strings.ToLower(policy.Policy.Root.Hex()), OldVersion: strconv.FormatUint(policy.Policy.Version, 10),
		TargetVersion: strconv.FormatUint(change.TargetVersion, 10), Enabled: enabled, Root: strings.ToLower(change.Root.Hex()),
		CanonicalMembers: canonical, AddedMembers: []string{}, RemovedMembers: []string{},
		MemberCount: len(canonical), Reason: strings.TrimSpace(reason)}
	if change.Candidate != nil {
		preview.MembersDigest = strings.ToLower(change.Candidate.MembersDigest.Hex())
		preview.SnapshotDigest = strings.ToLower(change.Candidate.Digest.Hex())
	}
	if policy.Policy.Enabled {
		if !viper.GetBool("idoV3PolicyJournal.enabled") {
			return V3PolicyPreview{}, invalid
		}
		if v3VerifyPriorReceipt(ctx, rpc, admitted, preview, projectID) != nil {
			return V3PolicyPreview{}, invalid
		}
		if approved && verifyApprovedV3PriorPolicy(ctx, db.Mysql(), rpc, admitted, preview, projectID) != nil {
			return V3PolicyPreview{}, invalid
		}
		err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, preview.Proxy) != nil ||
				chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
				return invalid
			}
			prior, e := v3PolicyPriorMembersTx(tx, preview, projectID)
			if e != nil {
				return invalid
			}
			preview.AddedMembers, preview.RemovedMembers = v3PolicyMemberDiff(prior, canonical)
			return nil
		})
		if err != nil {
			return V3PolicyPreview{}, invalid
		}
	} else {
		preview.AddedMembers = append(preview.AddedMembers, canonical...)
	}
	preview.PreviewDigest, err = V3PolicyPreviewDigest(preview)
	if err != nil {
		return V3PolicyPreview{}, invalid
	}
	if now, e := rpc.BlockNumber(ctx); e != nil || !v3PolicyPreviewHeadAdmitted(latest, now, initialCandidate) {
		return V3PolicyPreview{}, invalid
	}
	advance("final_project_admission")
	if approved {
		if chaintx.RecheckApprovedLockedPreview(ctx, db.Mysql(), rpc, projectID, previewAdmission) != nil ||
			requireV3PolicyEvidenceStore(ctx, db.Mysql(), projectID) != nil {
			return V3PolicyPreview{}, invalid
		}
	}
	advance("complete")
	complete = true
	return preview, nil
}

func v3PolicyPreviewRPCBudget(approved, requireFreshScan bool) time.Duration {
	if approved && !requireFreshScan {
		return 120 * time.Second
	}
	if approved {
		// One full original audit, bounded inside the overall review deadline.
		return 75 * time.Second
	}
	return 12 * time.Second
}

func v3PolicyReviewBaseMaxAge(approved, requireFreshScan, enabled bool) uint64 {
	if approved && requireFreshScan && enabled {
		return 1800 // ~one hour on BSC; not an unbounded historical review
	}
	return 120
}

// The policy evidence store excludes halted/reorg scopes, independently of
// financial freshness. Neither candidate nor review authorizes execution/proof.
func requireV3PolicyPreviewScan(ctx context.Context, database *gorm.DB, requireFreshScan bool, projectIDs ...uint64) error {
	if !requireFreshScan {
		return nil
	}
	id := uint64(0)
	if len(projectIDs) == 1 {
		id = projectIDs[0]
	}
	return requireV3PolicyEvidenceStore(ctx, database, id)
}

func approvedV3UnscannedInitialCandidate(state idov3whitelist.ProjectState, policy idov3whitelist.Policy, enabled bool) bool {
	return enabled && state == idov3whitelist.ProjectPending && !policy.Enabled && policy.Version == 0 && policy.Root == (common.Hash{})
}

// Every candidate/review is bound to its verified latestHeader, not a frozen
// moving head. Policy and canonical hashes remain independently checked.
func v3PolicyPreviewHeadAdmitted(verified, observed uint64, _ bool) bool {
	return observed >= verified
}
