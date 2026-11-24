package projectruntime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
)

const fairLaunchPrefix = "chain.project.fairLaunchV1"

// PaymentToken is a trusted payment-token snapshot for one project runtime.
// Values come from server configuration, never from a client-supplied address.
type PaymentToken struct {
	Symbol        string `json:"symbol"`
	Address       string `json:"address"`
	Decimals      uint8  `json:"decimals"`
	PolicyDigest  string `json:"policyDigest"`
	PolicyVersion uint32 `json:"policyVersion"`
}

// Runtime is the fail-closed server-side identity used to route one funding
// contract. It intentionally contains no signing material.
type Runtime struct {
	Enabled                bool                    `json:"enabled"`
	Version                string                  `json:"version"`
	FundingMode            string                  `json:"fundingMode"`
	ChainID                uint64                  `json:"chainId"`
	ContractAddress        string                  `json:"contractAddress"`
	ImplementationAddress  string                  `json:"implementationAddress"`
	ImplementationCodeHash string                  `json:"implementationCodeHash"`
	StartBlock             uint64                  `json:"startBlock"`
	OperatorAddress        string                  `json:"operatorAddress"`
	PlatformFeeReceiver    string                  `json:"platformFeeReceiver"`
	PlatformFeeBps         uint16                  `json:"platformFeeBps"`
	ReleasePolicyDigest    string                  `json:"releasePolicyDigest"`
	ReleasePolicyVersion   uint32                  `json:"releasePolicyVersion"`
	ConfirmedConfirmations uint64                  `json:"confirmedConfirmations"`
	FinalizedConfirmations uint64                  `json:"finalizedConfirmations"`
	PaymentTokens          map[string]PaymentToken `json:"paymentTokens"`
}

