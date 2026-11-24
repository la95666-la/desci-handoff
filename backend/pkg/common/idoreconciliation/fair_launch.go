package idoreconciliation

import (
	"errors"
	"math/big"
)

type FairLaunchProjectEvidence struct {
	SaleTokenAmountRaw              string
	FinalTotalRaisedRaw             string
	FinalPriceNumeratorRaw          string
	FinalPriceDenominatorRaw        string
	ProjectOwnerProceedsRaw         string
	PlatformFeeRaw                  string
	PaymentReserveRemainingRaw      string
	TotalTokensClaimedRaw           string
	ProjectTokenReserveRemainingRaw string
}
type FairLaunchReconciliationResult struct {
	State      string          `json:"state"`
	Checks     map[string]bool `json:"checks"`
	Mismatches []string        `json:"mismatches"`
}

func ReconcileFairLaunchProject(e FairLaunchProjectEvidence) (FairLaunchReconciliationResult, error) {
	parse := func(v string) (*big.Int, error) {
		n, ok := new(big.Int).SetString(v, 10)
		if !ok || n.Sign() < 0 {
			return nil, errors.New("invalid canonical raw amount")
		}
		return n, nil
	}
	sale, err := parse(e.SaleTokenAmountRaw)
	if err != nil {
		return FairLaunchReconciliationResult{}, err
	}
	raised, err := parse(e.FinalTotalRaisedRaw)
	if err != nil {
		return FairLaunchReconciliationResult{}, err
	}
	num, err := parse(e.FinalPriceNumeratorRaw)
	if err != nil {
		return FairLaunchReconciliationResult{}, err
	}
	den, err := parse(e.FinalPriceDenominatorRaw)
	if err != nil {
		return FairLaunchReconciliationResult{}, err
	}
	proceeds, _ := parse(e.ProjectOwnerProceedsRaw)
	fee, _ := parse(e.PlatformFeeRaw)
	paymentReserve, _ := parse(e.PaymentReserveRemainingRaw)
	claimed, _ := parse(e.TotalTokensClaimedRaw)
	tokenReserve, _ := parse(e.ProjectTokenReserveRemainingRaw)
	r := FairLaunchReconciliationResult{State: "BALANCED", Checks: map[string]bool{}}
	r.Checks["final_price_numerator"] = num.Cmp(raised) == 0
	r.Checks["final_price_denominator"] = den.Cmp(sale) == 0
	r.Checks["payment_conservation"] = new(big.Int).Add(new(big.Int).Add(proceeds, fee), paymentReserve).Cmp(raised) == 0
	r.Checks["project_token_conservation"] = new(big.Int).Add(claimed, tokenReserve).Cmp(sale) == 0
	for k, ok := range r.Checks {
		if !ok {
			r.State = "UNBALANCED"
			r.Mismatches = append(r.Mismatches, k)
		}
	}
	return r, nil
}
