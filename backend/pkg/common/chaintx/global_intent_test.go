package chaintx

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func baseIntent() Intent {
	return Intent{
		ProjectID:       42,
		Action:          "v2_create",
		ChainID:         97,
		SenderAddress:   common.HexToAddress("0x1000000000000000000000000000000000000001"),
		ContractAddress: common.HexToAddress("0x2000000000000000000000000000000000000002"),
		CallData:        []byte{0x01, 0x02},
	}
}

// 安全审计 HIGH-1：全局治理 action 使用 project_id=0；非全局 intent 禁止 0，
// 全局 intent 禁止携带 project_id，防止作用域混用。
func TestValidateIntentShapeGlobalScopeRules(t *testing.T) {
	if err := validateIntentShape(baseIntent()); err != nil {
		t.Fatalf("business intent rejected: %v", err)
	}

	global := baseIntent()
	global.ProjectID = 0
	global.Global = true
	global.Action = "global_pause"
	if err := validateIntentShape(global); err != nil {
		t.Fatalf("global governance intent rejected: %v", err)
	}

	nonGlobalZero := baseIntent()
	nonGlobalZero.ProjectID = 0
	if err := validateIntentShape(nonGlobalZero); err == nil {
		t.Fatal("non-global intent with project_id=0 accepted")
	}

	globalWithProject := global
	globalWithProject.ProjectID = 7
	if err := validateIntentShape(globalWithProject); err == nil {
		t.Fatal("global intent carrying a project id accepted")
	}

	for name, mutate := range map[string]func(*Intent){
		"empty action":   func(i *Intent) { i.Action = "" },
		"zero chain":     func(i *Intent) { i.ChainID = 0 },
		"zero sender":    func(i *Intent) { i.SenderAddress = common.Address{} },
		"zero contract":  func(i *Intent) { i.ContractAddress = common.Address{} },
		"empty calldata": func(i *Intent) { i.CallData = nil },
	} {
		broken := baseIntent()
		mutate(&broken)
		if err := validateIntentShape(broken); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
