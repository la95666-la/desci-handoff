package project

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
)

var errV3PolicyDisplay = errors.New("V3 policy display status unavailable")

const v3PolicyDisplayBudget = 30 * time.Second

// ReadApprovedV3PolicyDisplayStatusContext is ONLY an Admin read observation.
// Do not substitute it for review/approval/handoff/execution/proof admission.
// Another project's failed tick is not authority over this project's display.
func ReadApprovedV3PolicyDisplayStatusContext(parent context.Context, id uint64) (V3PolicyStatus, error) {
	if parent == nil || parent.Err() != nil || id == 0 || db.Mysql() == nil || !ApprovedV3ModeConfigured() || !viper.GetBool("idoV3PolicyPreview.enabled") {
		return V3PolicyStatus{}, errV3PolicyDisplay
	}
	owner, governor, e := approvedV3SafeHashes()
	if e != nil {
		return V3PolicyStatus{}, errV3PolicyDisplay
	}
	ctx, cancel := context.WithTimeout(parent, v3PolicyDisplayBudget)
	defer cancel()
	rpc, e := v3TokenGateRPC(ctx)
	if e != nil {
		return V3PolicyStatus{}, errV3PolicyDisplay
	}
	defer rpc.Close()
	return readApprovedV3Display(ctx, db.Mysql(), rpc, id, owner, governor)
}

func readApprovedV3Display(ctx context.Context, database *gorm.DB, rpc chaintx.V3LifecycleRecoveryRPC, id uint64, owner, governor common.Hash) (V3PolicyStatus, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || rpc == nil || id == 0 {
		return V3PolicyStatus{}, errV3PolicyDisplay
	}
	var rows []struct{ State string }
	r := database.WithContext(ctx).Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=56 AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' LIMIT 2`, strings.ToLower(v3TokenProxy), id).Scan(&rows)
	if r.Error != nil || len(rows) != 1 || (rows[0].State != "running" && rows[0].State != "reorg_halted") {
		return V3PolicyStatus{}, errV3PolicyDisplay
	}
	halted := rows[0].State == "reorg_halted"
	status, e := readV3PolicyDisplayStatus(ctx, database, rpc, id, owner, governor, v3PolicyDisplayChecks{
		admit: func(c context.Context, d *gorm.DB, r chaintx.V3LifecycleRecoveryRPC, id uint64, o, g common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
			s, e := readV3DisplayCandidate(c, d, id, halted)
			return idov3whitelist.AdmittedProject{Binding: s.Binding, Runtime: s.Runtime, CreationBlock: s.CreationBlock, Confirmations: 15}, s, e
		},
		recheck: func(c context.Context, d *gorm.DB, r chaintx.V3LifecycleRecoveryRPC, id uint64, s idov3whitelist.ApprovedLockedCandidate) error {
			after, e := readV3DisplayCandidate(c, d, id, halted)
			if e != nil {
				return e
			}
			return chaintx.RecheckPolicyObservationCandidate(s, after)
		},
	})
	if e == nil {
		status.ScannerState = rows[0].State
		if halted {
			status.ApprovalState = "historical_display_unavailable_for_proof"
		}
	}
	return status, e
}

func readV3DisplayCandidate(ctx context.Context, database *gorm.DB, id uint64, halted bool) (idov3whitelist.ApprovedLockedCandidate, error) {
	if halted {
		return idov3whitelist.ReadApprovedHaltedCandidate(ctx, database, id)
	}
	return idov3whitelist.ReadApprovedLockedCandidate(ctx, database, id)
}

// Per-invocation dependencies allow tests to exercise the chain observation
// without creating any global bypass or alternate production authority path.
type v3PolicyDisplayChecks struct {
	admit   func(context.Context, *gorm.DB, chaintx.V3LifecycleRecoveryRPC, uint64, common.Hash, common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error)
	recheck func(context.Context, *gorm.DB, chaintx.V3LifecycleRecoveryRPC, uint64, idov3whitelist.ApprovedLockedCandidate) error
}

