package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type v3ActivationRevertError struct{ data any }

func (e v3ActivationRevertError) Error() string          { return "execution reverted" }
func (e v3ActivationRevertError) ErrorData() interface{} { return e.data }

func TestV3ActivationReserveRevertRequiresExactRPCData(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		err          error
		want         bool
	}{
		{"exact", "activateProject", v3ActivationRevertError{"0xe9548d03"}, true},
		{"wrapped", "activateProject", fmt.Errorf("simulate: %w", v3ActivationRevertError{"0xE9548D03"}), true},
		{"other action", "endProject", v3ActivationRevertError{"0xe9548d03"}, false},
		{"wrong selector", "activateProject", v3ActivationRevertError{"0x95b1f6dc"}, false},
		{"extra data", "activateProject", v3ActivationRevertError{"0xe9548d0300"}, false},
		{"message only", "activateProject", errors.New("execution reverted: InsufficientProjectTokens()"), false},
		{"timeout", "activateProject", context.DeadlineExceeded, false},
		{"non-string", "activateProject", v3ActivationRevertError{[]byte{0xe9, 0x54, 0x8d, 0x03}}, false},
		{"nil", "activateProject", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := v3ActivationReserveRevert(tc.method, tc.err); got != tc.want {
				t.Fatalf("reserve revert classification = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApprovedV3ActivationDoesNotRequireGlobalScanFreshness(t *testing.T) {
	for _, name := range []string{"v3_lifecycle_project_scan.go", "v3_approved_lifecycle_execute.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "RequireFreshApprovedV3") || strings.Contains(string(raw), "RequireApprovedV3ProjectScan") {
			t.Fatal("operator still coupled to certificate", name)
		}
	}
}

func TestApprovedV3ActivationBudgetIsBelowClientProxyAndCore(t *testing.T) {
	if budget := v3ApprovedLifecycleBudget("ido_v3_activate"); budget != 150*time.Second || budget >= 160*time.Second {
		t.Fatalf("activation budget=%v", budget)
	}
	for _, action := range []string{"ido_v3_end", "ido_v3_withdraw_platform_fee"} {
		if budget := v3ApprovedLifecycleBudget(action); budget != 150*time.Second {
			t.Fatalf("%s budget changed: %v", action, budget)
		}
	}
	if v3ApprovedLifecycleBudget("unknown") != 45*time.Second {
		t.Fatal("unknown method budget broadened")
	}
}
