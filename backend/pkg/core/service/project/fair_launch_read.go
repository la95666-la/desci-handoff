package project

import (
	"fmt"
	"math/big"
	"strings"

	"joincare_server/pkg/common/constant"
)

const (
	FairLaunchChainStatusNotCreated                = "not_created"
	FairLaunchChainStatusPending                   = "pending"
	FairLaunchChainStatusActive                    = "active"
	FairLaunchChainStatusSuccessfulPendingRelease  = "successful_pending_release"
	FairLaunchChainStatusSuccessful                = "successful"
	FairLaunchChainStatusFailed                    = "failed"
	FairLaunchChainStatusCancelled                 = "cancelled"
	FairLaunchDisplayEndedSuccessPendingSettlement = "ended_success_pending_settlement"
	FairLaunchDisplayEndedFailedPendingSettlement  = "ended_failed_pending_settlement"

	FinancialStateAvailable         = "available"
	FinancialStateNotApplicable     = "not_applicable"
	FinancialStatePendingSettlement = "pending_settlement"
)

// FairLaunchFinancialValue distinguishes absent semantics from a real raw zero.
type FairLaunchFinancialValue struct {
	State          string  `json:"state"`
	Raw            *string `json:"raw"`
	NumeratorRaw   *string `json:"numeratorRaw,omitempty"`
	DenominatorRaw *string `json:"denominatorRaw,omitempty"`
}

// FairLaunchReadSummary is a pure, non-authoritative read projection. It does
// not enable an action and must not be used in place of a fresh chain preflight.
type FairLaunchReadSummary struct {
	FundingMode        string                   `json:"fundingMode"`
	ContractVersion    string                   `json:"contractVersion"`
	DisplayStatus      string                   `json:"displayStatus"`
	SoftCapReached     bool                     `json:"softCapReached"`
	SoftCapProgressBps string                   `json:"softCapProgressBps"`
	HardCap            FairLaunchFinancialValue `json:"hardCap"`
	EstimatedPrice     FairLaunchFinancialValue `json:"estimatedTokenPrice"`
	FinalPrice         FairLaunchFinancialValue `json:"finalTokenPrice"`
}

func parseRawUint(name, raw string) (*big.Int, error) {
	if raw == "" || strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") || (len(raw) > 1 && raw[0] == '0') {
		return nil, fmt.Errorf("%s must be a canonical unsigned decimal string", name)
	}
	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			return nil, fmt.Errorf("%s must be a canonical unsigned decimal string", name)
		}
	}
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return nil, fmt.Errorf("%s must be a canonical unsigned decimal string", name)
	}
	return value, nil
}

// FairLaunchAllocationRaw returns floor(contribution * saleTokenAmount /
// totalRaised) using arbitrary-precision integer arithmetic.
func FairLaunchAllocationRaw(contributionRaw, saleTokenAmountRaw, totalRaisedRaw string) (string, error) {
	contribution, err := parseRawUint("contributionRaw", contributionRaw)
	if err != nil {
		return "", err
	}
	saleTokenAmount, err := parseRawUint("saleTokenAmountRaw", saleTokenAmountRaw)
	if err != nil {
		return "", err
	}
	totalRaised, err := parseRawUint("totalRaisedRaw", totalRaisedRaw)
	if err != nil {
		return "", err
	}
	if saleTokenAmount.Sign() <= 0 || totalRaised.Sign() <= 0 {
		return "", fmt.Errorf("saleTokenAmountRaw and totalRaisedRaw must be greater than zero")
	}
	return new(big.Int).Quo(new(big.Int).Mul(contribution, saleTokenAmount), totalRaised).String(), nil
}

