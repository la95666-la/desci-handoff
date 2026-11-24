package purchaseentitlement

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/money"
)

// MatchesPurchaseFormula mirrors JoinCareIDOv2.purchase Math.mulDiv:
// floor(paymentAmount * 10^projectTokenDecimals / tokenPrice).
func MatchesPurchaseFormula(paymentRaw, projectRaw, tokenPriceRaw string, projectTokenDecimals uint8) bool {
	payment, paymentOK := canonicalPositiveRaw(paymentRaw)
	projectTokens, projectOK := canonicalPositiveRaw(projectRaw)
	price, priceOK := canonicalPositiveRaw(tokenPriceRaw)
	if !paymentOK || !projectOK || !priceOK || projectTokenDecimals == 0 || projectTokenDecimals > 18 {
		return false
	}
	scale := new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(uint64(projectTokenDecimals)), nil)
	expected := new(big.Int).Quo(new(big.Int).Mul(payment, scale), price)
	return expected.Sign() > 0 && expected.Cmp(projectTokens) == 0
}

func Validate(fact *entity.IDOPurchaseEntitlement) error {
	if fact == nil {
		return errors.New("purchase entitlement fact is nil")
	}
	if fact.FundLedgerID == 0 || fact.ChainID == 0 || fact.ProjectID == 0 || fact.BlockNumber == 0 {
		return errors.New("ledger, chain, project, and block identifiers must be positive")
	}
	for name, address := range map[string]string{
		"contract": fact.ContractAddress, "wallet": fact.WalletAddress,
		"payment token": fact.PaymentTokenAddress, "project token": fact.ProjectTokenAddress,
	} {
		if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) {
			return fmt.Errorf("%s address is invalid", name)
		}
	}
	if strings.EqualFold(fact.PaymentTokenAddress, fact.ProjectTokenAddress) {
		return errors.New("payment and project token addresses must differ")
	}
	if fact.ContractVersion != constant.ContractVersionV2Stablecoin || fact.Status != fundledger.StatusCanonical {
		return errors.New("purchase entitlement requires canonical V2 identity")
	}
	if strings.TrimSpace(fact.PaymentTokenSymbol) == "" || strings.TrimSpace(fact.ProjectTokenSymbol) == "" {
		return errors.New("token symbols are required")
	}
	if fact.PaymentTokenDecimals == 0 || fact.PaymentTokenDecimals > 18 || fact.ProjectTokenDecimals == 0 || fact.ProjectTokenDecimals > 18 {
		return errors.New("token decimals must be between 1 and 18")
	}
	for name, amount := range map[string]struct {
		raw      string
		display  string
		decimals uint8
	}{
		"payment":       {fact.PaymentAmountRaw, fact.PaymentAmountDisplay, fact.PaymentTokenDecimals},
		"project token": {fact.ProjectTokenAmountRaw, fact.ProjectTokenDisplay, fact.ProjectTokenDecimals},
	} {
		canonical, err := money.NormalizeRawUint256(amount.raw)
		if err != nil || canonical != amount.raw {
			return fmt.Errorf("%s raw amount is invalid", name)
		}
		expected, err := money.FormatRaw(canonical, amount.decimals)
		if err != nil || expected != amount.display {
			return fmt.Errorf("%s display amount is invalid", name)
		}
		if canonical == "0" {
			return fmt.Errorf("%s amount must be positive", name)
		}
	}
	if !validHash(fact.TxHash) || !validHash(fact.BlockHash) {
		return errors.New("transaction and block hashes must be 32-byte hex")
	}
	if strings.TrimSpace(fact.EventData) == "" || !stdjson.Valid([]byte(fact.EventData)) {
		return errors.New("event data must be valid JSON")
	}
	return nil
}

func validHash(value string) bool {
	value = strings.TrimPrefix(value, "0x")
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func canonicalPositiveRaw(raw string) (*big.Int, bool) {
	canonical, err := money.NormalizeRawUint256(raw)
	if err != nil || canonical != raw {
		return nil, false
	}
	value, ok := new(big.Int).SetString(canonical, 10)
	return value, ok && value.Sign() > 0
}
