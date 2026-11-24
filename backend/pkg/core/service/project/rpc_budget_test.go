package project

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestApprovedOperatorRPCBudgetsAndSignedRecheck(t *testing.T) {
	if v3WalletProofReadBudget(true) != 10*time.Second || v3WalletProofReadBudget(false) != 15*time.Second {
		t.Fatal("wallet proof budget does not fit its proxy/legacy boundary")
	}
	for _, action := range []string{"ido_v3_activate", "ido_v3_end", "ido_v3_withdraw_platform_fee"} {
		if b := v3ApprovedLifecycleBudget(action); b != 150*time.Second || b >= 160*time.Second {
			t.Fatalf("%s budget=%v", action, b)
		}
	}
	create, err := os.ReadFile("v3_approved_create_execute.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(create), "context.WithTimeout(context.Background(), 150*time.Second)") {
		t.Fatal("create deadline diverged from reviewed client/proxy budgets")
	}
	lifecycle, err := os.ReadFile("v3_approved_lifecycle_execute.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(lifecycle)
	for _, required := range []string{"AdmitApprovedLockedProjectForLifecycle", "RecheckApprovedLifecycleCreation", "previewApprovedV3Lifecycle", "BroadcastV3PersistedOnce", "checkOriginalWithIndependentRPC", "cancelFailedV3UnsignedIntent"} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing operator fence %s", required)
		}
	}
	if strings.Contains(s, "chaintx.AdmitApprovedLockedProject(ctx") {
		t.Fatal("duplicate full original-create audit reintroduced")
	}
	// Initial project admission and the signed original are independently
	// fenced. Neither can reach reserve/broadcast through the old global gate.
	if strings.Count(s, "requireApprovedV3OperatorProjectScan(ctx, db.Mysql(), rpc, admitted, action)") != 2 {
		t.Fatal("missing pre-sign or pre-broadcast project scanner fence")
	}
	first := strings.Index(s, "requireApprovedV3OperatorProjectScan(ctx, db.Mysql(), rpc, admitted, action)")
	last := strings.LastIndex(s, "requireApprovedV3OperatorProjectScan(ctx, db.Mysql(), rpc, admitted, action)")
	if first > strings.Index(s, "chaintx.ReserveV3OperatorIntentTx(") ||
		last < strings.Index(s, "chaintx.PrepareV3Once(") ||
		last > strings.Index(s, "phase = \"signed_preflight\"") {
		t.Fatal("project scanner fence moved outside signing/broadcast boundary")
	}
}
