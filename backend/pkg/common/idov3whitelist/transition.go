package idov3whitelist

import (
	"errors"
	"math"

	"github.com/ethereum/go-ethereum/common"
)

// ProjectState mirrors JoinCareIDOv3.ProjectStatus, not Backend marketing
// status. A caller must obtain it from the correct V3 proxy at a pinned block.
// Never infer it from a project listing or a pending receipt.
type ProjectState uint8

const (
	ProjectPending ProjectState = iota
	ProjectActive
	ProjectSuccessful
	ProjectFailed
	ProjectCancelled
)

// PolicyChange is an offline preview for setWhitelistPolicy. The actual
// governance call must be independently authorized and revalidated at send
// time; this value has no signing or transaction capability.
type PolicyChange struct {
	ExpectedVersion uint64
	TargetVersion   uint64
	Enabled         bool
	Root            common.Hash
	Candidate       *SnapshotCommitment
}

// PreviewPolicyChange mirrors the on-chain mode-freeze and version rules.
// A nil tree requests a disabled policy (Pending only); a non-nil tree
// replaces the entire enabled membership set. Preview neither authenticates
// the old policy nor confirms an Admin approval or chain execution.
func PreviewPolicyChange(scope Scope, state ProjectState, current Policy, tree *Tree) (PolicyChange, error) {
	if err := validateScope(scope); err != nil {
		return PolicyChange{}, err
	}
	if current.Version == math.MaxUint64 {
		return PolicyChange{}, errors.New("V3 policy version exhausted")
	}
	if current.Enabled == (current.Root == (common.Hash{})) {
		return PolicyChange{}, errors.New("V3 current policy is inconsistent")
	}
	if state != ProjectPending && state != ProjectActive {
		return PolicyChange{}, errors.New("V3 policy is terminal or state is unknown")
	}
	enabled := tree != nil
	if state == ProjectActive && (!current.Enabled || !enabled) {
		return PolicyChange{}, errors.New("V3 whitelist mode frozen after activation")
	}
	change := PolicyChange{ExpectedVersion: current.Version, TargetVersion: current.Version + 1, Enabled: enabled}
	if enabled {
		commitment, err := tree.CommitSnapshot(scope, change.TargetVersion)
		if err != nil {
			return PolicyChange{}, err
		}
		change.Root = commitment.Root
		change.Candidate = &commitment
	}
	if current.Enabled == change.Enabled && current.Root == change.Root {
		return PolicyChange{}, errors.New("V3 policy change is a no-op")
	}
	return change, nil
}
