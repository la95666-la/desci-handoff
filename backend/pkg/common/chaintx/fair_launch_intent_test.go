package chaintx

import (
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/projectruntime"
)

func fairRuntime(enabled bool) projectruntime.Runtime {
	return projectruntime.Runtime{
		Enabled: enabled, Version: constant.ContractVersionFairLaunchV1, FundingMode: constant.FundingModeFairLaunch,
		ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001",
		OperatorAddress: "0x2000000000000000000000000000000000000002",
	}
}

func TestFairLaunchIntentUsesTrustedOperatorSenderDomain(t *testing.T) {
	intent, err := FairLaunchIntent(fairRuntime(true), 8, constant.TransactionTypeFairLaunchEnd, []byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if intent.ChainID != 56 || intent.SenderAddress.Hex() != "0x2000000000000000000000000000000000000002" || intent.ContractAddress.Hex() != "0x1000000000000000000000000000000000000001" {
		t.Fatalf("wrong trusted intent identity: %+v", intent)
	}
}

func TestFairLaunchIntentFailsClosedWhenDisabledOrUnknown(t *testing.T) {
	if _, err := FairLaunchIntent(fairRuntime(false), 8, constant.TransactionTypeFairLaunchEnd, []byte{1, 2, 3, 4}); err == nil {
		t.Fatal("disabled runtime produced an intent")
	}
	if _, err := FairLaunchIntent(fairRuntime(true), 8, "future_action", []byte{1, 2, 3, 4}); err == nil {
		t.Fatal("unknown action produced an intent")
	}
}

func TestFairLaunchIntentRejectsBeneficiaryWalletWithdrawals(t *testing.T) {
	for _, action := range []string{constant.TransactionTypeFairLaunchProjectProceedsWithdraw, constant.TransactionTypeFairLaunchPlatformFeeWithdraw} {
		if _, err := FairLaunchIntent(fairRuntime(true), 8, action, []byte{1, 2, 3, 4}); err == nil {
			t.Fatalf("beneficiary wallet action %q routed through Operator", action)
		}
	}
}
