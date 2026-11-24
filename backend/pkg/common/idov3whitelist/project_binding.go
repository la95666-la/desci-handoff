package idov3whitelist

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
)

// ExpectedProjectBinding must come from an independently authenticated V3
// runtime and persisted capped_presale/ido_v3_stablecoin project identity.
// Constructing this value from a caller, RPC event or Warmup signup is unsafe.
type ExpectedProjectBinding struct {
	ChainID      uint64
	ProxyAddress common.Address
	ProjectID    uint64
	PaymentToken common.Address
	ProjectToken common.Address
}

// MatchProjectIdentityObservation is pure comparison, not runtime admission,
// finality certification, scanner registration or a financial write.
func MatchProjectIdentityObservation(expected ExpectedProjectBinding, got ProjectIdentityObservation) error {
	if expected.ChainID == 0 || expected.ProjectID == 0 || expected.ProxyAddress == (common.Address{}) ||
		expected.PaymentToken == (common.Address{}) || expected.ProjectToken == (common.Address{}) ||
		expected.PaymentToken == expected.ProjectToken || expected.ProxyAddress == expected.PaymentToken ||
		expected.ProxyAddress == expected.ProjectToken || got.BlockNumber == 0 || got.BlockHash == (common.Hash{}) ||
		got.State > ProjectCancelled || got.ChainID != expected.ChainID || got.ProxyAddress != expected.ProxyAddress ||
		got.ProjectID != expected.ProjectID || got.PaymentToken != expected.PaymentToken || got.ProjectToken != expected.ProjectToken {
		return errors.New("V3 project identity observation mismatch")
	}
	return nil
}
