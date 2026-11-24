package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/idov3whitelist"
)

// MatchV3ReplayProjectEndOfBlock is an additional consistency check on a
// complete, ordered block journal against a separately pinned getProject read.
// It does not authenticate either input or grant journal/entitlement access.
// Invoke only after all project logs at the block have been applied; checking
// midway through a block would reject legitimate transactions.
func MatchV3ReplayProjectEndOfBlock(binding idov3whitelist.ExpectedProjectBinding, totals V3ReplayTotals,
	observed idov3whitelist.ProjectIdentityObservation, height uint64, hash common.Hash) error {
	if height == 0 || hash == (common.Hash{}) || idov3whitelist.MatchProjectIdentityObservation(binding, observed) != nil ||
		observed.BlockNumber != height || observed.BlockHash != hash || observed.State != totals.State ||
		totals.Raised == nil || totals.Sold == nil || totals.Raised.Sign() < 0 || totals.Sold.Sign() < 0 ||
		totals.Raised.BitLen() > 256 || totals.Sold.BitLen() > 256 {
		return errors.New("V3 projection project identity or state mismatch")
	}
	parse := func(raw string) (*big.Int, bool) {
		n, ok := new(big.Int).SetString(raw, 10)
		return n, ok && n.Sign() >= 0 && n.BitLen() <= 256 && n.String() == raw
	}
	raised, raisedOK := parse(observed.RaisedAtBlockRaw)
	sold, soldOK := parse(observed.TokensSoldAtBlockRaw)
	if !raisedOK || !soldOK || raised.Cmp(totals.Raised) != 0 || sold.Cmp(totals.Sold) != 0 {
		return errors.New("V3 projection on-chain counters mismatch")
	}
	return nil
}

// MatchV3CreationProjectAtBlock also checks the immutable creation economics
// against getProject. A matching receipt or topic cannot establish source
// approval or a financial entitlement; both inputs need independent pinning.
func MatchV3CreationProjectAtBlock(created *V3CreatedEvent, binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation) error {
	if created == nil || observed.BlockNumber != created.BlockNumber || observed.BlockHash != created.BlockHash ||
		MatchV3CreationProjectImmutable(created, binding, observed) != nil {
		return errors.New("V3 creation economics do not match pinned project")
	}
	return nil
}

// MatchV3CreationProjectImmutable verifies fields that must not drift on any
// later canonical block; it does not accept an event or RPC as authority.
func MatchV3CreationProjectImmutable(created *V3CreatedEvent, binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation) error {
	if MatchV3CreatedBinding(created, binding) != nil || idov3whitelist.MatchProjectIdentityObservation(binding, observed) != nil ||
		observed.BlockNumber < created.BlockNumber ||
		observed.ProjectOwner != created.ProjectOwner || observed.FeeReceiver != created.FeeReceiver ||
		observed.ProjectDecimals != created.ProjectDecimals || observed.PaymentDecimals != created.PaymentDecimals || observed.FeeRate != created.FeeRate ||
		observed.TokenPriceRaw != created.TokenPriceRaw || observed.HardCapRaw != created.HardCapRaw || observed.SoftCapRaw != created.SoftCapRaw ||
		observed.MaxPerUserRaw != created.MaxPerUserRaw || observed.StartTimeRaw != created.StartTimeRaw || observed.EndTimeRaw != created.EndTimeRaw {
		return errors.New("V3 creation economics do not match pinned project")
	}
	return nil
}
