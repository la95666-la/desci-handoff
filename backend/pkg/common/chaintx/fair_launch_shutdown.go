package chaintx

import (
	"errors"
	"strings"

	"joincare_server/pkg/common/constant"
)

// ErrFairLaunchMaterializationUnavailable is returned before any Fair Launch
// transaction can reserve a nonce, reach a signer/RPC, or mutate progression
// state while the authoritative materializer is incomplete.
var ErrFairLaunchMaterializationUnavailable = errors.New(constant.FairLaunchMaterializationUnavailable)

// ErrIDOV3MaterializationUnavailable fences future V3 actions at every shared
// signer/outbox entry point until a separately reviewed V3 materializer,
// durable action schema and reorg path exist. V2 actions remain unchanged.
var ErrIDOV3MaterializationUnavailable = errors.New("IDO V3 materialization unavailable")

// A formerly applied signature whose journal anchor was orphaned is kept
// durably, but is not replayed, resent, or treated as a fresh intent.
const ChainTxStateV3ReorgSuspended = "v3_reorg_suspended"

// IsFairLaunchTransactionAction classifies both current and future Fair Launch
// transaction types. New actions inherit the shutdown without an allowlist edit.
func IsFairLaunchTransactionAction(action string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(action)), "fair_launch_")
}

// IsIDOV3TransactionAction is intentionally prefix-based: an unknown future
// action must inherit the V3 shutdown, including in the recovery worker.
func IsIDOV3TransactionAction(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	return strings.HasPrefix(action, "v3_") || strings.HasPrefix(action, "ido_v3_")
}

func rejectFairLaunchTransactionAction(action string) error {
	if IsIDOV3TransactionAction(action) {
		return ErrIDOV3MaterializationUnavailable
	}
	if IsFairLaunchTransactionAction(action) {
		return ErrFairLaunchMaterializationUnavailable
	}
	return nil
}