func readV3PolicyDisplayStatus(ctx context.Context, database *gorm.DB, rpc chaintx.V3LifecycleRecoveryRPC, id uint64, owner, governor common.Hash, checks v3PolicyDisplayChecks) (result V3PolicyStatus, resultErr error) {
	phase := "input"
	started := time.Now()
	defer func() {
		if resultErr != nil {
			if logger := log.GetLogger(); logger != nil {
				logger.WithFields(map[string]any{"phase": phase, "project_id": strconv.FormatUint(id, 10), "elapsed_ms": time.Since(started).Milliseconds(), "deadline": errors.Is(ctxErr(ctx), context.DeadlineExceeded)}).Warn("V3 policy display observation unavailable")
			}
			resultErr = v3PolicyContextDiagnostic(ctx, resultErr, phase)
		}
	}()
	zero := V3PolicyStatus{}
	if ctx == nil || ctx.Err() != nil || database == nil || rpc == nil || id == 0 || owner == (common.Hash{}) || governor == (common.Hash{}) || checks.admit == nil || checks.recheck == nil {
		return zero, errV3PolicyDisplay
	}
	// DB-bound identity for display only; no full original receipt audit.
	phase = "admission"
	a, snapshot, e := checks.admit(ctx, database, rpc, id, owner, governor)
	if e != nil || a.Binding.ProjectID != id || a.Binding.ChainID != 56 || a.Binding.ProxyAddress != common.HexToAddress(v3TokenProxy) || a.Confirmations != 15 || a.CreationBlock == 0 {
		return zero, errV3PolicyDisplay
	}
	phase = "confirmed_head"
	head, e := rpc.BlockNumber(ctx)
	if e != nil || !idov3whitelist.HasConfirmations(head, a.CreationBlock, 15) {
		return zero, errV3PolicyDisplay
	}
	height := head - 14
	block := new(big.Int).SetUint64(height)
	anchor, e := rpc.HeaderByNumber(ctx, block)
	if e != nil || anchor == nil || anchor.Number == nil || anchor.Number.Cmp(block) != 0 {
		return zero, errV3PolicyDisplay
	}
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: new(big.Int).SetUint64(id), ProxyAddress: a.Binding.ProxyAddress}
	phase = "project"
	observed, e := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, a.Runtime, height, anchor.Hash())
	if e != nil || idov3whitelist.MatchProjectIdentityObservation(a.Binding, observed) != nil {
		return zero, errV3PolicyDisplay
	}
	phase = "policy"
	policy, e := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, a.Runtime, height, anchor.Hash())
	if e != nil {
		return zero, errV3PolicyDisplay
	}
	// DB-only immutable identity and contiguous cursor fence, not chain authority.
	phase = "final_admission"
	if e := checks.recheck(ctx, database, rpc, id, snapshot); e != nil {
		return zero, errV3PolicyDisplay
	}
	phase = "final_anchor"
	finalHead, e := rpc.BlockNumber(ctx)
	if e != nil || finalHead < head || !idov3whitelist.HasConfirmations(finalHead, height, 15) || ctx.Err() != nil {
		return zero, errV3PolicyDisplay
	}
	stable, e := rpc.HeaderByNumber(ctx, block)
	if e != nil || stable == nil || stable.Number == nil || stable.Number.Cmp(block) != 0 || stable.Hash() != anchor.Hash() {
		return zero, errV3PolicyDisplay
	}
	states := []string{"pending", "active", "successful", "failed", "cancelled"}
	if int(observed.State) >= len(states) {
		return zero, errV3PolicyDisplay
	}
	return V3PolicyStatus{ChainID: "56", ProjectID: strconv.FormatUint(id, 10), Proxy: strings.ToLower(a.Binding.ProxyAddress.Hex()), Block: strconv.FormatUint(height, 10), BlockHash: strings.ToLower(anchor.Hash().Hex()), ProjectState: states[observed.State], Enabled: policy.Policy.Enabled, Root: strings.ToLower(policy.Policy.Root.Hex()), Version: strconv.FormatUint(policy.Policy.Version, 10), ApprovalState: "unavailable_for_proof"}, nil
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
