package money

import (
	"errors"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"
)

var maxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// NormalizeRawUint256 validates and canonicalizes an unsigned base-10 uint256 amount.
func NormalizeRawUint256(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("raw amount is required")
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return "", errors.New("raw amount must contain decimal digits only")
		}
	}
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok || value.Sign() < 0 || value.Cmp(maxUint256) > 0 {
		return "", errors.New("raw amount is outside uint256 range")
	}
	return value.String(), nil
}

// DisplayToRaw converts a non-negative display amount into a canonical uint256 raw-unit string.
func DisplayToRaw(value decimal.Decimal, decimals uint8) (string, error) {
	if value.IsNegative() {
		return "", errors.New("display amount must not be negative")
	}
	if decimals > 77 {
		return "", errors.New("token decimals exceed supported raw-unit range")
	}
	shifted := value.Shift(int32(decimals))
	if !shifted.IsInteger() {
		return "", errors.New("display amount has more fractional digits than token decimals")
	}
	return NormalizeRawUint256(shifted.BigInt().String())
}

// FormatRaw renders a canonical raw token amount without floats or scientific notation.
func FormatRaw(raw string, decimals uint8) (string, error) {
	canonical, err := NormalizeRawUint256(raw)
	if err != nil {
		return "", err
	}
	if decimals == 0 {
		return canonical, nil
	}
	if decimals > 77 {
		return "", errors.New("token decimals exceed supported display range")
	}
	if len(canonical) <= int(decimals) {
		canonical = strings.Repeat("0", int(decimals)-len(canonical)+1) + canonical
	}
	cut := len(canonical) - int(decimals)
	integerPart := canonical[:cut]
	fractionPart := strings.TrimRight(canonical[cut:], "0")
	if fractionPart == "" {
		return integerPart, nil
	}
	return integerPart + "." + fractionPart, nil
}
