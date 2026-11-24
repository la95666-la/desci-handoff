package constant

import "testing"

func TestExactV2ProjectIdentityRequiresBothFields(t *testing.T) {
	tests := []struct {
		name, mode, version string
		want                bool
	}{
		{"exact pair", FundingModeCappedPresale, ContractVersionV2Stablecoin, true},
		{"fair mode with V2 version", FundingModeFairLaunch, ContractVersionV2Stablecoin, false},
		{"capped mode with Fair Launch version", FundingModeCappedPresale, ContractVersionFairLaunchV1, false},
		{"empty mode", "", ContractVersionV2Stablecoin, false},
		{"V3 must not be routed as V2", FundingModeCappedPresale, ContractVersionIDOV3Stablecoin, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsExactV2ProjectIdentity(test.mode, test.version); got != test.want {
				t.Fatalf("IsExactV2ProjectIdentity(%q,%q)=%t want %t", test.mode, test.version, got, test.want)
			}
			if err := RequireExactV2ProjectIdentity(test.mode, test.version); (err == nil) != test.want {
				t.Fatalf("RequireExactV2ProjectIdentity(%q,%q) err=%v", test.mode, test.version, err)
			}
		})
	}
}

func TestExactIDOV3ProjectIdentityNeverFallsBackToV2(t *testing.T) {
	tests := []struct {
		mode, version string
		want          bool
	}{
		{FundingModeCappedPresale, ContractVersionIDOV3Stablecoin, true},
		{FundingModeCappedPresale, ContractVersionV2Stablecoin, false},
		{FundingModeFairLaunch, ContractVersionIDOV3Stablecoin, false},
		{"", ContractVersionIDOV3Stablecoin, false},
	}
	for _, test := range tests {
		if got := IsExactIDOV3ProjectIdentity(test.mode, test.version); got != test.want {
			t.Fatalf("V3 route mode=%q version=%q got=%t want=%t", test.mode, test.version, got, test.want)
		}
		if err := RequireExactIDOV3ProjectIdentity(test.mode, test.version); (err == nil) != test.want {
			t.Fatalf("V3 route mode=%q version=%q err=%v", test.mode, test.version, err)
		}
	}
}
