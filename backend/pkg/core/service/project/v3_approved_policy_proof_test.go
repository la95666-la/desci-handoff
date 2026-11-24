package project

import (
	"testing"

	"github.com/spf13/viper"
)

func TestApprovedV3ProofFailsClosedWithoutPolicyAndSessionAdmission(t *testing.T) {
	for _, key := range []string{"idoV3PolicyProof.enabled", "idoV3PolicyJournal.enabled"} {
		old := viper.Get(key)
		t.Cleanup(func() { viper.Set(key, old) })
		viper.Set(key, false)
	}
	if _, err := ProveApprovedV3Wallet(1, 19, "0x1111111111111111111111111111111111111111", 1); err == nil {
		t.Fatal("disabled policy proof served")
	}
	if _, err := ProveApprovedV3Wallet(0, 19, "0x1111111111111111111111111111111111111111", 1); err == nil {
		t.Fatal("unauthenticated wallet served")
	}
}
