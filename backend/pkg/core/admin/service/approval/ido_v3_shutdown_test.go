package approval

import "testing"

func TestIDOV3ApprovalSubmitCannotReachRegistryOrDatabase(t *testing.T) {
	for _, op := range []string{"ido_v3.set_policy", "ido_v3_policy", "v3.set_policy", " V3_POLICY "} {
		if !isUnmaterializedIDOV3Approval(op) {
			t.Fatalf("V3 operation not reserved: %q", op)
		}
		if _, err := SubmitTx(nil, SubmitReq{Operation: op, InitiatorID: 1}); err == nil || err.Error() != "IDO V3 approval workflow unavailable" {
			t.Fatalf("V3 operation reached approval registry or DB: %q: %v", op, err)
		}
	}
	for _, op := range []string{"v2.set", "fair_launch.set", "v3else.set", "ido_v3else.set"} {
		if isUnmaterializedIDOV3Approval(op) {
			t.Fatalf("unrelated operation incorrectly reserved: %q", op)
		}
	}
}
