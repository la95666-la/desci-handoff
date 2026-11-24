package project

import (
	"math/big"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestFinalizePortfolioPositionUsesArbitraryPrecisionRawAmounts(t *testing.T) {
	bought, ok := new(big.Int).SetString("57896044618658097711785492504343953926634992332820282019728792003956564819968", 10)
	if !ok {
		t.Fatal("parse bought amount")
	}
	claimed := new(big.Int).Quo(new(big.Int).Set(bought), big.NewInt(4))
	now := time.Now()
	position := V2PortfolioPosition{Status: constant.ProjectStatusSuccessful, PaymentTokenDecimals: 6, ProjectTokenDecimals: 18}
	schedule := []entity.ProjectTge{
		{Index: 1, VestingTime: now.Add(-time.Hour), VestingPercent: decimal.RequireFromString("50")},
		{Index: 2, VestingTime: now.Add(time.Hour), VestingPercent: decimal.RequireFromString("50")},
	}
	if err := finalizePortfolioPosition(&position, big.NewInt(123456789), bought, claimed, new(big.Int), schedule, now); err != nil {
		t.Fatal(err)
	}
	expectedClaimable := new(big.Int).Quo(new(big.Int).Set(bought), big.NewInt(2))
	expectedClaimable.Sub(expectedClaimable, claimed)
	if position.ClaimableRaw != expectedClaimable.String() {
		t.Fatalf("claimable raw=%s want=%s", position.ClaimableRaw, expectedClaimable)
	}
	if position.NextClaimTime != schedule[1].VestingTime.Unix() {
		t.Fatalf("next claim time=%d", position.NextClaimTime)
	}
}

func TestFinalizePortfolioPositionRefundIsBoundedAtZero(t *testing.T) {
	position := V2PortfolioPosition{Status: constant.ProjectStatusFailed, PaymentTokenDecimals: 6, ProjectTokenDecimals: 18}
	if err := finalizePortfolioPosition(&position, big.NewInt(100), big.NewInt(10), new(big.Int), big.NewInt(120), nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if position.RefundableRaw != "0" || position.RefundableDisplay != "0" {
		t.Fatalf("over-refunded position exposed negative refund: %+v", position)
	}
}

func TestFinalizePortfolioPositionSupportsV3VestingAndRefundStates(t *testing.T) {
	now := time.Now()
	position := V2PortfolioPosition{Status: constant.ProjectStatusSuccessful, ContractVersion: constant.ContractVersionIDOV3Stablecoin, PaymentTokenDecimals: 6, ProjectTokenDecimals: 18}
	schedule := []entity.ProjectTge{
		{Index: 1, VestingTime: now.Add(-time.Hour), VestingPercent: decimal.NewFromInt(25)},
		{Index: 2, VestingTime: now.Add(time.Hour), VestingPercent: decimal.NewFromInt(75)},
	}
	if err := finalizePortfolioPosition(&position, big.NewInt(100), big.NewInt(1000), big.NewInt(100), new(big.Int), schedule, now); err != nil {
		t.Fatal(err)
	}
	if position.ClaimableRaw != "150" || position.NextClaimRaw != "750" || position.NextClaimTime == 0 {
		t.Fatalf("unexpected V3 claim projection: %+v", position)
	}
	position.Status = constant.ProjectStatusCanceled
	if err := finalizePortfolioPosition(&position, big.NewInt(100), big.NewInt(1000), big.NewInt(100), big.NewInt(25), schedule, now); err != nil {
		t.Fatal(err)
	}
	if position.RefundableRaw != "75" {
		t.Fatalf("unexpected V3 refund projection: %+v", position)
	}
}
