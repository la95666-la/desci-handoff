package project

import (
	"encoding/json/v2"
	"fmt"
	"strings"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

// FundingReadProjection contains only read-model values that can be derived
// exactly from the existing project row. It must never be used to authorize a
// transaction or replace a fresh chain preflight.
type FundingReadProjection struct {
	FundingMode        string
	ChainStatus        string
	SaleTokenAmount    string
	SaleTokenAmountRaw string
	SoftCap            string
	SoftCapRaw         string
}

type fairLaunchOnchainReadMetadata struct {
	FairLaunch *struct {
		SaleTokenAmountRaw string `json:"saleTokenAmountRaw"`
		SoftCapRaw         string `json:"softCapRaw,omitempty"`
	} `json:"fairLaunch"`
}

// CanonicalRawUnits formats an unsigned raw amount as a canonical decimal
// string without using floating point. Trailing fractional zeroes are removed.
func CanonicalRawUnits(raw string, decimals int) (string, error) {
	if decimals < 0 || decimals > 255 {
		return "", fmt.Errorf("decimals out of range")
	}
	if _, err := parseRawUint("raw", raw); err != nil {
		return "", err
	}
	if decimals == 0 {
		return raw, nil
	}
	padded := raw
	if len(padded) <= decimals {
		padded = strings.Repeat("0", decimals+1-len(padded)) + padded
	}
	split := len(padded) - decimals
	whole := padded[:split]
	fraction := strings.TrimRight(padded[split:], "0")
	if fraction == "" {
		return whole, nil
	}
	return whole + "." + fraction, nil
}

// ProjectFundingReadProjection derives the exact funding identity and the
// minimum financial fields needed by Phase-1 read-only clients. Fair Launch's
// sale token amount has no production column yet, so the only accepted source
// is an explicitly namespaced onchain_json read-model object. The optional
// duplicated softCapRaw must match the existing canonical projects column.
func ProjectFundingReadProjection(info entity.Project) (FundingReadProjection, error) {
	mode, err := constant.FundingModeForContractVersion(info.ContractVersion)
	if err != nil {
		switch info.ContractVersion {
		case constant.ContractVersionUnassigned, constant.ContractVersionV1Legacy, constant.ContractVersionV1LegacyUnverified:
			// These existing records predate funding modes. Do not infer a formula,
			// but keep their established read paths available.
			return FundingReadProjection{}, nil
		default:
			return FundingReadProjection{}, err
		}
	}
	projection := FundingReadProjection{FundingMode: mode}
	// V3 read identity cannot be inferred from contract_version alone. A
	// mismatched persisted pair must not be advertised as an eligible V3 route.
	if info.ContractVersion == constant.ContractVersionIDOV3Stablecoin &&
		!constant.IsExactIDOV3ProjectIdentity(info.FundingMode, info.ContractVersion) {
		return FundingReadProjection{}, fmt.Errorf("IDO V3 funding identity mismatch")
	}
	if mode != constant.FundingModeFairLaunch {
		return projection, nil
	}

	projection.ChainStatus, err = FairLaunchChainStatusForProject(info)
	if err != nil {
		return FundingReadProjection{}, err
	}
	softCap, err := parseRawUint("softCapRaw", info.SoftCapRaw)
	if err != nil || softCap.Sign() <= 0 {
		return FundingReadProjection{}, fmt.Errorf("fair launch softCapRaw must be a canonical positive decimal string")
	}
	projection.SoftCapRaw = info.SoftCapRaw
	projection.SoftCap, err = CanonicalRawUnits(info.SoftCapRaw, info.PaymentTokenDecimals)
	if err != nil {
		return FundingReadProjection{}, err
	}

	var metadata fairLaunchOnchainReadMetadata
	if info.OnchainJSON == "" || json.Unmarshal([]byte(info.OnchainJSON), &metadata) != nil || metadata.FairLaunch == nil {
		return FundingReadProjection{}, fmt.Errorf("fair launch onchain_json metadata is unavailable")
	}
	saleTokenAmount, err := parseRawUint("saleTokenAmountRaw", metadata.FairLaunch.SaleTokenAmountRaw)
	if err != nil || saleTokenAmount.Sign() <= 0 {
		return FundingReadProjection{}, fmt.Errorf("fair launch saleTokenAmountRaw must be a canonical positive decimal string")
	}
	if metadata.FairLaunch.SoftCapRaw != "" && metadata.FairLaunch.SoftCapRaw != info.SoftCapRaw {
		return FundingReadProjection{}, fmt.Errorf("fair launch softCapRaw metadata mismatch")
	}
	projection.SaleTokenAmountRaw = metadata.FairLaunch.SaleTokenAmountRaw
	projection.SaleTokenAmount, err = CanonicalRawUnits(metadata.FairLaunch.SaleTokenAmountRaw, info.TokenDecimals)
	if err != nil {
		return FundingReadProjection{}, err
	}
	return projection, nil
}

// FairLaunchChainStatusForProject maps only lifecycle states whose meaning is
// unambiguous in the existing row. Unknown combinations fail closed.
func FairLaunchChainStatusForProject(info entity.Project) (string, error) {
	if info.ContractVersion != constant.ContractVersionFairLaunchV1 {
		return "", fmt.Errorf("not a fair launch project")
	}
	if !info.OnchainLocked {
		switch info.Status {
		case constant.ProjectStatusSetup, constant.ProjectStatusSet,
			constant.ProjectStatusPreLaunch, constant.ProjectStatusWarmup:
			return FairLaunchChainStatusNotCreated, nil
		default:
			return "", fmt.Errorf("unlocked fair launch project has unsupported status %q", info.Status)
		}
	}
	switch info.Status {
	case constant.ProjectStatusPending:
		return FairLaunchChainStatusPending, nil
	case constant.ProjectStatusActive:
		return FairLaunchChainStatusActive, nil
	case constant.ProjectStatusSuccessfulPendingRelease:
		return FairLaunchChainStatusSuccessfulPendingRelease, nil
	case constant.ProjectStatusSuccessful:
		return FairLaunchChainStatusSuccessful, nil
	case constant.ProjectStatusFailed:
		return FairLaunchChainStatusFailed, nil
	case constant.ProjectStatusCanceled:
		return FairLaunchChainStatusCancelled, nil
	default:
		return "", fmt.Errorf("locked fair launch project has unsupported status %q", info.Status)
	}
}
