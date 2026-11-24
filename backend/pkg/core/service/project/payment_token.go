package project

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

type trustedPaymentToken struct {
	Symbol                 string
	Address                string
	Decimals               int
	ChainID                uint64
	Contract               string
	ImplementationAddress  string
	ImplementationCodeHash string
	FeeReceiver            string
	SuccessFeeRate         uint16
}

func validateProjectV2Identity(info entity.Project, trusted trustedPaymentToken) error {
	if info.OnchainLocked {
		return errors.New("project on-chain identity is already locked")
	}
	return validateProjectV2IdentityFields(info, trusted)
}

func validateProjectV2IdentityFields(info entity.Project, trusted trustedPaymentToken) error {
	if err := constant.RequireExactV2ProjectIdentity(info.FundingMode, info.ContractVersion); err != nil {
		return err
	}
	if info.ChainID != trusted.ChainID || !strings.EqualFold(info.ContractAddress, trusted.Contract) {
		return errors.New("project chain or contract address does not match trusted V2 configuration")
	}
	if info.PaymentToken != trusted.Symbol || !strings.EqualFold(info.PaymentTokenAddress, trusted.Address) || info.PaymentTokenDecimals != trusted.Decimals {
		return errors.New("project payment token does not match trusted V2 configuration")
	}
	if !validNonZeroAddress(info.TokenAddress) || info.TokenDecimals <= 0 || info.TokenDecimals > 18 {
		return errors.New("project token identity is invalid")
	}
	if !validNonZeroAddress(info.OwnerAddress) {
		return errors.New("project owner address is invalid")
	}
	if strings.EqualFold(info.TokenAddress, info.PaymentTokenAddress) {
		return errors.New("project token and payment token must be different")
	}
	return nil
}

func resolveTrustedPaymentToken(symbol string) (trustedPaymentToken, error) {
	normalized, err := normalizePaymentTokenSymbol(symbol)
	if err != nil {
		return trustedPaymentToken{}, err
	}
	chainID := viper.GetUint64("chain.project.chainID")
	contractAddress := strings.TrimSpace(viper.GetString("chain.project.v2.contractAddress"))
	implementationAddress := strings.TrimSpace(viper.GetString("chain.project.v2.implementationAddress"))
	implementationCodeHash := strings.TrimSpace(viper.GetString("chain.project.v2.implementationCodeHash"))
	feeReceiver := strings.TrimSpace(viper.GetString("chain.project.v2.feeReceiver"))
	successFeeRateRaw := strings.TrimSpace(viper.GetString("chain.project.v2.successFeeRate"))
	successFeeRate, feeRateErr := strconv.ParseUint(successFeeRateRaw, 10, 16)
	if chainID == 0 {
		return trustedPaymentToken{}, errors.New("project chain ID is not configured")
	}
	if !validNonZeroAddress(contractAddress) {
		return trustedPaymentToken{}, errors.New("IDO V2 contract address is not configured or invalid")
	}
	if !validNonZeroAddress(implementationAddress) || !validHash(implementationCodeHash) || common.HexToHash(implementationCodeHash) == (common.Hash{}) {
		return trustedPaymentToken{}, errors.New("IDO V2 approved implementation address or runtime code hash is invalid")
	}
	if !validNonZeroAddress(feeReceiver) || feeRateErr != nil || successFeeRate > 1_000 {
		return trustedPaymentToken{}, errors.New("IDO V2 expected feeReceiver or successFeeRate is invalid")
	}
	tokens := make(map[string]trustedPaymentToken, 2)
	for _, configuredSymbol := range []string{constant.PaymentTokenUSDT, constant.PaymentTokenUSDC} {
		prefix := "chain.project.v2.paymentTokens." + configuredSymbol
		address := strings.TrimSpace(viper.GetString(prefix + ".address"))
		decimals := viper.GetInt(prefix + ".decimals")
		if !validNonZeroAddress(address) {
			return trustedPaymentToken{}, fmt.Errorf("trusted %s address is not configured or invalid", configuredSymbol)
		}
		if decimals <= 0 || decimals > 18 {
			return trustedPaymentToken{}, fmt.Errorf("trusted %s decimals must be between 1 and 18", configuredSymbol)
		}
		tokens[configuredSymbol] = trustedPaymentToken{
			Symbol: configuredSymbol, Address: common.HexToAddress(address).Hex(), Decimals: decimals,
			ChainID: chainID, Contract: common.HexToAddress(contractAddress).Hex(),
			ImplementationAddress: common.HexToAddress(implementationAddress).Hex(), ImplementationCodeHash: common.HexToHash(implementationCodeHash).Hex(),
			FeeReceiver: common.HexToAddress(feeReceiver).Hex(), SuccessFeeRate: uint16(successFeeRate),
		}
	}
	if strings.EqualFold(tokens[constant.PaymentTokenUSDT].Address, tokens[constant.PaymentTokenUSDC].Address) {
		return trustedPaymentToken{}, errors.New("USDT and USDC trusted addresses must be different")
	}
	return tokens[normalized], nil
}

func validHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, char := range value[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

func normalizePaymentTokenSymbol(symbol string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(symbol))
	if normalized != constant.PaymentTokenUSDT && normalized != constant.PaymentTokenUSDC {
		return "", errors.New("payment token must be USDT or USDC")
	}
	return normalized, nil
}

func validNonZeroAddress(value string) bool {
	return common.IsHexAddress(value) && common.HexToAddress(value) != (common.Address{})
}
