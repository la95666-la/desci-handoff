package project

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestApprovedV3ModeRefusesPartialConfiguration(t *testing.T) {
	keys := []string{"idoV3Approved.enabled", "idoV3TokenAdmission.enabled", "idoV3Execution.enabled",
		"idoV3Lifecycle.enabled", "idoV3PolicyPreview.enabled", "idoV3PolicyReview.enabled",
		"idoV3PolicyExecution.enabled", "idoV3PolicyProof.enabled", "idoV3PolicyJournal.enabled",
		"idoV3Scanner.enabled", "idoV3TokenAdmission.primaryRPC",
		"idoV3TokenAdmission.ownerSafeCodeHash", "idoV3TokenAdmission.governorSafeCodeHash",
		"chain.project.chainID", "idoV3Execution.proxyAddress", "idoV3Execution.operatorAddress",
		"chain.project.v2.contractAddress"}
	for _, key := range keys {
		name, old := key, viper.Get(key)
		t.Cleanup(func() { viper.Set(name, old) })
	}
	for _, key := range keys[:9] {
		viper.Set(key, true)
	}
	viper.Set("idoV3Scanner.enabled", false)
	viper.Set("idoV3TokenAdmission.primaryRPC", "https://silent-bold-vineyard.bsc.quiknode.pro/placeholder/")
	viper.Set("idoV3TokenAdmission.ownerSafeCodeHash", "0x"+strings.Repeat("a", 64))
	viper.Set("idoV3TokenAdmission.governorSafeCodeHash", "0x"+strings.Repeat("b", 64))
	viper.Set("chain.project.chainID", 56)
	viper.Set("idoV3Execution.proxyAddress", v3TokenProxy)
	viper.Set("idoV3Execution.operatorAddress", v3DiagnosticOperator)
	viper.Set("chain.project.v2.contractAddress", "0x1111111111111111111111111111111111111111")
	if !ApprovedV3ModeEnabled() {
		t.Fatal("complete candidate config rejected")
	}
	if !approvedV3ExecutionIdentityConfigured() {
		t.Fatal("fixed execution identity rejected")
	}
	viper.Set("idoV3Execution.proxyAddress", "0x1111111111111111111111111111111111111111")
	if approvedV3ExecutionIdentityConfigured() {
		t.Fatal("foreign proxy admitted")
	}
	viper.Set("idoV3Execution.proxyAddress", v3TokenProxy)
	viper.Set("chain.project.v2.contractAddress", v3TokenProxy)
	if approvedV3ExecutionIdentityConfigured() {
		t.Fatal("V2 client borrowed V3 identity")
	}
	viper.Set("chain.project.v2.contractAddress", "0x1111111111111111111111111111111111111111")
	for _, key := range keys[1:9] {
		viper.Set(key, false)
		if ApprovedV3ModeEnabled() {
			t.Fatalf("missing %s accepted", key)
		}
		viper.Set(key, true)
	}
	viper.Set("idoV3Scanner.enabled", true)
	if ApprovedV3ModeEnabled() {
		t.Fatal("old and approved scanners enabled together")
	}
	viper.Set("idoV3Scanner.enabled", false)
	viper.Set("idoV3TokenAdmission.governorSafeCodeHash", "")
	if ApprovedV3ModeEnabled() {
		t.Fatal("missing Governor Safe trust root accepted")
	}
	viper.Set("idoV3Approved.enabled", false)
	if ApprovedV3ModeConfigured() || ApprovedV3ModeReady() {
		t.Fatal("default-off mode opened")
	}
}
