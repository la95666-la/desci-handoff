package fundledger

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
)

const (
	AssetRolePayment = "payment"
	AssetRoleProject = "project"
	StatusCanonical  = "canonical"

	EntryProjectTokenDeposited  = "project_token_deposited"
	EntryPurchasePayment        = "purchase_payment"
	EntryPaymentContributed     = "payment_contributed"
	EntryPaymentRefunded        = "payment_refunded"
	EntryProjectProceedsAccrued = "project_proceeds_accrued"
	EntryPlatformFeeAccrued     = "platform_fee_accrued"
	EntryProjectTokenClaimed    = "project_token_claimed"
	EntryProjectProceedsPaid    = "project_proceeds_paid"
	EntryPlatformFeePaid        = "platform_fee_paid"
	EntryProjectTokensWithdrawn = "project_tokens_withdrawn"
)

var entryAssetRoles = map[string]string{
	EntryProjectTokenDeposited:  AssetRoleProject,
	EntryPurchasePayment:        AssetRolePayment,
	EntryPaymentContributed:     AssetRolePayment,
	EntryPaymentRefunded:        AssetRolePayment,
	EntryProjectTokenClaimed:    AssetRoleProject,
	EntryProjectProceedsAccrued: AssetRolePayment,
	EntryProjectProceedsPaid:    AssetRolePayment,
	EntryPlatformFeeAccrued:     AssetRolePayment,
	EntryPlatformFeePaid:        AssetRolePayment,
	EntryProjectTokensWithdrawn: AssetRoleProject,
}

func Validate(fact *entity.FundLedger) error {
	if fact == nil {
		return errors.New("fund ledger fact is nil")
	}
	if fact.ChainID == 0 || fact.ProjectID == 0 || fact.BlockNumber == 0 {
		return errors.New("chain, project, and block identifiers must be positive")
	}
	if !validAddress(fact.ContractAddress) || !validAddress(fact.TokenAddress) {
		return errors.New("contract and token addresses must be valid non-zero EVM addresses")
	}
	if fact.WalletAddress != "" && !validAddress(fact.WalletAddress) {
		return errors.New("wallet address is invalid")
	}
	if fact.CounterpartyAddress != "" && !validAddress(fact.CounterpartyAddress) {
		return errors.New("counterparty address is invalid")
	}
	if fact.ContractVersion != constant.ContractVersionV2Stablecoin && fact.ContractVersion != constant.ContractVersionFairLaunchV1 {
		return fmt.Errorf("unsupported contract version %q", fact.ContractVersion)
	}
	if fact.ContractVersion == constant.ContractVersionFairLaunchV1 && fact.EntryType == EntryPurchasePayment {
		return errors.New("fair launch contribution cannot be recorded as a fixed-price purchase")
	}
	if fact.ContractVersion == constant.ContractVersionV2Stablecoin && fact.EntryType == EntryPaymentContributed {
		return errors.New("capped presale payment cannot be recorded as a fair launch contribution")
	}
	if fact.AssetRole != AssetRolePayment && fact.AssetRole != AssetRoleProject {
		return fmt.Errorf("unsupported asset role %q", fact.AssetRole)
	}
	expectedAssetRole, ok := entryAssetRoles[fact.EntryType]
	if !ok {
		return fmt.Errorf("unsupported entry type %q", fact.EntryType)
	}
	if fact.AssetRole != expectedAssetRole {
		return fmt.Errorf("entry type %q requires asset role %q", fact.EntryType, expectedAssetRole)
	}
	if strings.TrimSpace(fact.TokenSymbol) == "" {
		return errors.New("token symbol is required")
	}
	if fact.TokenDecimals == 0 || fact.TokenDecimals > 18 {
		return errors.New("token decimals must be between 1 and 18")
	}
	canonical, err := money.NormalizeRawUint256(fact.AmountRaw)
	if err != nil {
		return fmt.Errorf("invalid amount_raw: %w", err)
	}
	display, err := money.FormatRaw(canonical, fact.TokenDecimals)
	if err != nil {
		return err
	}
	if fact.AmountRaw != canonical || fact.AmountDisplay != display {
		return fmt.Errorf("amount fields are not canonical: raw=%q display=%q expected_raw=%q expected_display=%q", fact.AmountRaw, fact.AmountDisplay, canonical, display)
	}
	if !validHash(fact.TxHash) || !validHash(fact.BlockHash) {
		return errors.New("transaction and block hashes must be 32-byte hex")
	}
	if fact.Status != StatusCanonical {
		return fmt.Errorf("unsupported ledger status %q", fact.Status)
	}
	if strings.TrimSpace(fact.EventData) == "" || !stdjson.Valid([]byte(fact.EventData)) {
		return errors.New("event data must be valid JSON")
	}
	return nil
}

func validAddress(value string) bool {
	return common.IsHexAddress(value) && common.HexToAddress(value) != (common.Address{})
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
