package project

import (
	"joincare_server/pkg/common/entity"
	"testing"
)

func ptrString(value string) *string { return &value }

func TestResolveMinimumForSet(t *testing.T) {
	trusted := trustedPaymentToken{Symbol: "USDT", Decimals: 6}
	for _, tc := range []struct {
		name      string
		row       entity.Project
		max, want string
		ok        bool
	}{
		{"empty", entity.Project{}, "10000000", "", true},
		{"Setup draft", entity.Project{MinPurchaseDraftAmount: ptrString("2"), MinPurchaseDraftToken: ptrString("USDT")}, "10000000", "2000000", true},
		{"wrong token", entity.Project{MinPurchaseDraftAmount: ptrString("2"), MinPurchaseDraftToken: ptrString("USDC")}, "10000000", "", false},
		{"over cap", entity.Project{MinPurchaseDraftAmount: ptrString("20"), MinPurchaseDraftToken: ptrString("USDT")}, "10000000", "", false},
		{"existing", entity.Project{PaymentToken: "USDT", PaymentTokenDecimals: 6, MinPurchaseAmountRaw: ptrString("2000000")}, "10000000", "2000000", true},
		{"existing changed token", entity.Project{PaymentToken: "USDC", PaymentTokenDecimals: 6, MinPurchaseAmountRaw: ptrString("2000000")}, "10000000", "", false},
		{"existing reduced cap", entity.Project{PaymentToken: "USDT", PaymentTokenDecimals: 6, MinPurchaseAmountRaw: ptrString("2000000")}, "1000000", "", false},
	} {
		got, err := resolveMinimumForSet(tc.row, trusted, tc.max)
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if tc.ok && ((got == nil && tc.want != "") || (got != nil && *got != tc.want)) {
			t.Errorf("%s: got %v, want %q", tc.name, got, tc.want)
		}
	}
}

func TestValidateSetupMinimumForSet(t *testing.T) {
	for _, tc := range []struct {
		amount   string
		decimals int
		max      string
		raw      string
		ok       bool
	}{
		{"0.000001", 6, "1000000", "1", true},
		{"1", 6, "1000000", "1000000", true},
		{"1.0000001", 6, "10000000", "", false},
		{"2", 6, "1000000", "", false},
		{"0", 6, "1000000", "", false},
		{"1e1", 6, "100000000", "", false},
		{"1", 0, "1000000", "", false},
		{"1", 6, "0", "", false},
	} {
		raw, err := validateSetupMinimumForSet(tc.amount, tc.decimals, tc.max)
		if (err == nil) != tc.ok {
			t.Errorf("%q/%d/%s: %v", tc.amount, tc.decimals, tc.max, err)
		}
		if tc.ok && (raw == nil || *raw != tc.raw) {
			t.Errorf("%q: raw=%v, want %s", tc.amount, raw, tc.raw)
		}
	}
}
