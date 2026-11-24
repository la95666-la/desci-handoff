package project

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
)

// InsufficientProjectTokensError is the stable business error returned when the
// IDO V2 proxy's activateProject would revert with InsufficientProjectTokens().
// It carries the exact raw amounts so the client can show a precise shortfall.
type InsufficientProjectTokensError struct {
	// CurrentRaw is the project token raw balance reported for the project.
	CurrentRaw string `json:"current"`
	// RequiredRaw is the soft-cap-equivalent project token raw amount that the
	// contract requires deposited before activation (floor(mulDiv(softCap, 10^decimals, tokenPrice))).
	RequiredRaw string `json:"required"`
	// ShortfallRaw is RequiredRaw - CurrentRaw when positive.
	ShortfallRaw string `json:"shortfall"`
	// Symbol is the project token symbol (e.g. JCC).
	Symbol string `json:"symbol"`
	// Decimals is the project token decimals.
	Decimals int `json:"decimals"`
}

func (e *InsufficientProjectTokensError) Error() string {
	return fmt.Sprintf("V2 activation requires %s %s project tokens (raw), got %s (shortfall %s)", e.RequiredRaw, strings.TrimSpace(e.Symbol), e.CurrentRaw, e.ShortfallRaw)
}

// v2ActivationRequiredProjectTokens mirrors the IDO V2 proxy:
//
//	softCapTokenAmount = Math.mulDiv(softCap, 10 ** tokenDecimals, tokenPrice)
//
// which floor-divides (all operands are positive). raw amounts are canonical
// base-10 uint256 strings from the project row.
func v2ActivationRequiredProjectTokens(project entity.Project) (*big.Int, error) {
	softCap, err := money.NormalizeRawUint256(project.SoftCapRaw)
	if err != nil {
		return nil, fmt.Errorf("soft cap raw: %w", err)
	}
	price, err := money.NormalizeRawUint256(project.TokenPriceRaw)
	if err != nil {
		return nil, fmt.Errorf("token price raw: %w", err)
	}
	softCapInt, _ := new(big.Int).SetString(softCap, 10)
	priceInt, _ := new(big.Int).SetString(price, 10)
	if priceInt.Sign() <= 0 {
		return nil, errors.New("token price raw must be positive")
	}
	if project.TokenDecimals < 0 || project.TokenDecimals > 77 {
		return nil, errors.New("project token decimals are invalid")
	}
	scaled := new(big.Int).Mul(softCapInt, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(project.TokenDecimals)), nil))
	return new(big.Int).Div(scaled, priceInt), nil
}

// V2ActivationTokenShortfall compares the deposited project token amount
// against the soft-cap-equivalent requirement. It returns nil, nil when the
// deposit suffices; a *InsufficientProjectTokensError when it does not; and a
// validation error when the stored raw amounts are malformed.
func V2ActivationTokenShortfall(project entity.Project) (*InsufficientProjectTokensError, error) {
	required, err := v2ActivationRequiredProjectTokens(project)
	if err != nil {
		return nil, err
	}
	current, err := money.NormalizeRawUint256(project.TotalDepositedRaw)
	if err != nil {
		return nil, fmt.Errorf("total deposited raw: %w", err)
	}
	currentInt, _ := new(big.Int).SetString(current, 10)
	if currentInt.Cmp(required) >= 0 {
		return nil, nil
	}
	return &InsufficientProjectTokensError{
		CurrentRaw:   current,
		RequiredRaw:  required.String(),
		ShortfallRaw: new(big.Int).Sub(required, currentInt).String(),
		Symbol:       project.TokenSymbol,
		Decimals:     project.TokenDecimals,
	}, nil
}

// requireV2TokenSufficiency is the activateV2 preflight. It returns the typed
// business error the controller maps to a stable response.
func requireV2TokenSufficiency(project entity.Project) error {
	shortfall, err := V2ActivationTokenShortfall(project)
	if err != nil {
		return fmt.Errorf("V2 activation token sufficiency: %w", err)
	}
	if shortfall != nil {
		return shortfall
	}
	return nil
}

// ActivationTokenShortfallFromProject reloads the persisted project row and
// re-computes the token sufficiency shortfall. It is used to map a relayer
// revert surfaced during gas estimation back to the stable business error.
func ActivationTokenShortfallFromProject(projectID int64) (*InsufficientProjectTokensError, error) {
	info := entity.Project{}
	if err := db.Mysql().Select("token_symbol", "token_decimals", "soft_cap_raw", "token_price_raw", "total_deposited_raw").Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	return V2ActivationTokenShortfall(info)
}

// insufficientProjectTokensSelector is keccak256("InsufficientProjectTokens()")[:4].
// Some RPC stacks include the raw revert data in the error message.
var insufficientProjectTokensSelector = "e9548d03"

// IsInsufficientProjectTokensRPCError reports whether a relayer/RPC error is the
// on-chain InsufficientProjectTokens revert, so callers never echo raw RPC
// internals and instead answer with the stable business error.
func IsInsufficientProjectTokensRPCError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "insufficientprojecttokens") ||
		strings.Contains(message, "insufficient project tokens") ||
		strings.Contains(message, "0x"+insufficientProjectTokensSelector) ||
		strings.Contains(message, insufficientProjectTokensSelector)
}

// relayerInternalErrorMarkers are raw protocol/RPC fragments that must never be
// echoed verbatim to the admin client (the requirement: no RPC internals).
var relayerInternalErrorMarkers = []string{
	"estimate gas failed",
	"execution reverted",
	"rpc error",
	"insufficient funds",
	"connection refused",
	"dial tcp",
	"no such host",
	"context deadline exceeded",
}

// IsRelayerInternalError reports whether an error leaks raw chain RPC/protocol
// internals that a stable API response should not echo back to clients.
func IsRelayerInternalError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range relayerInternalErrorMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// V2ActivationInsufficientMessage is the stable user-facing message for the
// insufficient project token deposit business error.
func V2ActivationInsufficientMessage() string {
	return "IDO V2 activation requires more project tokens deposited"
}

// V2ActivationRelayerUnavailableMessage is the stable user-facing message used
// when a raw chain RPC/internal error must not be echoed back to the client.
func V2ActivationRelayerUnavailableMessage() string {
	return "IDO V2 activation could not be prepared by the chain relayer; check on-chain state and retry"
}

// V2ActivationInsufficientUnknownMessage is used when the on-chain revert could
// be identified but the persisted row no longer permits recomputing amounts.
func V2ActivationInsufficientUnknownMessage() string {
	return "IDO V2 activation was rejected on-chain due to insufficient project tokens; deposit tokens and retry"
}
