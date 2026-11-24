package chain

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
)

func TestV3JobShadowSourceCannotRegisterOrReuseV2(t *testing.T) {
	v1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	v2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	newConfig := func() *viper.Viper {
		config := viper.New()
		config.Set("chain.project.v3.enabled", false)
		config.Set("chain.project.v3.chainId", "31337")
		config.Set("chain.project.v3.contractAddress", "0x3333333333333333333333333333333333333333")
		config.Set("chain.project.v3.implementationAddress", "0x4444444444444444444444444444444444444444")
		config.Set("chain.project.v3.governorAddress", "0x5555555555555555555555555555555555555555")
		config.Set("chain.project.v3.proxyCodeHash", "0x"+strings.Repeat("a", 64))
		config.Set("chain.project.v3.implementationCodeHash", "0x"+strings.Repeat("b", 64))
		config.Set("chain.project.v3.startBlock", "900")
		return config
	}
	if source, err := readV3ShadowSource(viper.New(), 31337, v1, v2); err != nil || source != nil {
		t.Fatalf("absent V3 shadow source changed existing Job: %+v %v", source, err)
	}
	unknownOnly := viper.New()
	unknownOnly.Set("chain.project.v3.skipRuntimeVerify", true)
	if source, err := readV3ShadowSource(unknownOnly, 31337, v1, v2); err == nil || source != nil {
		t.Fatalf("unknown-only V3 source silently ignored: %+v %v", source, err)
	}
	rootOnly := viper.New()
	rootOnly.Set("chain.project.v3", "disabled")
	if source, err := readV3ShadowSource(rootOnly, 31337, v1, v2); err == nil || source != nil {
		t.Fatalf("scalar V3 source silently ignored: %+v %v", source, err)
	}
	config := newConfig()
	source, err := readV3ShadowSource(config, 31337, v1, v2)
	if err != nil || source == nil || source.StartBlock != 900 || source.ChainID != 31337 {
		t.Fatalf("complete disabled shadow identity rejected: %+v %v", source, err)
	}
	for name, edit := range map[string]func(*viper.Viper){
		"enabled":          func(c *viper.Viper) { c.Set("chain.project.v3.enabled", true) },
		"missing enabled":  func(c *viper.Viper) { c.Set("chain.project.v3.enabled", "") },
		"V2 address reuse": func(c *viper.Viper) { c.Set("chain.project.v3.contractAddress", v2.Hex()) },
		"V1 address reuse": func(c *viper.Viper) { c.Set("chain.project.v3.contractAddress", v1.Hex()) },
		"proxy implementation overlap": func(c *viper.Viper) {
			c.Set("chain.project.v3.implementationAddress", c.GetString("chain.project.v3.contractAddress"))
		},
		"different chain":     func(c *viper.Viper) { c.Set("chain.project.v3.chainId", "97") },
		"missing chain":       func(c *viper.Viper) { c.Set("chain.project.v3.chainId", "") },
		"zero start":          func(c *viper.Viper) { c.Set("chain.project.v3.startBlock", "0") },
		"negative start":      func(c *viper.Viper) { c.Set("chain.project.v3.startBlock", "-1") },
		"overflow start":      func(c *viper.Viper) { c.Set("chain.project.v3.startBlock", "18446744073709551616") },
		"zero code hash":      func(c *viper.Viper) { c.Set("chain.project.v3.proxyCodeHash", "0x"+strings.Repeat("0", 64)) },
		"uppercase code hash": func(c *viper.Viper) { c.Set("chain.project.v3.proxyCodeHash", "0x"+strings.Repeat("A", 64)) },
		"unknown key":         func(c *viper.Viper) { c.Set("chain.project.v3.skipRuntimeVerify", true) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := newConfig()
			edit(bad)
			if source, err := readV3ShadowSource(bad, 31337, v1, v2); err == nil || source != nil {
				t.Fatalf("unsafe V3 Job identity accepted: %+v %v", source, err)
			}
		})
	}
}
