package projecttracking

import (
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
	"math/big"
)

// Chain flags matter even for zero-value withdrawals: a zero amount is not
// evidence of an executed withdrawal. Evaluation never requests a transaction.
type ExitFlags struct {
	Funds, Fee, Tokens bool
	Deposited          string
}

func ClosureReason(t commonchain.V3ReplayTotals, feeRate uint16, f ExitFlags) string {
	all := []*big.Int{t.Raised, t.Sold, t.Deposited, t.Refunded, t.Claimed, t.ProceedsPaid, t.FeePaid, t.ProjectTokensWithdrawn}
	for _, n := range all {
		if n == nil || n.Sign() < 0 || n.BitLen() > 256 {
			return "invalid_totals"
		}
	}
	deposited, ok := new(big.Int).SetString(f.Deposited, 10)
	if !ok || deposited.String() != f.Deposited || deposited.Cmp(t.Deposited) != 0 || feeRate > 1000 {
		return "chain_totals_mismatch"
	}
	switch t.State {
	case idov3whitelist.ProjectSuccessful:
		fee := new(big.Int).Div(new(big.Int).Mul(t.Raised, new(big.Int).SetUint64(uint64(feeRate))), big.NewInt(10000))
		owner := new(big.Int).Sub(t.Raised, fee)
		if !f.Funds || t.ProceedsPaid.Cmp(owner) != 0 {
			return "project_proceeds_pending"
		}
		if !f.Fee || t.FeePaid.Cmp(fee) != 0 {
			return "platform_fee_pending"
		}
		if t.Claimed.Cmp(t.Sold) != 0 {
			return "user_claims_pending"
		}
		if !f.Tokens || new(big.Int).Add(t.Claimed, t.ProjectTokensWithdrawn).Cmp(t.Deposited) != 0 {
			return "project_tokens_pending"
		}
	case idov3whitelist.ProjectFailed, idov3whitelist.ProjectCancelled:
		if t.Refunded.Cmp(t.Raised) != 0 {
			return "user_refunds_pending"
		}
		if !f.Tokens || t.ProjectTokensWithdrawn.Cmp(t.Deposited) != 0 {
			return "project_tokens_pending"
		}
	default:
		return "lifecycle_not_terminal"
	}
	for _, w := range t.Wallets {
		if w.Payment == nil || w.Tokens == nil || w.Refunded == nil || w.Claimed == nil {
			return "invalid_wallet_totals"
		}
		if t.State == idov3whitelist.ProjectSuccessful && w.Claimed.Cmp(w.Tokens) != 0 {
			return "user_claims_pending"
		}
		if t.State != idov3whitelist.ProjectSuccessful && w.Refunded.Cmp(w.Payment) != 0 {
			return "user_refunds_pending"
		}
	}
	return ""
}
