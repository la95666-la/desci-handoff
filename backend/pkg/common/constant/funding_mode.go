package constant

import "fmt"

const (
	FundingModeCappedPresale = "capped_presale"
	FundingModeFairLaunch    = "fair_launch"

	ContractVersionFairLaunchV1 = "fair_launch_v1"
)

// FundingModeForContractVersion returns a funding mode only for an explicitly
// frozen mode/version pair. Legacy, unassigned, and unknown versions are not
// inferred because treating an unknown contract as a known funding formula is
// unsafe.
func FundingModeForContractVersion(contractVersion string) (string, error) {
	switch contractVersion {
	case ContractVersionV2Stablecoin, ContractVersionIDOV3Stablecoin:
		return FundingModeCappedPresale, nil
	case ContractVersionFairLaunchV1:
		return FundingModeFairLaunch, nil
	default:
		return "", fmt.Errorf("unsupported funding contract version %q", contractVersion)
	}
}

// ValidateFundingModeContractVersion rejects unknown values and mismatched
// pairs instead of falling back to a legacy contract or formula.
func ValidateFundingModeContractVersion(fundingMode, contractVersion string) error {
	derived, err := FundingModeForContractVersion(contractVersion)
	if err != nil {
		return err
	}
	if fundingMode != derived {
		return fmt.Errorf("funding mode %q does not match contract version %q", fundingMode, contractVersion)
	}
	return nil
}
