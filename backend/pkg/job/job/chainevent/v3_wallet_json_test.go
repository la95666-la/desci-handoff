package chainevent

import (
	"strings"
	"testing"
)

func TestV3WalletProjectionJSONStrictSemanticEquality(t *testing.T) {
	const address = "0x4444444444444444444444444444444444444444"
	raw := `{"` + address + `":{"payment":"9007199254740993","tokens":"200000000000000000000","refunded":"0","claimed":"1"}}`
	a := v3ParityProjection{State: "successful", Wallets: raw}
	b := a
	b.Wallets = `{ "` + address + `": { "tokens": "200000000000000000000", "claimed": "1", "payment": "9007199254740993", "refunded": "0" } }`
	if !v3ProjectionEqual(a, b) {
		t.Fatal("MySQL JSON formatting rejected")
	}
	for name, bad := range map[string]string{
		"payment":   strings.Replace(raw, "9007199254740993", "9007199254740992", 1),
		"tokens":    strings.Replace(raw, "200000000000000000000", "200000000000000000001", 1),
		"refund":    strings.Replace(raw, `"refunded":"0"`, `"refunded":"1"`, 1),
		"claim":     strings.Replace(raw, `"claimed":"1"`, `"claimed":"2"`, 1),
		"numeric":   strings.Replace(raw, `"9007199254740993"`, `9007199254740993`, 1),
		"duplicate": strings.Replace(raw, `"claimed":"1"`, `"claimed":"0","claimed":"1"`, 1),
		"unknown":   strings.Replace(raw, `"claimed":"1"`, `"claimed":"1","other":"0"`, 1),
		"missing":   strings.Replace(raw, `,"claimed":"1"`, "", 1),
		"null":      `null`, "array": `[]`, "trailing": raw + ` {}`,
		"leading_zero":     strings.Replace(raw, `"claimed":"1"`, `"claimed":"01"`, 1),
		"overflow":         strings.Replace(raw, "9007199254740993", strings.Repeat("9", 79), 1),
		"address":          strings.Replace(raw, address, "0x01", 1),
		"no_prefix":        strings.Replace(raw, address, strings.TrimPrefix(address, "0x"), 1),
		"zero_address":     strings.Replace(raw, address, "0x"+strings.Repeat("0", 40), 1),
		"duplicate_wallet": raw[:len(raw)-1] + "," + raw[1:],
	} {
		t.Run(name, func(t *testing.T) {
			b := a
			b.Wallets = bad
			if v3ProjectionEqual(a, b) {
				t.Fatal("invalid or changed wallet accepted")
			}
			switch name {
			case "payment", "tokens", "refund", "claim":
			default:
				if v3ProjectionEqual(b, b) {
					t.Fatal("malformed wallet accepted against itself")
				}
			}
		})
	}
}
