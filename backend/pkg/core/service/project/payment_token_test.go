package project

import (
	"testing"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func configureTrustedPaymentTokensForTest() {
	viper.Set("chain.project.chainID", 97)
	viper.Set("chain.project.v2.contractAddress", "0x1111111111111111111111111111111111111111")
	viper.Set("chain.project.v2.implementationAddress", "0x5555555555555555555555555555555555555555")
	viper.Set("chain.project.v2.implementationCodeHash", "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	viper.Set("chain.project.v2.feeReceiver", "0x8888888888888888888888888888888888888888")
	viper.Set("chain.project.v2.successFeeRate", 200)
	viper.Set("chain.project.v2.paymentTokens.USDT.address", "0x2222222222222222222222222222222222222222")
	viper.Set("chain.project.v2.paymentTokens.USDT.decimals", 6)
	viper.Set("chain.project.v2.paymentTokens.USDC.address", "0x3333333333333333333333333333333333333333")
	viper.Set("chain.project.v2.paymentTokens.USDC.decimals", 18)
}

func TestValidateProjectV2Identity(t *testing.T) {
	trusted := trustedPaymentToken{
		Symbol: "USDT", Address: "0x2222222222222222222222222222222222222222", Decimals: 6,
		ChainID: 97, Contract: "0x1111111111111111111111111111111111111111",
		ImplementationAddress: "0x5555555555555555555555555555555555555555", ImplementationCodeHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FeeReceiver: "0x8888888888888888888888888888888888888888", SuccessFeeRate: 200,
	}
	project := entity.Project{
		OwnerAddress: "0x7777777777777777777777777777777777777777",
		TokenAddress: "0x4444444444444444444444444444444444444444", TokenDecimals: 18,
		PaymentToken: "USDT", PaymentTokenAddress: trusted.Address, PaymentTokenDecimals: 6,
		ChainID: 97, ContractAddress: trusted.Contract, ContractVersion: constant.ContractVersionV2Stablecoin, FundingMode: constant.FundingModeCappedPresale,
	}
	if err := validateProjectV2Identity(project, trusted); err != nil {
		t.Fatalf("valid V2 identity rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*entity.Project)
	}{
		{name: "locked", mutate: func(p *entity.Project) { p.OnchainLocked = true }},
		{name: "wrong chain", mutate: func(p *entity.Project) { p.ChainID = 56 }},
		{name: "wrong contract", mutate: func(p *entity.Project) { p.ContractAddress = "0x5555555555555555555555555555555555555555" }},
		{name: "wrong token address", mutate: func(p *entity.Project) { p.PaymentTokenAddress = "0x6666666666666666666666666666666666666666" }},
		{name: "wrong decimals", mutate: func(p *entity.Project) { p.PaymentTokenDecimals = 18 }},
		{name: "legacy version", mutate: func(p *entity.Project) { p.ContractVersion = constant.ContractVersionV1Legacy }},
		{name: "fair mode with V2 version", mutate: func(p *entity.Project) { p.FundingMode = constant.FundingModeFairLaunch }},
		{name: "capped mode with Fair Launch version", mutate: func(p *entity.Project) { p.ContractVersion = constant.ContractVersionFairLaunchV1 }},
		{name: "same project and payment token", mutate: func(p *entity.Project) { p.TokenAddress = p.PaymentTokenAddress }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := project
			test.mutate(&candidate)
			if err := validateProjectV2Identity(candidate, trusted); err == nil {
				t.Fatal("invalid V2 identity accepted")
			}
		})
	}
}

func TestResolveTrustedPaymentToken(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	configureTrustedPaymentTokensForTest()

	info, err := resolveTrustedPaymentToken(" usdt ")
	if err != nil {
		t.Fatalf("resolve trusted USDT: %v", err)
	}
	if info.Symbol != "USDT" || info.Decimals != 6 || info.ChainID != 97 || info.Address != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("unexpected token config: %+v", info)
	}
}

func TestResolveTrustedPaymentTokenFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		setup  func()
		symbol string
	}{
		{name: "JCC rejected", setup: configureTrustedPaymentTokensForTest, symbol: "JCC"},
		{name: "arbitrary symbol rejected", setup: configureTrustedPaymentTokensForTest, symbol: "DAI"},
		{name: "missing address", setup: func() {
			configureTrustedPaymentTokensForTest()
			viper.Set("chain.project.v2.paymentTokens.USDT.address", "")
		}, symbol: "USDT"},
		{name: "missing other token address", setup: func() {
			configureTrustedPaymentTokensForTest()
			viper.Set("chain.project.v2.paymentTokens.USDC.address", "")
		}, symbol: "USDT"},
		{name: "wrong decimals", setup: func() {
			configureTrustedPaymentTokensForTest()
			viper.Set("chain.project.v2.paymentTokens.USDC.decimals", 0)
		}, symbol: "USDC"},
		{name: "missing V2 contract", setup: func() {
			configureTrustedPaymentTokensForTest()
			viper.Set("chain.project.v2.contractAddress", "")
		}, symbol: "USDT"},
		{name: "same address for both symbols", setup: func() {
			configureTrustedPaymentTokensForTest()
			viper.Set("chain.project.v2.paymentTokens.USDC.address", "0x2222222222222222222222222222222222222222")
		}, symbol: "USDT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			test.setup()
			if _, err := resolveTrustedPaymentToken(test.symbol); err == nil {
				t.Fatal("invalid payment token configuration accepted")
			}
		})
	}
}
