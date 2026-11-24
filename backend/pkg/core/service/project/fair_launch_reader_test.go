package project

import "testing"

func TestFairLaunchPublicWalletActionMapIsExactlyFailClosed(t *testing.T) {
	actions := fairLaunchPublicWalletActions()
	for _, action := range []string{"contribute", "claim", "refund", "depositProjectTokens", "withdrawFailedProjectTokens", "withdrawProjectProceeds", "withdrawPlatformFee"} {
		gate, ok := actions[action]
		if !ok || gate["available"] != false || gate["reason"] != "durable_wallet_submission_unavailable" {
			t.Fatalf("%s gate is not exactly fail closed: %#v", action, gate)
		}
	}
	if len(actions) != 8 || actions["release"]["available"] != false || actions["release"]["reason"] != "operator_only" {
		t.Fatalf("unexpected public action surface: %#v", actions)
	}
}