// FairLaunchSoftCapProgressBps returns an uncapped progress value so a result
// above 10000 remains visible. Reaching the soft cap is not settlement.
func FairLaunchSoftCapProgressBps(totalRaisedRaw, softCapRaw string) (string, bool, error) {
	totalRaised, err := parseRawUint("totalRaisedRaw", totalRaisedRaw)
	if err != nil {
		return "", false, err
	}
	softCap, err := parseRawUint("softCapRaw", softCapRaw)
	if err != nil {
		return "", false, err
	}
	if softCap.Sign() <= 0 {
		return "", false, fmt.Errorf("softCapRaw must be greater than zero")
	}
	progress := new(big.Int).Quo(new(big.Int).Mul(totalRaised, big.NewInt(10_000)), softCap)
	return progress.String(), totalRaised.Cmp(softCap) >= 0, nil
}

// FairLaunchDisplayStatus derives only the pre-settlement display states. A
// soft cap reached Active project stays Active until end time and remains an
// ended_*_pending_settlement projection until the authoritative chain status
// changes.
func FairLaunchDisplayStatus(chainStatus string, nowUnix, endTimeUnix int64, softCapReached bool) (string, error) {
	switch chainStatus {
	case FairLaunchChainStatusNotCreated,
		FairLaunchChainStatusPending,
		FairLaunchChainStatusSuccessfulPendingRelease,
		FairLaunchChainStatusSuccessful,
		FairLaunchChainStatusFailed,
		FairLaunchChainStatusCancelled:
		return chainStatus, nil
	case FairLaunchChainStatusActive:
		if nowUnix < endTimeUnix {
			return FairLaunchChainStatusActive, nil
		}
		if softCapReached {
			return FairLaunchDisplayEndedSuccessPendingSettlement, nil
		}
		return FairLaunchDisplayEndedFailedPendingSettlement, nil
	default:
		return "", fmt.Errorf("unsupported fair launch chain status %q", chainStatus)
	}
}

// BuildFairLaunchReadSummary constructs read-only semantics without float math
// and without a hard-cap sentinel masquerading as a real financial value.
func BuildFairLaunchReadSummary(chainStatus string, nowUnix, endTimeUnix int64, totalRaisedRaw, softCapRaw, saleTokenAmountRaw string) (FairLaunchReadSummary, error) {
	progress, reached, err := FairLaunchSoftCapProgressBps(totalRaisedRaw, softCapRaw)
	if err != nil {
		return FairLaunchReadSummary{}, err
	}
	displayStatus, err := FairLaunchDisplayStatus(chainStatus, nowUnix, endTimeUnix, reached)
	if err != nil {
		return FairLaunchReadSummary{}, err
	}
	saleTokenAmount, err := parseRawUint("saleTokenAmountRaw", saleTokenAmountRaw)
	if err != nil {
		return FairLaunchReadSummary{}, err
	}
	if saleTokenAmount.Sign() <= 0 {
		return FairLaunchReadSummary{}, fmt.Errorf("saleTokenAmountRaw must be greater than zero")
	}

	total := totalRaisedRaw
	sale := saleTokenAmountRaw
	finalPrice := FairLaunchFinancialValue{State: FinancialStatePendingSettlement}
	switch chainStatus {
	case FairLaunchChainStatusSuccessfulPendingRelease, FairLaunchChainStatusSuccessful:
		finalPrice = FairLaunchFinancialValue{State: FinancialStateAvailable, NumeratorRaw: &total, DenominatorRaw: &sale}
	case FairLaunchChainStatusFailed, FairLaunchChainStatusCancelled:
		finalPrice = FairLaunchFinancialValue{State: FinancialStateNotApplicable}
	}
	return FairLaunchReadSummary{
		FundingMode:        constant.FundingModeFairLaunch,
		ContractVersion:    constant.ContractVersionFairLaunchV1,
		DisplayStatus:      displayStatus,
		SoftCapReached:     reached,
		SoftCapProgressBps: progress,
		HardCap:            FairLaunchFinancialValue{State: FinancialStateNotApplicable},
		EstimatedPrice:     FairLaunchFinancialValue{State: FinancialStateAvailable, NumeratorRaw: &total, DenominatorRaw: &sale},
		FinalPrice:         finalPrice,
	}, nil
}
