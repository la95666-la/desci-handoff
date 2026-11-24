package chain

import (
	"github.com/spf13/viper"
	"testing"
)

func TestV3OnlyStartupDoesNotConnectLegacyRPCOrRequireSigningKey(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("application.jobMode", "ido-v3")
	viper.Set("chain.project.v1.enabled", false)
	viper.Set("chain.project.chainID", 56)
	viper.Set("idoV3Approved.enabled", true)
	// Intentionally invalid V2 RPC and no jobPrivate: V3-only should never use them.
	viper.Set("chain.project.url", "http://127.0.0.1:1")
	viper.Set("chain.project.v3.enabled", false)
	viper.Set("chain.project.v3.chainId", 56)
	viper.Set("chain.project.v3.startBlock", 128805696)
	viper.Set("chain.project.v3.contractAddress", "0x0d4040875742975269c55e16c8884d3ee9510b8e")
	viper.Set("chain.project.v3.implementationAddress", "0x887fcc6e54fc8229919166ad90b16c19b4992560")
	viper.Set("chain.project.v3.governorAddress", "0xc46659e67543aaaec553f7f3fc8f004b7dd286ef")
	viper.Set("chain.project.v3.proxyCodeHash", "0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8")
	viper.Set("chain.project.v3.implementationCodeHash", "0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590")
	if e := NewClient(); e != nil {
		t.Fatal(e)
	}
	if GetClient() != nil || V2Enabled() || V1Enabled() || !V3ShadowConfigured() {
		t.Fatal("legacy runtime initialized")
	}
	viper.Set("chain.project.private", "unwanted-key")
	if NewClient() == nil {
		t.Fatal("Core signing secret accepted")
	}
}
