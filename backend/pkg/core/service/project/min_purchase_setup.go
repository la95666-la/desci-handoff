package project

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
)

func resolveMinimumForSet(current entity.Project, trusted trustedPaymentToken, maxRaw string) (*string, error) {
	if current.MinPurchaseDraftAmount != nil {
		if current.MinPurchaseDraftToken == nil || *current.MinPurchaseDraftToken != trusted.Symbol {
			return nil, errors.New("Setup minimum payment token differs from the Set payment token")
		}
		raw, err := validateSetupMinimumForSet(*current.MinPurchaseDraftAmount, trusted.Decimals, maxRaw)
		if err != nil {
			return nil, fmt.Errorf("Setup minimum cannot be applied at Set: %w", err)
		}
		return raw, nil
	}
	if current.MinPurchaseAmountRaw != nil {
		if current.PaymentToken != trusted.Symbol || current.PaymentTokenDecimals != trusted.Decimals {
			return nil, errors.New("existing minimum must be cleared before changing Set payment token")
		}
		if err := validateExistingMinimumForSet(*current.MinPurchaseAmountRaw, maxRaw); err != nil {
			return nil, fmt.Errorf("existing minimum cannot be applied at Set: %w", err)
		}
	}
	return current.MinPurchaseAmountRaw, nil
}

func validateSetupMinimumForSet(amount string, decimals int, maxRaw string) (*string, error) {
	if decimals < 1 || decimals > 18 {
		return nil, errors.New("payment token decimals are invalid")
	}
	for _, ch := range amount {
		if (ch < '0' || ch > '9') && ch != '.' {
			return nil, errors.New("minimum must be a plain positive decimal")
		}
	}
	parsed, err := decimal.NewFromString(strings.TrimSpace(amount))
	if err != nil || !parsed.IsPositive() {
		return nil, errors.New("minimum must be a positive decimal")
	}
	raw, err := money.DisplayToRaw(parsed, uint8(decimals))
	if err != nil {
		return nil, err
	}
	if err := validateExistingMinimumForSet(raw, maxRaw); err != nil {
		return nil, err
	}
	return &raw, nil
}

func validateExistingMinimumForSet(raw string, maxRaw string) error {
	canonical, err := money.NormalizeRawUint256(raw)
	if err != nil || canonical == "0" || canonical != raw {
		return errors.New("minimum raw units are invalid")
	}
	maximum, err := money.NormalizeRawUint256(maxRaw)
	if err != nil {
		return errors.New("on-chain per-user maximum is invalid")
	}
	min, _ := decimal.NewFromString(canonical)
	max, _ := decimal.NewFromString(maximum)
	if min.GreaterThan(max) {
		return errors.New("minimum exceeds on-chain per-user maximum")
	}
	return nil
}
