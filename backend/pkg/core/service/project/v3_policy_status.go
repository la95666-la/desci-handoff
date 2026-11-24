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

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/db"
)

// V3PolicyStatus describes a confirmed read-only observation. It intentionally
// cannot claim an approved member set, effective policy, or proof eligibility.
type V3PolicyStatus struct {
	ChainID       string `json:"chain_id"`
	ProjectID     string `json:"project_id"`
	Proxy         string `json:"proxy"`
	Block         string `json:"block"`
	BlockHash     string `json:"block_hash"`
	ProjectState  string `json:"project_state"`
	Enabled       bool   `json:"enabled"`
	Root          string `json:"root"`
	Version       string `json:"version"`
	ApprovalState string `json:"approval_state"`
	ScannerState  string `json:"scanner_state,omitempty"`
}

// ReadV3PolicyStatus never reads candidate rows or Warmup registrations.
// It is gated separately from Core execution and cannot promote a root to an
// approved, reorg-monitored member snapshot merely because the chain has one.
func ReadV3PolicyStatus(projectID uint64) (V3PolicyStatus, error) {
	invalid := errors.New("V3 policy status unavailable")
	if !viper.GetBool("idoV3PolicyPreview.enabled") || projectID == 0 || db.Mysql() == nil {
		return V3PolicyStatus{}, invalid
	}
	admitted, set, _, _, _, err := pinnedV3Admission(projectID, false)
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	rpc, err := chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	defer rpc.Close()
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != admitted.Binding.ChainID {
		return V3PolicyStatus{}, invalid
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, admitted.CreationBlock, admitted.Confirmations) {
		return V3PolicyStatus{}, invalid
	}
	height := head - (admitted.Confirmations - 1)
	block := new(big.Int).SetUint64(height)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 {
		return V3PolicyStatus{}, invalid
	}
	scope := idov3whitelist.Scope{ChainID: chainID, ProjectID: new(big.Int).SetUint64(projectID), ProxyAddress: admitted.Binding.ProxyAddress}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, header.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil {
		return V3PolicyStatus{}, invalid
	}
	policy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, height, header.Hash())
	if err != nil {
		return V3PolicyStatus{}, invalid
	}
	governance := idov3whitelist.GovernanceIdentity{Owner: common.HexToAddress(set.OwnerSafe),
		Governor: common.HexToAddress(set.Governor), OwnerCodeHash: common.HexToHash(set.OwnerCodeHash),
		GovernorCodeHash: common.HexToHash(set.GovernorCodeHash)}
	if _, err := idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime, governance, height, header.Hash()); err != nil {
		return V3PolicyStatus{}, invalid
	}
	if err := checkV3ExecutionCursor(ctx, rpc, admitted); err != nil {
		return V3PolicyStatus{}, invalid
	}
	again, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || again == nil || again.Hash() != header.Hash() {
		return V3PolicyStatus{}, invalid
	}
	states := []string{"pending", "active", "successful", "failed", "cancelled"}
	if int(observed.State) >= len(states) {
		return V3PolicyStatus{}, invalid
	}
	return V3PolicyStatus{ChainID: strconv.FormatUint(admitted.Binding.ChainID, 10), ProjectID: strconv.FormatUint(projectID, 10),
		Proxy: strings.ToLower(admitted.Binding.ProxyAddress.Hex()), Block: strconv.FormatUint(height, 10),
		BlockHash: strings.ToLower(header.Hash().Hex()), ProjectState: states[observed.State],
		Enabled: policy.Policy.Enabled, Root: strings.ToLower(policy.Policy.Root.Hex()),
		Version: strconv.FormatUint(policy.Policy.Version, 10), ApprovalState: "unavailable_for_proof"}, nil
}
