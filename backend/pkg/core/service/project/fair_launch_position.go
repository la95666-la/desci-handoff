package project

import (
	"fmt"
	"math/big"

	"joincare_server/pkg/common/money"
)

const (
	FinancialStatePendingRelease = "pending_release"
	FinancialStateUnavailable    = "unavailable"
)

// FairLaunchAmount is a discriminated financial value. Raw and display are
// populated only when State is available, so zero cannot stand for unrelated
// meanings such as pending settlement or not applicable.
type FairLaunchAmount struct {
	State   string  `json:"state"`
	Raw     *string `json:"raw"`
	Display *string `json:"display"`
}

// FairLaunchPositionInput contains canonical chain-derived values needed for a
// wallet position projection. It performs no database or RPC authority check.
type FairLaunchPositionInput struct {
	ChainStatus         string
	ContributionRaw     string
	ClaimedRaw          string
	TotalRaisedRaw      string
	FinalTotalRaisedRaw string
	SaleTokenAmountRaw  string
	PaymentDecimals     uint8
	ProjectDecimals     uint8
	Refunded            bool
}

// FairLaunchPositionProjection preserves the distinction between estimated
// allocation during Active and final allocation after successful settlement.
type FairLaunchPositionProjection struct {
	Contribution        FairLaunchAmount `json:"contribution"`
	EstimatedAllocation FairLaunchAmount `json:"estimatedAllocation"`
	FinalAllocation     FairLaunchAmount `json:"finalAllocation"`
	Claimed             FairLaunchAmount `json:"claimed"`
	Claimable           FairLaunchAmount `json:"claimable"`
	Refund              FairLaunchAmount `json:"refund"`
}

func BuildFairLaunchPositionProjection(input FairLaunchPositionInput) (FairLaunchPositionProjection, error) {
	contribution, err := availableAmount(input.ContributionRaw, input.PaymentDecimals)
	if err != nil {
		return FairLaunchPositionProjection{}, fmt.Errorf("contribution: %w", err)
	}
	claimed, err := availableAmount(input.ClaimedRaw, input.ProjectDecimals)
	if err != nil {
		return FairLaunchPositionProjection{}, fmt.Errorf("claimed: %w", err)
	}
	projection := FairLaunchPositionProjection{
		Contribution:        contribution,
		Claimed:             claimed,
		EstimatedAllocation: stateAmount(FinancialStateNotApplicable),
		FinalAllocation:     stateAmount(FinancialStatePendingSettlement),
		Claimable:           stateAmount(FinancialStatePendingSettlement),
		Refund:              stateAmount(FinancialStateNotApplicable),
	}

	switch input.ChainStatus {
	case FairLaunchChainStatusNotCreated, FairLaunchChainStatusPending:
		projection.FinalAllocation = stateAmount(FinancialStatePendingSettlement)
		projection.Claimable = stateAmount(FinancialStatePendingSettlement)
	case FairLaunchChainStatusActive:
		if input.TotalRaisedRaw == "0" {
			projection.EstimatedAllocation = stateAmount(FinancialStateUnavailable)
			return projection, nil
		}
		allocation, err := FairLaunchAllocationRaw(input.ContributionRaw, input.SaleTokenAmountRaw, input.TotalRaisedRaw)
		if err != nil {
			return FairLaunchPositionProjection{}, err
		}
		projection.EstimatedAllocation, err = availableAmount(allocation, input.ProjectDecimals)
		if err != nil {
			return FairLaunchPositionProjection{}, err
		}
	case FairLaunchChainStatusSuccessfulPendingRelease, FairLaunchChainStatusSuccessful:
		allocation, err := FairLaunchAllocationRaw(input.ContributionRaw, input.SaleTokenAmountRaw, input.FinalTotalRaisedRaw)
		if err != nil {
			return FairLaunchPositionProjection{}, fmt.Errorf("final allocation: %w", err)
		}
		projection.FinalAllocation, err = availableAmount(allocation, input.ProjectDecimals)
		if err != nil {
			return FairLaunchPositionProjection{}, err
		}
		if input.ChainStatus == FairLaunchChainStatusSuccessfulPendingRelease {
			projection.Claimable = stateAmount(FinancialStatePendingRelease)
			return projection, nil
		}
		claimable, err := subtractRaw(allocation, input.ClaimedRaw)
		if err != nil {
			return FairLaunchPositionProjection{}, err
		}
		projection.Claimable, err = availableAmount(claimable, input.ProjectDecimals)
		if err != nil {
			return FairLaunchPositionProjection{}, err
		}
	case FairLaunchChainStatusFailed, FairLaunchChainStatusCancelled:
		projection.FinalAllocation = stateAmount(FinancialStateNotApplicable)
		projection.Claimable = stateAmount(FinancialStateNotApplicable)
		if input.Refunded {
			projection.Refund, err = availableAmount("0", input.PaymentDecimals)
		} else {
			projection.Refund = contribution
		}
		if err != nil {
			return FairLaunchPositionProjection{}, err
		}
	default:
		return FairLaunchPositionProjection{}, fmt.Errorf("unsupported fair launch chain status %q", input.ChainStatus)
	}
	return projection, nil
}

func stateAmount(state string) FairLaunchAmount {
	return FairLaunchAmount{State: state}
}

func availableAmount(raw string, decimals uint8) (FairLaunchAmount, error) {
	canonical, err := parseRawUint("raw", raw)
	if err != nil {
		return FairLaunchAmount{}, err
	}
	display, err := money.FormatRaw(canonical.String(), decimals)
	if err != nil {
		return FairLaunchAmount{}, err
	}
	rawValue, displayValue := canonical.String(), display
	return FairLaunchAmount{State: FinancialStateAvailable, Raw: &rawValue, Display: &displayValue}, nil
}

func subtractRaw(totalRaw, claimedRaw string) (string, error) {
	total, err := parseRawUint("totalRaw", totalRaw)
	if err != nil {
		return "", err
	}
	claimed, err := parseRawUint("claimedRaw", claimedRaw)
	if err != nil {
		return "", err
	}
	if claimed.Cmp(total) > 0 {
		return "", fmt.Errorf("claimed amount exceeds final allocation")
	}
	return new(big.Int).Sub(total, claimed).String(), nil
}
