package constant

import "testing"

func TestFundingModeForContractVersion(t *testing.T) {
	tests := []struct {
		name            string
		contractVersion string
		want            string
		wantErr         bool
	}{
		{name: "capped presale", contractVersion: ContractVersionV2Stablecoin, want: FundingModeCappedPresale},
		{name: "V3 capped presale identity only", contractVersion: ContractVersionIDOV3Stablecoin, want: FundingModeCappedPresale},
		{name: "fair launch", contractVersion: ContractVersionFairLaunchV1, want: FundingModeFairLaunch},
		{name: "legacy is not inferred", contractVersion: ContractVersionV1Legacy, wantErr: true},
		{name: "unverified legacy is not inferred", contractVersion: ContractVersionV1LegacyUnverified, wantErr: true},
		{name: "unassigned is not inferred", contractVersion: ContractVersionUnassigned, wantErr: true},
		{name: "unknown fails closed", contractVersion: "future_version", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := FundingModeForContractVersion(test.contractVersion)
			if test.wantErr {
				if err == nil || got != "" {
					t.Fatalf("FundingModeForContractVersion(%q)=(%q,%v), want empty result and error", test.contractVersion, got, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("FundingModeForContractVersion(%q)=(%q,%v), want (%q,nil)", test.contractVersion, got, err, test.want)
			}
		})
	}
}

func TestValidateFundingModeContractVersion(t *testing.T) {
	valid := [][2]string{
		{FundingModeCappedPresale, ContractVersionV2Stablecoin},
		{FundingModeCappedPresale, ContractVersionIDOV3Stablecoin},
		{FundingModeFairLaunch, ContractVersionFairLaunchV1},
	}
	for _, pair := range valid {
		if err := ValidateFundingModeContractVersion(pair[0], pair[1]); err != nil {
			t.Fatalf("valid pair %q/%q rejected: %v", pair[0], pair[1], err)
		}
	}

	invalid := [][2]string{
		{FundingModeFairLaunch, ContractVersionV2Stablecoin},
		{FundingModeFairLaunch, ContractVersionIDOV3Stablecoin},
		{FundingModeCappedPresale, ContractVersionFairLaunchV1},
		{FundingModeCappedPresale, "future_version"},
		{"future_mode", ContractVersionFairLaunchV1},
	}
	for _, pair := range invalid {
		if err := ValidateFundingModeContractVersion(pair[0], pair[1]); err == nil {
			t.Fatalf("invalid pair %q/%q accepted", pair[0], pair[1])
		}
	}
}
