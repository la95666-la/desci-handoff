package chain

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestValidateV2CoreOperator(t *testing.T) {
	core := common.HexToAddress("0x1111111111111111111111111111111111111111")
	job := common.HexToAddress("0x2222222222222222222222222222222222222222")
	for _, tc := range []struct {
		name, configured string
		job, actual      common.Address
		valid            bool
	}{
		{"separated roles", core.Hex(), job, core, true},
		{"missing explicit core", "", job, core, false},
		{"zero core", (common.Address{}).Hex(), job, core, false},
		{"operator rotated", core.Hex(), job, job, false},
		{"shared signer", core.Hex(), core, core, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateV2CoreOperator(tc.configured, tc.job, tc.actual)
			if (err == nil) != tc.valid {
				t.Fatalf("admission err=%v valid=%t", err, tc.valid)
			}
		})
	}
}
