package project

import (
	"testing"

	"github.com/spf13/viper"
)

func TestV3ExecuteRemainsClosedWithoutReleaseAdmissions(t *testing.T) {
	old := viper.Get("idoV3Execution.enabled")
	t.Cleanup(func() { viper.Set("idoV3Execution.enabled", old) })
	viper.Set("idoV3Execution.enabled", false)
	if _, err := executeV3Create(7, nil); err == nil {
		t.Fatal("unreviewed V3 create reached execution")
	}
	for _, pair := range []struct{ method, action string }{
		{"activateProject", "ido_v3_activate"}, {"endProject", "ido_v3_end"},
		{"withdrawPlatformFee", "ido_v3_withdraw_platform_fee"},
		{"cancelProject", "ido_v3_cancel"}, {"withdrawRaisedFunds", "ido_v3_withdraw_raised_funds"},
	} {
		if _, err := executeV3Lifecycle(7, nil, pair.method, pair.action); err == nil {
			t.Fatalf("V3 method %s bypassed operator admission", pair.method)
		}
	}
}
