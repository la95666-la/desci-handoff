package idoreconciliation

import "testing"

func TestFairLaunchReconciliation(t *testing.T) {
	r, e := ReconcileFairLaunchProject(FairLaunchProjectEvidence{SaleTokenAmountRaw: "100", FinalTotalRaisedRaw: "30", FinalPriceNumeratorRaw: "30", FinalPriceDenominatorRaw: "100", ProjectOwnerProceedsRaw: "20", PlatformFeeRaw: "5", PaymentReserveRemainingRaw: "5", TotalTokensClaimedRaw: "40", ProjectTokenReserveRemainingRaw: "60"})
	if e != nil || r.State != "BALANCED" {
		t.Fatalf("%+v %v", r, e)
	}
	r, _ = ReconcileFairLaunchProject(FairLaunchProjectEvidence{SaleTokenAmountRaw: "100", FinalTotalRaisedRaw: "30", FinalPriceNumeratorRaw: "29", FinalPriceDenominatorRaw: "100", ProjectOwnerProceedsRaw: "20", PlatformFeeRaw: "5", PaymentReserveRemainingRaw: "5", TotalTokensClaimedRaw: "40", ProjectTokenReserveRemainingRaw: "60"})
	if r.State != "UNBALANCED" {
		t.Fatal(r)
	}
}
