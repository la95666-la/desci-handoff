package projecttracking

import (
	"github.com/ethereum/go-ethereum/common"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
	"math/big"
	"testing"
)

func settledTotals() commonchain.V3ReplayTotals {
	return commonchain.V3ReplayTotals{State: idov3whitelist.ProjectSuccessful, Raised: big.NewInt(1000), Sold: big.NewInt(400), Deposited: big.NewInt(500), Refunded: big.NewInt(0), Claimed: big.NewInt(400), ProceedsPaid: big.NewInt(980), FeePaid: big.NewInt(20), ProjectTokensWithdrawn: big.NewInt(100)}
}

func TestClosureFeeFloorAndIndividualWalletObligations(t *testing.T) {
	totals := settledTotals()
	totals.Raised = big.NewInt(1001)
	totals.ProceedsPaid = big.NewInt(981)
	flags := ExitFlags{Funds: true, Fee: true, Tokens: true, Deposited: "500"}
	if got := ClosureReason(totals, 200, flags); got != "" {
		t.Fatal(got)
	}
	totals.Wallets = map[common.Address]commonchain.V3ReplayWallet{common.HexToAddress("0x1111111111111111111111111111111111111111"): {Payment: big.NewInt(1001), Tokens: big.NewInt(400), Claimed: big.NewInt(399), Refunded: big.NewInt(0)}}
	if got := ClosureReason(totals, 200, flags); got != "user_claims_pending" {
		t.Fatal(got)
	}
}
func TestSuccessfulClosureRequiresEveryAssetAndUserObligation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*commonchain.V3ReplayTotals, *ExitFlags)
		want string
	}{
		{"settled", func(*commonchain.V3ReplayTotals, *ExitFlags) {}, ""},
		{"user unclaimed", func(t *commonchain.V3ReplayTotals, _ *ExitFlags) { t.Claimed = big.NewInt(399) }, "user_claims_pending"},
		{"fee flag", func(_ *commonchain.V3ReplayTotals, f *ExitFlags) { f.Fee = false }, "platform_fee_pending"},
		{"owner payment", func(t *commonchain.V3ReplayTotals, _ *ExitFlags) { t.ProceedsPaid = big.NewInt(979) }, "project_proceeds_pending"},
		{"token flag", func(_ *commonchain.V3ReplayTotals, f *ExitFlags) { f.Tokens = false }, "project_tokens_pending"},
		{"token amount", func(t *commonchain.V3ReplayTotals, _ *ExitFlags) { t.ProjectTokensWithdrawn = big.NewInt(99) }, "project_tokens_pending"},
		{"not terminal", func(t *commonchain.V3ReplayTotals, _ *ExitFlags) { t.State = idov3whitelist.ProjectActive }, "lifecycle_not_terminal"},
		{"chain mismatch", func(_ *commonchain.V3ReplayTotals, f *ExitFlags) { f.Deposited = "501" }, "chain_totals_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			totals := settledTotals()
			flags := ExitFlags{Funds: true, Fee: true, Tokens: true, Deposited: "500"}
			tc.edit(&totals, &flags)
			if got := ClosureReason(totals, 200, flags); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
}
func TestFailedAndCancelledClosureIncludesRefundsAndAllDeposits(t *testing.T) {
	for _, state := range []idov3whitelist.ProjectState{idov3whitelist.ProjectFailed, idov3whitelist.ProjectCancelled} {
		totals := settledTotals()
		totals.State = state
		totals.Refunded = big.NewInt(1000)
		totals.Claimed = big.NewInt(0)
		totals.ProjectTokensWithdrawn = big.NewInt(500)
		flags := ExitFlags{Tokens: true, Deposited: "500"}
		if got := ClosureReason(totals, 200, flags); got != "" {
			t.Fatal(got)
		}
		totals.Refunded = big.NewInt(999)
		if got := ClosureReason(totals, 200, flags); got != "user_refunds_pending" {
			t.Fatal(got)
		}
	}
}
func TestZeroAmountsStillRequireExecutedExitFlags(t *testing.T) {
	totals := settledTotals()
	totals.State = idov3whitelist.ProjectCancelled
	totals.Raised = big.NewInt(0)
	totals.Deposited = big.NewInt(0)
	totals.Refunded = big.NewInt(0)
	totals.ProjectTokensWithdrawn = big.NewInt(0)
	if got := ClosureReason(totals, 0, ExitFlags{Deposited: "0"}); got != "project_tokens_pending" {
		t.Fatal(got)
	}
	if got := ClosureReason(totals, 0, ExitFlags{Tokens: true, Deposited: "0"}); got != "" {
		t.Fatal(got)
	}
}
