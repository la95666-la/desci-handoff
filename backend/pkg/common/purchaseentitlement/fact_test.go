package purchaseentitlement

import (
	"strings"
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
)

func validFact() entity.IDOPurchaseEntitlement {
	return entity.IDOPurchaseEntitlement{
		FundLedgerID: 1, ChainID: 97, ContractAddress: "0x1000000000000000000000000000000000000001",
		ContractVersion: constant.ContractVersionV2Stablecoin, ProjectID: 42,
		WalletAddress:      "0x2000000000000000000000000000000000000002",
		PaymentTokenSymbol: "USDT", PaymentTokenAddress: "0x3000000000000000000000000000000000000003", PaymentTokenDecimals: 6,
		PaymentAmountRaw: "1000001", PaymentAmountDisplay: "1.000001",
		ProjectTokenSymbol: "SCI", ProjectTokenAddress: "0x4000000000000000000000000000000000000004", ProjectTokenDecimals: 18,
		ProjectTokenAmountRaw: "2500000000000000000", ProjectTokenDisplay: "2.5",
		TxHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64), BlockNumber: 100,
		Status: fundledger.StatusCanonical, EventData: `{}`,
	}
}

func TestValidate(t *testing.T) {
	fact := validFact()
	if err := Validate(&fact); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*entity.IDOPurchaseEntitlement)
	}{
		{"missing ledger", func(f *entity.IDOPurchaseEntitlement) { f.FundLedgerID = 0 }},
		{"same tokens", func(f *entity.IDOPurchaseEntitlement) { f.ProjectTokenAddress = f.PaymentTokenAddress }},
		{"noncanonical payment", func(f *entity.IDOPurchaseEntitlement) { f.PaymentAmountRaw = "01" }},
		{"zero project entitlement", func(f *entity.IDOPurchaseEntitlement) { f.ProjectTokenAmountRaw, f.ProjectTokenDisplay = "0", "0" }},
		{"display mismatch", func(f *entity.IDOPurchaseEntitlement) { f.ProjectTokenDisplay = "2.4" }},
		{"raw event leak shape", func(f *entity.IDOPurchaseEntitlement) { f.EventData = "{" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copy := fact
			test.mutate(&copy)
			if err := Validate(&copy); err == nil {
				t.Fatal("invalid entitlement accepted")
			}
		})
	}
}

func TestMatchesPurchaseFormula(t *testing.T) {
	for _, fixture := range []struct {
		payment, project, price string
		decimals                uint8
	}{
		{"1000001", "1000001000000000000", "1000000", 18},
		{"1", "333333333333", "3000000", 18},
		{"1000000000000000000", "500000", "2000000000000000000", 6},
	} {
		if !MatchesPurchaseFormula(fixture.payment, fixture.project, fixture.price, fixture.decimals) {
			t.Fatalf("valid formula rejected: %+v", fixture)
		}
	}
	if MatchesPurchaseFormula("1000001", "1000001000000000001", "1000000", 18) {
		t.Fatal("project-token amount drift accepted")
	}
}
