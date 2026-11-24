package project

import "testing"

func TestFairLaunchPositionActiveUsesEstimatedAllocationOnly(t *testing.T) {
	projection, err := BuildFairLaunchPositionProjection(FairLaunchPositionInput{
		ChainStatus:     FairLaunchChainStatusActive,
		ContributionRaw: "9007199254740993", ClaimedRaw: "0",
		TotalRaisedRaw: "18014398509481987", FinalTotalRaisedRaw: "0",
		SaleTokenAmountRaw: "1000000000000000000000000",
		PaymentDecimals:    6, ProjectDecimals: 18,
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.EstimatedAllocation.State != FinancialStateAvailable || projection.EstimatedAllocation.Raw == nil || *projection.EstimatedAllocation.Raw != "499999999999999972244424" {
		t.Fatalf("wrong estimated allocation: %+v", projection.EstimatedAllocation)
	}
	if projection.FinalAllocation.State != FinancialStatePendingSettlement || projection.Claimable.State != FinancialStatePendingSettlement {
		t.Fatalf("active position exposed final entitlement: %+v", projection)
	}
}

func TestFairLaunchPositionPendingReleaseFreezesFinalAllocationButNotClaimable(t *testing.T) {
	projection, err := BuildFairLaunchPositionProjection(FairLaunchPositionInput{
		ChainStatus:     FairLaunchChainStatusSuccessfulPendingRelease,
		ContributionRaw: "25", ClaimedRaw: "0", TotalRaisedRaw: "999",
		FinalTotalRaisedRaw: "100", SaleTokenAmountRaw: "1000",
		PaymentDecimals: 2, ProjectDecimals: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.FinalAllocation.Raw == nil || *projection.FinalAllocation.Raw != "250" {
		t.Fatalf("final allocation did not use finalTotalRaised: %+v", projection.FinalAllocation)
	}
	if projection.Claimable.State != FinancialStatePendingRelease || projection.Claimable.Raw != nil {
		t.Fatalf("pending release exposed claimable raw amount: %+v", projection.Claimable)
	}
}

func TestFairLaunchPositionSuccessfulSubtractsClaimedExactly(t *testing.T) {
	projection, err := BuildFairLaunchPositionProjection(FairLaunchPositionInput{
		ChainStatus:     FairLaunchChainStatusSuccessful,
		ContributionRaw: "25", ClaimedRaw: "150", TotalRaisedRaw: "100",
		FinalTotalRaisedRaw: "100", SaleTokenAmountRaw: "1000",
		PaymentDecimals: 2, ProjectDecimals: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.Claimable.Raw == nil || *projection.Claimable.Raw != "100" || projection.Claimable.Display == nil || *projection.Claimable.Display != "1" {
		t.Fatalf("wrong exact claimable: %+v", projection.Claimable)
	}
}

func TestFairLaunchPositionFailedExposesRefundOnlyUntilRefunded(t *testing.T) {
	input := FairLaunchPositionInput{
		ChainStatus:     FairLaunchChainStatusFailed,
		ContributionRaw: "1234567", ClaimedRaw: "0", TotalRaisedRaw: "1234567",
		FinalTotalRaisedRaw: "0", SaleTokenAmountRaw: "1000",
		PaymentDecimals: 6, ProjectDecimals: 18,
	}
	projection, err := BuildFairLaunchPositionProjection(input)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Refund.Raw == nil || *projection.Refund.Raw != "1234567" || projection.FinalAllocation.State != FinancialStateNotApplicable {
		t.Fatalf("wrong failed projection: %+v", projection)
	}
	input.Refunded = true
	projection, err = BuildFairLaunchPositionProjection(input)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Refund.Raw == nil || *projection.Refund.Raw != "0" {
		t.Fatalf("refunded projection retained refundable amount: %+v", projection.Refund)
	}
}

func TestFairLaunchPositionRejectsClaimedAboveFinalAllocation(t *testing.T) {
	_, err := BuildFairLaunchPositionProjection(FairLaunchPositionInput{
		ChainStatus:     FairLaunchChainStatusSuccessful,
		ContributionRaw: "1", ClaimedRaw: "2", TotalRaisedRaw: "1",
		FinalTotalRaisedRaw: "1", SaleTokenAmountRaw: "1",
		PaymentDecimals: 18, ProjectDecimals: 18,
	})
	if err == nil {
		t.Fatal("claimed amount above final allocation accepted")
	}
}
