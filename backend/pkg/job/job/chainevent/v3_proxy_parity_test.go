package chainevent

import (
	"github.com/ethereum/go-ethereum/common"
	commonchain "joincare_server/pkg/common/chain"
	"math/big"
	"testing"
)

func TestProxyEconomicParityIncludesEveryCounterAndWallet(t *testing.T) {
	z := func() *big.Int { return new(big.Int) }
	totals := commonchain.V3ReplayTotals{Raised: z(), Sold: z(), Deposited: z(), Refunded: z(), Claimed: z(), ProceedsPaid: z(), FeePaid: z(), ProjectTokensWithdrawn: z(), Wallets: map[common.Address]commonchain.V3ReplayWallet{}}
	expected, e := v3ExpectedProjection(totals)
	if e != nil {
		t.Fatal(e)
	}
	copy := expected
	copy.Wallets = "{ }"
	if !v3ProjectionEqual(copy, expected) {
		t.Fatal("JSON whitespace")
	}
	for _, field := range []*string{&copy.RaisedRaw, &copy.SoldRaw, &copy.DepositedRaw, &copy.RefundedRaw, &copy.ClaimedRaw, &copy.ProceedsPaidRaw, &copy.FeePaidRaw, &copy.ProjectTokensWithdrawnRaw} {
		old := *field
		*field = "1"
		if v3ProjectionEqual(copy, expected) {
			t.Fatal("counter omitted")
		}
		*field = old
	}
	copy.Wallets = `{"0x01":{"payment":"1"}}`
	if v3ProjectionEqual(copy, expected) {
		t.Fatal("wallet omitted")
	}
}
