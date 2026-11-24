package project

import (
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestFormatListDataExposesV2Identity(t *testing.T) {
	info := entity.Project{
		ProjectID:            42,
		PaymentToken:         constant.PaymentTokenUSDT,
		PaymentTokenAddress:  "0x1111111111111111111111111111111111111111",
		PaymentTokenDecimals: 6,
		ChainID:              97,
		ContractAddress:      "0x2222222222222222222222222222222222222222",
		ContractVersion:      constant.ContractVersionV2Stablecoin,
		OnchainLocked:        true,
		FeeReceiver:          "0x3333333333333333333333333333333333333333",
		SuccessFeeRate:       200,
		PlatformFeeWithdrawn: 1,
		TokenDecimals:        18,
		TokenPriceRaw:        "1000000",
		HardCapRaw:           "1000000000",
		SoftCapRaw:           "500000000",
		MaxPerUserRaw:        "100000000",
		TotalRaisedRaw:       "250000000",
	}
	item := BriefInfo{}
	applyBriefV2Identity(&item, info)
	if item.PaymentTokenAddress != info.PaymentTokenAddress || item.PaymentTokenDecimals != 6 || item.ChainID != 97 || item.ContractAddress != info.ContractAddress || item.ContractVersion != constant.ContractVersionV2Stablecoin || !item.OnchainLocked || item.FeeReceiver != info.FeeReceiver || item.SuccessFeeRate != 200 || !item.PlatformFeeWithdrawn {
		t.Fatalf("V2 identity was not exposed: %+v", item)
	}
	if item.FundingMode != constant.FundingModeCappedPresale {
		t.Fatalf("funding mode=%q want=%q", item.FundingMode, constant.FundingModeCappedPresale)
	}
	if item.TokenDecimals != 18 || item.TokenPriceRaw != info.TokenPriceRaw || item.TotalRaisedRaw != info.TotalRaisedRaw {
		t.Fatalf("V2 raw amounts were not exposed: %+v", item)
	}
	detail := DetailResponse{}
	applyDetailV2Identity(&detail, info)
	if detail.PaymentTokenAddress != info.PaymentTokenAddress || detail.ContractVersion != info.ContractVersion || detail.FundingMode != constant.FundingModeCappedPresale || !detail.OnchainLocked || detail.HardCapRaw != info.HardCapRaw {
		t.Fatalf("detail V2 identity was not exposed: %+v", detail)
	}
}

func TestReadOnlyProjectDTOCanLabelExplicitV3WithoutRewritingV2Identity(t *testing.T) {
	info := entity.Project{
		ProjectID: 43, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		FundingMode: constant.FundingModeCappedPresale,
		ChainID:     97, ContractAddress: "0x3333333333333333333333333333333333333333",
		PaymentTokenAddress: "0x4444444444444444444444444444444444444444",
	}
	brief := BriefInfo{}
	applyBriefV2Identity(&brief, info)
	if brief.ContractVersion != constant.ContractVersionIDOV3Stablecoin || brief.FundingMode != constant.FundingModeCappedPresale || brief.ContractAddress != info.ContractAddress {
		t.Fatalf("V3 read DTO lost exact identity: %+v", brief)
	}
	if constant.IsExactV2ProjectIdentity(brief.FundingMode, brief.ContractVersion) {
		t.Fatal("V3 read DTO must not be admitted to V2 financial operations")
	}
	if !constant.IsExactIDOV3ProjectIdentity(brief.FundingMode, brief.ContractVersion) {
		t.Fatal("V3 read DTO did not retain separate V3 identity")
	}
}

func TestV3PublicDetailDoesNotExposeWarmupSelfRegistrationCount(t *testing.T) {
	for _, mode := range []string{constant.FundingModeCappedPresale, constant.FundingModeFairLaunch, ""} {
		if exposeWarmupRegistrationCount(entity.Project{ContractVersion: constant.ContractVersionIDOV3Stablecoin, FundingMode: mode}) {
			t.Fatalf("V3 row with mode %q exposed self-registration as membership", mode)
		}
	}
	if !exposeWarmupRegistrationCount(entity.Project{ContractVersion: constant.ContractVersionV2Stablecoin}) {
		t.Fatal("V2 Warmup registration count was hidden")
	}
}

func TestPublicDTOFundingProjectionIsExactAndUnknownFailsClosed(t *testing.T) {
	fair := fairLaunchProjectionFixture()
	brief := BriefInfo{}
	if err := applyBriefFundingProjection(&brief, fair); err != nil {
		t.Fatal(err)
	}
	if brief.FundingMode != constant.FundingModeFairLaunch || brief.ChainStatus != FairLaunchChainStatusNotCreated || brief.SaleTokenAmountRaw != "1000000000000000000000001" || brief.SoftCapRaw != "10000001" {
		t.Fatalf("fair brief projection=%+v", brief)
	}
	detail := DetailResponse{}
	if err := applyDetailFundingProjection(&detail, fair); err != nil {
		t.Fatal(err)
	}
	if detail.FundingMode != constant.FundingModeFairLaunch || detail.SaleTokenAmount != "1000000.000000000000000001" || detail.SoftCap.String() != "10.000001" {
		t.Fatalf("fair detail projection=%+v", detail)
	}

	unknown := entity.Project{ContractVersion: "future_version"}
	if err := applyBriefFundingProjection(&BriefInfo{}, unknown); err == nil {
		t.Fatal("unknown contract version was accepted")
	}
}
