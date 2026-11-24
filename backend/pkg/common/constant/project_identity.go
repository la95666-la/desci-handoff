package constant

import "fmt"

// IsExactV2ProjectIdentity admits the capped-presale V2 product only when both
// independently persisted identity fields match. Callers must not infer one
// field from the other at financial, RPC, mutation, or event boundaries.
func IsExactV2ProjectIdentity(fundingMode, contractVersion string) bool {
	return fundingMode == FundingModeCappedPresale && contractVersion == ContractVersionV2Stablecoin
}

// RequireExactV2ProjectIdentity returns a stable fail-closed error for product
// boundaries that require the capped-presale V2 formula and ABI.
func RequireExactV2ProjectIdentity(fundingMode, contractVersion string) error {
	if !IsExactV2ProjectIdentity(fundingMode, contractVersion) {
		return fmt.Errorf("project funding identity must be %s/%s, got %q/%q", FundingModeCappedPresale, ContractVersionV2Stablecoin, fundingMode, contractVersion)
	}
	return nil
}

// V3 uses a new proxy and ABI. Do not widen IsExactV2ProjectIdentity: its
// existing callers are V2-specific financial, event, and signer boundaries.
func IsExactIDOV3ProjectIdentity(fundingMode, contractVersion string) bool {
	return fundingMode == FundingModeCappedPresale && contractVersion == ContractVersionIDOV3Stablecoin
}

func RequireExactIDOV3ProjectIdentity(fundingMode, contractVersion string) error {
	if !IsExactIDOV3ProjectIdentity(fundingMode, contractVersion) {
		return fmt.Errorf("project funding identity must be %s/%s, got %q/%q", FundingModeCappedPresale, ContractVersionIDOV3Stablecoin, fundingMode, contractVersion)
	}
	return nil
}
