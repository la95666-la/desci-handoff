package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestVerifyV3ActionCallBindsABIProjectAndCanonicalArguments(t *testing.T) {
	for _, tc := range []struct{ action, method string }{
		{"ido_v3_activate", "activateProject"}, {"ido_v3_end", "endProject"},
		{"ido_v3_withdraw_platform_fee", "withdrawPlatformFee"},
	} {
		data, err := PackV3Lifecycle(tc.method, big.NewInt(42))
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyV3ActionCall(tc.action, 42, data); err != nil {
			t.Fatalf("%s rejected: %v", tc.action, err)
		}
		for _, wrong := range []struct {
			action string
			id     uint64
			data   []byte
		}{
			{tc.action, 43, data}, {"ido_v3_future", 42, data}, {"project_end", 42, data},
			{tc.action, 42, append(append([]byte{}, data...), 0)},
		} {
			if VerifyV3ActionCall(wrong.action, wrong.id, wrong.data) == nil {
				t.Fatalf("accepted invalid %s", tc.action)
			}
		}
	}
	// These methods are valid wallet/Safe calls, but never relayer outbox actions.
	for _, method := range []string{"cancelProject", "withdrawRaisedFunds", "withdrawProjectTokens"} {
		data, err := PackV3Lifecycle(method, big.NewInt(42))
		if err != nil {
			t.Fatal(err)
		}
		action := map[string]string{"cancelProject": "ido_v3_cancel", "withdrawRaisedFunds": "ido_v3_withdraw_raised_funds", "withdrawProjectTokens": "ido_v3_withdraw_project_tokens"}[method]
		if VerifyV3ActionCall(action, 42, data) == nil {
			t.Fatalf("wallet/Safe call admitted as relayer action: %s", action)
		}
	}
	addr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	params := V3CreateParams{ProjectID: big.NewInt(42), ProjectToken: addr,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		ProjectOwner: addr, FeeReceiver: addr, Price: big.NewInt(1), HardCap: big.NewInt(10), SoftCap: big.NewInt(5),
		MaxPerUser: big.NewInt(8), Start: big.NewInt(100), End: big.NewInt(200),
		VestingTimes: []*big.Int{big.NewInt(300)}, VestingPercents: []*big.Int{big.NewInt(10000)}}
	data, err := PackV3Create(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyV3ActionCall("ido_v3_create", 42, data); err != nil {
		t.Fatal(err)
	}
	if VerifyV3ActionCall("ido_v3_create", 43, data) == nil {
		t.Fatal("wrong creation project accepted")
	}
	if VerifyV3ActionCall("ido_v3_activate", 42, data) == nil {
		t.Fatal("creation payload routed as activation")
	}
}
