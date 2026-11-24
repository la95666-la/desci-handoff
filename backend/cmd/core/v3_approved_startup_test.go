package main

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Approved mode must not ask for the retired project-scoped Set/scan release.
// This negative startup test intentionally supplies no valid relayer key or DB: it
// cannot enable signing, but identifies which gate rejected the configuration.
func TestApprovedV3StartupUsesApprovedPrerequisitesNotOfflineSignatures(t *testing.T) {
	keys := []string{"idoV3Execution.enabled", "idoV3Approved.enabled", "idoV3Execution.proxyAddress",
		"idoV3Execution.operatorAddress", "chain.project.v2.contractAddress", "chain.project.jobPrivate",
		"chain.project.chainID", "idoV3Set.pinnedPublicKey", "idoV3Scanner.pinnedPublicKey",
		"idoV3Scanner.enabled", "idoV3TokenAdmission.enabled", "idoV3Lifecycle.enabled",
		"idoV3PolicyPreview.enabled", "idoV3PolicyReview.enabled", "idoV3PolicyExecution.enabled",
		"idoV3PolicyProof.enabled", "idoV3PolicyJournal.enabled", "idoV3TokenAdmission.primaryRPC",
		"idoV3TokenAdmission.ownerSafeCodeHash", "idoV3TokenAdmission.governorSafeCodeHash"}
	for _, key := range keys {
		key, old := key, viper.Get(key)
		t.Cleanup(func() { viper.Set(key, old) })
	}
	viper.Set("idoV3Execution.enabled", true)
	viper.Set("idoV3Approved.enabled", true)
	viper.Set("idoV3Execution.proxyAddress", "0x0D4040875742975269c55E16C8884d3Ee9510B8E")
	viper.Set("idoV3Execution.operatorAddress", "0xfd712061ee08E070A930eCD45E935Ae5d7f5445C")
	viper.Set("chain.project.v2.contractAddress", "0x1111111111111111111111111111111111111111")
	viper.Set("chain.project.jobPrivate", "placeholder-not-a-key")
	viper.Set("chain.project.chainID", 56)
	viper.Set("idoV3Set.pinnedPublicKey", "")
	viper.Set("idoV3Scanner.pinnedPublicKey", "")
	viper.Set("idoV3Scanner.enabled", false)
	for _, key := range []string{"idoV3TokenAdmission.enabled", "idoV3Lifecycle.enabled",
		"idoV3PolicyPreview.enabled", "idoV3PolicyReview.enabled", "idoV3PolicyExecution.enabled",
		"idoV3PolicyProof.enabled", "idoV3PolicyJournal.enabled"} {
		viper.Set(key, true)
	}
	viper.Set("idoV3TokenAdmission.primaryRPC", "https://silent-bold-vineyard.bsc.quiknode.pro/placeholder/")
	viper.Set("idoV3TokenAdmission.ownerSafeCodeHash", "0x"+strings.Repeat("a", 64))
	viper.Set("idoV3TokenAdmission.governorSafeCodeHash", "0x"+strings.Repeat("b", 64))
	err := initProjectV3ChainClient()
	if err == nil || !strings.Contains(err.Error(), "execution client unavailable") {
		t.Fatalf("approved startup did not pass the obsolete signature gate and reject missing custody: %v", err)
	}
}
