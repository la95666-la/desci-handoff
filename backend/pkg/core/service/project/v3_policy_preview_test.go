package project

import (
	"testing"

	"github.com/spf13/viper"
)

func TestV3InitialPolicyPreviewClosedWithoutIndependentGate(t *testing.T) {
	previous := viper.Get("idoV3PolicyPreview.enabled")
	t.Cleanup(func() { viper.Set("idoV3PolicyPreview.enabled", previous) })
	viper.Set("idoV3PolicyPreview.enabled", false)
	if _, err := PreviewV3InitialPolicy(93, []string{"0x1111111111111111111111111111111111111111"}, "review"); err == nil {
		t.Fatal("V3 preview reached database or RPC without its separate gate")
	}
	viper.Set("idoV3PolicyPreview.enabled", true)
	for _, tc := range []struct {
		wallets []string
		reason  string
	}{
		{nil, "review"},
		{[]string{"0x1111111111111111111111111111111111111111"}, ""},
	} {
		if _, err := PreviewV3InitialPolicy(93, tc.wallets, tc.reason); err == nil {
			t.Fatal("incomplete preview reached database or RPC")
		}
	}
	if _, err := previewV3PolicyChangeAt(93, nil, false, "close", 0, "0x"+"1"); err == nil {
		t.Fatal("base hash without a confirmed base block accepted")
	}
}
