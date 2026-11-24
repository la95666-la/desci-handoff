package idov3whitelist

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestApprovedSafeCodeHashesRequireBothCanonicalIndependentInputs(t *testing.T) {
	good := "0x" + strings.Repeat("a", 64)
	other := "0x" + strings.Repeat("b", 64)
	owner, governor, err := ParseApprovedV3SafeCodeHashes(good, other)
	if err != nil || owner != common.HexToHash(good) || governor != common.HexToHash(other) {
		t.Fatalf("reviewed deployment hashes rejected: %v", err)
	}
	for _, pair := range [][2]string{
		{"", other}, {good, ""}, {good, "0x" + strings.Repeat("0", 64)},
		{good, strings.ToUpper(other)}, {good, other[2:]},
		{good, "0x" + strings.Repeat("z", 64)},
	} {
		if _, _, err := ParseApprovedV3SafeCodeHashes(pair[0], pair[1]); err == nil {
			t.Fatalf("invalid Safe code identities admitted")
		}
	}
}
