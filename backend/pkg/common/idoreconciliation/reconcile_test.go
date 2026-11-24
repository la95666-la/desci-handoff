package idoreconciliation

import (
	"math/big"
	"testing"
)

func TestCanonicalUintAndChecksFailClosed(t *testing.T) {
	maximum := "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	for _, value := range []string{"0", "1", maximum} {
		if _, ok := canonicalUint(value); !ok {
			t.Fatalf("canonical uint rejected: %s", value)
		}
	}
	for _, value := range []string{"", "00", "01", "-1", "+1", "1.0", "1e2", maximum + "0"} {
		if _, ok := canonicalUint(value); ok {
			t.Fatalf("invalid uint accepted: %q", value)
		}
	}
	if AllChecksOK(nil) || AllChecksOK([]Check{}) {
		t.Fatal("empty evidence reported balanced")
	}
	if AllChecksOK([]Check{{Code: "x", OK: false}}) {
		t.Fatal("failed check reported balanced")
	}
	if !AllChecksOK([]Check{{Code: "x", OK: true}}) {
		t.Fatal("passing non-empty checks rejected")
	}
}

func TestUintStringRejectsInvalidValues(t *testing.T) {
	if UintString(nil) != "invalid" || UintString(big.NewInt(-1)) != "invalid" {
		t.Fatal("nil or negative integer accepted")
	}
	overflow := new(big.Int).Lsh(big.NewInt(1), 256)
	if UintString(overflow) != "invalid" {
		t.Fatal("uint256 overflow accepted")
	}
	if UintString(big.NewInt(0)) != "0" {
		t.Fatal("zero was not canonical")
	}
}

func TestIdentityValidatorsAreExact(t *testing.T) {
	if validAddress("") || validAddress("0x1") || validAddress("0x0000000000000000000000000000000000000000") {
		t.Fatal("invalid address accepted")
	}
	if !validAddress("0x1000000000000000000000000000000000000001") {
		t.Fatal("valid address rejected")
	}
	if validHash("0x1") || validHash("0x"+string(make([]byte, 64))) {
		t.Fatal("invalid hash accepted")
	}
	if !validHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("valid hash rejected")
	}
}
