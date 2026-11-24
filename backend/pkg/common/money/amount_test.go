package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFormatRaw(t *testing.T) {
	tests := []struct {
		raw      string
		decimals uint8
		want     string
	}{
		{raw: "0", decimals: 6, want: "0"},
		{raw: "0001000000", decimals: 6, want: "1"},
		{raw: "1", decimals: 6, want: "0.000001"},
		{raw: "123456789", decimals: 6, want: "123.456789"},
		{raw: "1000000000000000000", decimals: 18, want: "1"},
		{raw: "1234500000000000000", decimals: 18, want: "1.2345"},
	}
	for _, test := range tests {
		got, err := FormatRaw(test.raw, test.decimals)
		if err != nil || got != test.want {
			t.Fatalf("FormatRaw(%q,%d)=%q err=%v want=%q", test.raw, test.decimals, got, err, test.want)
		}
	}
}

func TestDisplayToRaw(t *testing.T) {
	tests := []struct {
		display  string
		decimals uint8
		want     string
	}{
		{display: "0", decimals: 6, want: "0"},
		{display: "1", decimals: 6, want: "1000000"},
		{display: "1.234567", decimals: 6, want: "1234567"},
		{display: "0.000000000000000001", decimals: 18, want: "1"},
	}
	for _, test := range tests {
		got, err := DisplayToRaw(decimal.RequireFromString(test.display), test.decimals)
		if err != nil || got != test.want {
			t.Fatalf("DisplayToRaw(%s,%d)=%q err=%v want=%q", test.display, test.decimals, got, err, test.want)
		}
	}
	if _, err := DisplayToRaw(decimal.RequireFromString("1.0000001"), 6); err == nil {
		t.Fatal("over-precision display amount accepted")
	}
}

func TestNormalizeRawUint256RejectsInvalidAndOverflow(t *testing.T) {
	invalid := []string{"", "-1", "+1", "1.0", "1e6", "0x01", "abc", "115792089237316195423570985008687907853269984665640564039457584007913129639936"}
	for _, value := range invalid {
		if _, err := NormalizeRawUint256(value); err == nil {
			t.Fatalf("invalid raw amount accepted: %q", value)
		}
	}
	max := "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	if got, err := NormalizeRawUint256(max); err != nil || got != max {
		t.Fatalf("uint256 max rejected: got=%q err=%v", got, err)
	}
}