// FairLaunchV1 loads the trusted runtime. Disabled mode accepts an entirely
// empty identity so deployments can ship inert support before addresses exist.
// Once enabled, every security-relevant identity is mandatory and validated.
func FairLaunchV1() (Runtime, error) {
	enabled, err := strictBool(fairLaunchPrefix + ".enabled")
	if err != nil {
		return Runtime{}, err
	}
	runtime := Runtime{
		Enabled:                enabled,
		Version:                constant.ContractVersionFairLaunchV1,
		FundingMode:            constant.FundingModeFairLaunch,
		ChainID:                viper.GetUint64("chain.project.chainID"),
		ContractAddress:        canonicalAddress(viper.GetString(fairLaunchPrefix + ".contractAddress")),
		ImplementationAddress:  canonicalAddress(viper.GetString(fairLaunchPrefix + ".implementationAddress")),
		ImplementationCodeHash: canonicalHash(viper.GetString(fairLaunchPrefix + ".implementationCodeHash")),
		StartBlock:             viper.GetUint64(fairLaunchPrefix + ".startBlock"),
		OperatorAddress:        canonicalAddress(viper.GetString(fairLaunchPrefix + ".operatorAddress")),
		PlatformFeeReceiver:    canonicalAddress(viper.GetString(fairLaunchPrefix + ".platformFeeReceiver")),
		ReleasePolicyDigest:    canonicalHash(viper.GetString(fairLaunchPrefix + ".releasePolicyDigest")),
		ConfirmedConfirmations: viper.GetUint64("chain.project.confirmedConfirmations"),
		FinalizedConfirmations: viper.GetUint64("chain.project.finalizedConfirmations"),
		PaymentTokens:          map[string]PaymentToken{},
	}
	if !enabled {
		return runtime, nil
	}
	if runtime.ChainID == 0 {
		return Runtime{}, errors.New("fair launch chain ID is required when enabled")
	}
	if !validAddress(runtime.ContractAddress) || !validAddress(runtime.ImplementationAddress) || !validAddress(runtime.OperatorAddress) || !validAddress(runtime.PlatformFeeReceiver) {
		return Runtime{}, errors.New("fair launch contract, implementation, operator, and platform fee receiver must be valid non-zero addresses")
	}
	if !validHash(runtime.ImplementationCodeHash) || !validHash(runtime.ReleasePolicyDigest) {
		return Runtime{}, errors.New("fair launch implementation code hash and release policy digest must be non-zero 32-byte hashes")
	}
	if runtime.StartBlock == 0 {
		return Runtime{}, errors.New("fair launch start block must be positive when enabled")
	}
	feeBps, err := parseUintConfig(fairLaunchPrefix+".platformFeeBps", 16)
	if err != nil || feeBps > 1000 {
		return Runtime{}, errors.New("fair launch platform fee bps must be an integer between 0 and 1000")
	}
	runtime.PlatformFeeBps = uint16(feeBps)
	releaseVersion, err := parseUintConfig(fairLaunchPrefix+".releasePolicyVersion", 32)
	if err != nil || releaseVersion == 0 {
		return Runtime{}, errors.New("fair launch release policy version must be positive")
	}
	runtime.ReleasePolicyVersion = uint32(releaseVersion)
	if runtime.ConfirmedConfirmations == 0 || runtime.FinalizedConfirmations < runtime.ConfirmedConfirmations {
		return Runtime{}, errors.New("fair launch confirmation policy is invalid")
	}

	symbols := viper.GetStringSlice(fairLaunchPrefix + ".paymentTokenSymbols")
	if len(symbols) == 0 {
		return Runtime{}, errors.New("fair launch payment token symbols are required when enabled")
	}
	seenAddresses := map[string]string{}
	for _, configured := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(configured))
		if symbol == "" {
			return Runtime{}, errors.New("fair launch payment token symbol is empty")
		}
		if _, exists := runtime.PaymentTokens[symbol]; exists {
			return Runtime{}, fmt.Errorf("duplicate fair launch payment token symbol %s", symbol)
		}
		prefix := fairLaunchPrefix + ".paymentTokens." + symbol
		decimals := viper.GetUint64(prefix + ".decimals")
		policyVersion := viper.GetUint64(prefix + ".policyVersion")
		token := PaymentToken{
			Symbol:        symbol,
			Address:       canonicalAddress(viper.GetString(prefix + ".address")),
			Decimals:      uint8(decimals),
			PolicyDigest:  canonicalHash(viper.GetString(prefix + ".policyDigest")),
			PolicyVersion: uint32(policyVersion),
		}
		if !validAddress(token.Address) || decimals == 0 || decimals > 18 || !validHash(token.PolicyDigest) || policyVersion == 0 || policyVersion > uint64(^uint32(0)) {
			return Runtime{}, fmt.Errorf("fair launch payment token %s identity or policy is invalid", symbol)
		}
		if previous, exists := seenAddresses[strings.ToLower(token.Address)]; exists {
			return Runtime{}, fmt.Errorf("fair launch payment tokens %s and %s share an address", previous, symbol)
		}
		seenAddresses[strings.ToLower(token.Address)] = symbol
		runtime.PaymentTokens[symbol] = token
	}
	return runtime, nil
}

func strictBool(key string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(viper.GetString(key)))
	switch value {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false", key)
	}
}

func parseUintConfig(key string, bits int) (uint64, error) {
	value := strings.TrimSpace(viper.GetString(key))
	if value == "" {
		return 0, errors.New("missing value")
	}
	return strconv.ParseUint(value, 10, bits)
}

func canonicalAddress(value string) string {
	value = strings.TrimSpace(value)
	if !common.IsHexAddress(value) {
		return value
	}
	return common.HexToAddress(value).Hex()
}

func canonicalHash(value string) string {
	value = strings.TrimSpace(value)
	if !validHash(value) {
		return value
	}
	return common.HexToHash(value).Hex()
}

func validAddress(value string) bool {
	return common.IsHexAddress(value) && common.HexToAddress(value) != (common.Address{})
}

func validHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") || common.HexToHash(value) == (common.Hash{}) {
		return false
	}
	for _, char := range value[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}
