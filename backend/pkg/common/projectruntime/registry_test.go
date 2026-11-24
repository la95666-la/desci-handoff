package projectruntime

import (
	"strings"
	"testing"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
)

func resetRuntimeConfig(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func configureValidFairLaunchRuntime() {
	viper.Set("chain.project.chainID", 56)
	viper.Set("chain.project.confirmedConfirmations", 5)
	viper.Set("chain.project.finalizedConfirmations", 20)
	viper.Set(fairLaunchPrefix+".enabled", "true")
	viper.Set(fairLaunchPrefix+".contractAddress", "0x1000000000000000000000000000000000000001")
	viper.Set(fairLaunchPrefix+".implementationAddress", "0x2000000000000000000000000000000000000002")
	viper.Set(fairLaunchPrefix+".implementationCodeHash", "0x"+strings.Repeat("a", 64))
	viper.Set(fairLaunchPrefix+".startBlock", 100)
	viper.Set(fairLaunchPrefix+".operatorAddress", "0x3000000000000000000000000000000000000003")
	viper.Set(fairLaunchPrefix+".platformFeeReceiver", "0x4000000000000000000000000000000000000004")
	viper.Set(fairLaunchPrefix+".platformFeeBps", "250")
	viper.Set(fairLaunchPrefix+".releasePolicyDigest", "0x"+strings.Repeat("b", 64))
	viper.Set(fairLaunchPrefix+".releasePolicyVersion", "2")
	viper.Set(fairLaunchPrefix+".paymentTokenSymbols", []string{"USDT", "USDC"})
	viper.Set(fairLaunchPrefix+".paymentTokens.USDT.address", "0x5000000000000000000000000000000000000005")
	viper.Set(fairLaunchPrefix+".paymentTokens.USDT.decimals", 18)
	viper.Set(fairLaunchPrefix+".paymentTokens.USDT.policyDigest", "0x"+strings.Repeat("c", 64))
	viper.Set(fairLaunchPrefix+".paymentTokens.USDT.policyVersion", 7)
	viper.Set(fairLaunchPrefix+".paymentTokens.USDC.address", "0x6000000000000000000000000000000000000006")
	viper.Set(fairLaunchPrefix+".paymentTokens.USDC.decimals", 18)
	viper.Set(fairLaunchPrefix+".paymentTokens.USDC.policyDigest", "0x"+strings.Repeat("d", 64))
	viper.Set(fairLaunchPrefix+".paymentTokens.USDC.policyVersion", 8)
}

func TestFairLaunchV1DefaultsToInertWithoutAddressesOrSigner(t *testing.T) {
	resetRuntimeConfig(t)
	runtime, err := FairLaunchV1()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Enabled || runtime.Version != constant.ContractVersionFairLaunchV1 || runtime.FundingMode != constant.FundingModeFairLaunch {
		t.Fatalf("unexpected inert runtime: %+v", runtime)
	}
	if runtime.ContractAddress != "" || runtime.OperatorAddress != "" || len(runtime.PaymentTokens) != 0 {
		t.Fatalf("disabled runtime invented trusted identity: %+v", runtime)
	}
}

func TestFairLaunchV1RejectsAmbiguousEnabledValue(t *testing.T) {
	resetRuntimeConfig(t)
	viper.Set(fairLaunchPrefix+".enabled", "yes")
	if _, err := FairLaunchV1(); err == nil {
		t.Fatal("ambiguous enabled value accepted")
	}
}

func TestFairLaunchV1ValidatesCompleteTrustedRuntime(t *testing.T) {
	resetRuntimeConfig(t)
	configureValidFairLaunchRuntime()
	runtime, err := FairLaunchV1()
	if err != nil {
		t.Fatal(err)
	}
	if !runtime.Enabled || runtime.ChainID != 56 || runtime.PlatformFeeBps != 250 || runtime.ReleasePolicyVersion != 2 {
		t.Fatalf("wrong trusted runtime: %+v", runtime)
	}
	if runtime.PaymentTokens["USDT"].PolicyVersion != 7 || runtime.PaymentTokens["USDT"].Decimals != 18 {
		t.Fatalf("wrong payment token snapshot: %+v", runtime.PaymentTokens)
	}
}

func TestFairLaunchV1FailsClosedOnSharedPaymentTokenAddress(t *testing.T) {
	resetRuntimeConfig(t)
	configureValidFairLaunchRuntime()
	viper.Set(fairLaunchPrefix+".paymentTokens.USDC.address", "0x5000000000000000000000000000000000000005")
	if _, err := FairLaunchV1(); err == nil {
		t.Fatal("shared payment-token address accepted")
	}
}

func TestFairLaunchV1DoesNotAcceptPrivateKeyConfiguration(t *testing.T) {
	resetRuntimeConfig(t)
	configureValidFairLaunchRuntime()
	viper.Set(fairLaunchPrefix+".privateKey", strings.Repeat("1", 64))
	runtime, err := FairLaunchV1()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.OperatorAddress == "" {
		t.Fatal("operator identity was lost")
	}
	// The runtime type has no key field by construction; setting this legacy-like
	// value cannot turn it into a signer or broadcaster.
}
