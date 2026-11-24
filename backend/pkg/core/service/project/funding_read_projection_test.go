package project

import (
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func fairLaunchProjectionFixture() entity.Project {
	return entity.Project{
		ContractVersion:      constant.ContractVersionFairLaunchV1,
		Status:               constant.ProjectStatusSet,
		TokenDecimals:        18,
		PaymentTokenDecimals: 6,
		SoftCapRaw:           "10000001",
		OnchainJSON:          `{"fairLaunch":{"saleTokenAmountRaw":"1000000000000000000000001","softCapRaw":"10000001"}}`,
	}
}

func TestProjectFundingReadProjectionUsesExactExistingSources(t *testing.T) {
	got, err := ProjectFundingReadProjection(fairLaunchProjectionFixture())
	if err != nil {
		t.Fatal(err)
	}
	if got.FundingMode != constant.FundingModeFairLaunch || got.ChainStatus != FairLaunchChainStatusNotCreated {
		t.Fatalf("wrong identity/status: %+v", got)
	}
	if got.SaleTokenAmountRaw != "1000000000000000000000001" || got.SaleTokenAmount != "1000000.000000000000000001" {
		t.Fatalf("sale token amount lost precision: %+v", got)
	}
	if got.SoftCapRaw != "10000001" || got.SoftCap != "10.000001" {
		t.Fatalf("soft cap lost precision: %+v", got)
	}
}

func TestProjectFundingReadProjectionFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		edit func(*entity.Project)
	}{
		{name: "unknown version", edit: func(p *entity.Project) { p.ContractVersion = "future_v9" }},
		{name: "missing namespace", edit: func(p *entity.Project) { p.OnchainJSON = `{}` }},
		{name: "zero sale amount", edit: func(p *entity.Project) { p.OnchainJSON = `{"fairLaunch":{"saleTokenAmountRaw":"0"}}` }},
		{name: "noncanonical sale amount", edit: func(p *entity.Project) { p.OnchainJSON = `{"fairLaunch":{"saleTokenAmountRaw":"01"}}` }},
		{name: "soft cap mismatch", edit: func(p *entity.Project) { p.OnchainJSON = `{"fairLaunch":{"saleTokenAmountRaw":"1","softCapRaw":"2"}}` }},
		{name: "locked setup mismatch", edit: func(p *entity.Project) { p.OnchainLocked = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := fairLaunchProjectionFixture()
			test.edit(&project)
			if _, err := ProjectFundingReadProjection(project); err == nil {
				t.Fatal("invalid projection accepted")
			}
		})
	}
}

func TestExistingLegacyProjectionRemainsUnclassified(t *testing.T) {
	for _, version := range []string{constant.ContractVersionUnassigned, constant.ContractVersionV1Legacy, constant.ContractVersionV1LegacyUnverified} {
		got, err := ProjectFundingReadProjection(entity.Project{ContractVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		if got.FundingMode != "" {
			t.Fatalf("legacy version %q was inferred as %q", version, got.FundingMode)
		}
	}
}

func TestCappedProjectionDerivesModeWithoutFairLaunchMetadata(t *testing.T) {
	for _, version := range []string{constant.ContractVersionV2Stablecoin, constant.ContractVersionIDOV3Stablecoin} {
		got, err := ProjectFundingReadProjection(entity.Project{FundingMode: constant.FundingModeCappedPresale, ContractVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		if got.FundingMode != constant.FundingModeCappedPresale || got.SaleTokenAmountRaw != "" {
			t.Fatalf("unexpected capped projection for %q: %+v", version, got)
		}
	}
}

func TestV3ProjectionRejectsMissingOrMismatchedPersistedMode(t *testing.T) {
	for _, mode := range []string{"", constant.FundingModeFairLaunch, "unknown"} {
		if _, err := ProjectFundingReadProjection(entity.Project{ContractVersion: constant.ContractVersionIDOV3Stablecoin, FundingMode: mode}); err == nil {
			t.Fatalf("V3 version with mode %q was advertised as a valid route", mode)
		}
	}
}
