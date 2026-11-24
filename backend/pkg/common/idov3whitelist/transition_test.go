package idov3whitelist

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestPreviewPolicyChangeMirrorsContractModeFreeze(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(77), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	a, err := Build(scope, []string{"0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(scope, []string{"0x2222222222222222222222222222222222222222"})
	if err != nil {
		t.Fatal(err)
	}
	disabled := Policy{}
	initial, err := PreviewPolicyChange(scope, ProjectPending, disabled, a)
	if err != nil || initial.ExpectedVersion != 0 || initial.TargetVersion != 1 || !initial.Enabled || initial.Root != a.Root || initial.Candidate == nil || initial.Candidate.Version != 1 {
		t.Fatalf("pending enable preview invalid: %+v err=%v", initial, err)
	}
	current := Policy{Enabled: true, Root: a.Root, Version: 1}
	disable, err := PreviewPolicyChange(scope, ProjectPending, current, nil)
	if err != nil || disable.Enabled || disable.Root != (common.Hash{}) || disable.Candidate != nil || disable.TargetVersion != 2 {
		t.Fatalf("pending disable preview invalid: %+v err=%v", disable, err)
	}
	change, err := PreviewPolicyChange(scope, ProjectActive, current, b)
	if err != nil || change.Root != b.Root || change.TargetVersion != 2 || change.Candidate == nil || change.Candidate.Digest == (common.Hash{}) {
		t.Fatalf("active member replacement preview invalid: %+v err=%v", change, err)
	}
	for name, input := range map[string]struct {
		state  ProjectState
		policy Policy
		tree   *Tree
	}{
		"active first enable":    {ProjectActive, disabled, a},
		"active disable":         {ProjectActive, current, nil},
		"pending no-op":          {ProjectPending, current, a},
		"pending disabled no-op": {ProjectPending, disabled, nil},
		"successful":             {ProjectSuccessful, current, b},
		"failed":                 {ProjectFailed, current, b},
		"cancelled":              {ProjectCancelled, current, b},
		"unknown":                {ProjectState(255), current, b},
		"malformed policy":       {ProjectPending, Policy{Enabled: true}, a},
		"version overflow":       {ProjectPending, Policy{Enabled: true, Root: a.Root, Version: math.MaxUint64}, b},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := PreviewPolicyChange(scope, input.state, input.policy, input.tree); err == nil {
				t.Fatal("unsafe transition accepted")
			}
		})
	}
	other := scope
	other.ProjectID = big.NewInt(78)
	if _, err := PreviewPolicyChange(other, ProjectPending, disabled, a); err == nil {
		t.Fatal("cross-project member snapshot admitted")
	}
}
