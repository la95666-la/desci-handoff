package project

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"joincare_server/pkg/common/idov3whitelist"
)

type v3ObservationCurrentRPC interface {
	idov3whitelist.PolicyRPC
	BlockNumber(context.Context) (uint64, error)
}

// Pin the latest block observed at entry, then authenticate policy, project
// identity/state and the same canonical hash across the reads. Normal chain
// growth does not invalidate this observation: requiring a frozen head makes
// a multi-second BSC read practically impossible. Subsequent original-receipt,
// admission and scanner gates remain in the recording flow. This observation
// is not a claim that no policy change can happen after the pinned instant.
func checkApprovedV3ObservationCurrentPolicy(ctx context.Context, rpc v3ObservationCurrentRPC, scope idov3whitelist.Scope, admitted idov3whitelist.AdmittedProject, preview V3PolicyPreview) error {
	invalid := errors.New("V3 current policy observation unavailable")
	if ctx == nil || ctx.Err() != nil || rpc == nil {
		return invalid
	}
	latest, err := rpc.BlockNumber(ctx)
	if err != nil {
		return invalid
	}
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != latest {
		return invalid
	}
	current, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, latest, header.Hash())
	if err != nil || current.Policy.Version != mustParseV3Version(preview.TargetVersion) || current.Policy.Enabled != preview.Enabled || strings.ToLower(current.Policy.Root.Hex()) != preview.Root {
		return invalid
	}
	currentProject, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, latest, header.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, currentProject) != nil ||
		(currentProject.State != idov3whitelist.ProjectPending && currentProject.State != idov3whitelist.ProjectActive) ||
		(currentProject.State == idov3whitelist.ProjectActive && !preview.Enabled) {
		return invalid
	}
	if preview.State != "" {
		states := []string{"pending", "active", "successful", "failed", "cancelled"}
		if int(currentProject.State) >= len(states) || states[currentProject.State] != preview.State {
			return invalid
		}
	}
	if again, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest)); e != nil || again == nil || again.Number == nil || !again.Number.IsUint64() || again.Number.Uint64() != latest || again.Hash() != header.Hash() {
		return invalid
	}
	if ctx.Err() != nil {
		return invalid
	}
	return nil
}
