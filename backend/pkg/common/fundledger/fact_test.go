package fundledger

import (
	"strings"
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func validFact() entity.FundLedger {
	return entity.FundLedger{
		ChainID:         97,
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ContractVersion: constant.ContractVersionV2Stablecoin,
		ProjectID:       42,
		WalletAddress:   "0x2222222222222222222222222222222222222222",
		EntryType:       EntryPurchasePayment,
		AssetRole:       AssetRolePayment,
		TokenSymbol:     constant.PaymentTokenUSDT,
		TokenAddress:    "0x3333333333333333333333333333333333333333",
		TokenDecimals:   6,
		AmountRaw:       "1234567",
		AmountDisplay:   "1.234567",
		TxHash:          "0x" + strings.Repeat("a", 64),
		LogIndex:        3,
		BlockNumber:     100,
		BlockHash:       "0x" + strings.Repeat("b", 64),
		Status:          StatusCanonical,
		EventData:       `{}`,
	}
}

func TestValidateAcceptsCanonicalFact(t *testing.T) {
	fact := validFact()
	if err := Validate(&fact); err != nil {
		t.Fatalf("valid fact rejected: %v", err)
	}
}

func TestValidateAcceptsFairLaunchContributionWithoutFixedEntitlement(t *testing.T) {
	fact := validFact()
	fact.ContractVersion = constant.ContractVersionFairLaunchV1
	fact.EntryType = EntryPaymentContributed
	if err := Validate(&fact); err != nil {
		t.Fatalf("valid fair launch contribution rejected: %v", err)
	}
	fact.EntryType = EntryPurchasePayment
	if err := Validate(&fact); err == nil {
		t.Fatal("fair launch contribution accepted as fixed-price purchase")
	}
}

func TestValidateRejectsDisplayOrIdentityMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*entity.FundLedger)
	}{
		{name: "display mismatch", mutate: func(f *entity.FundLedger) { f.AmountDisplay = "1.23" }},
		{name: "noncanonical raw", mutate: func(f *entity.FundLedger) { f.AmountRaw = "001234567" }},
		{name: "legacy contract", mutate: func(f *entity.FundLedger) { f.ContractVersion = constant.ContractVersionV1Legacy }},
		{name: "role mismatches entry", mutate: func(f *entity.FundLedger) { f.EntryType = EntryProjectTokenClaimed }},
		{name: "unknown entry", mutate: func(f *entity.FundLedger) { f.EntryType = "purchase" }},
		{name: "bad tx hash", mutate: func(f *entity.FundLedger) { f.TxHash = "0x01" }},
		{name: "zero token", mutate: func(f *entity.FundLedger) { f.TokenAddress = "0x0000000000000000000000000000000000000000" }},
		{name: "invalid event JSON", mutate: func(f *entity.FundLedger) { f.EventData = "{" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fact := validFact()
			test.mutate(&fact)
			if err := Validate(&fact); err == nil {
				t.Fatal("invalid fact accepted")
			}
		})
	}
}
