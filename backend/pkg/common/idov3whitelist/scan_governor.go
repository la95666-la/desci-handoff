package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// ReadScanGovernorAt verifies the governor recorded in a separately signed
// scanner admission. The signed address cannot be inferred from the runtime
// candidate row or from the contract getter itself. This checks identity,
// not Safe threshold, signer custody or an approval of any policy snapshot.
func ReadScanGovernorAt(ctx context.Context, rpc PolicyRPC, scope Scope, runtime RuntimeIdentity,
	governor common.Address, height uint64, hash common.Hash) error {
	if governor == (common.Address{}) || governor == scope.ProxyAddress || governor == runtime.Implementation {
		return errors.New("V3 scanner governor identity invalid")
	}
	if _, err := ReadPolicyAt(ctx, rpc, scope, runtime, height, hash); err != nil {
		return errors.New("V3 scanner governor runtime unavailable")
	}
	block := new(big.Int).SetUint64(height)
	code, err := rpc.CodeAt(ctx, governor, block)
	if err != nil || len(code) == 0 {
		return errors.New("V3 scanner governor contract unavailable")
	}
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{To: &scope.ProxyAddress, Data: v3GovernorSelector}, block)
	if err != nil || len(result) != 32 || !bytes.Equal(result[:12], make([]byte, 12)) || !bytes.Equal(result[12:], governor[:]) {
		return errors.New("V3 scanner governor getter mismatched")
	}
	// Recheck the actual implementation after the getter. A provider may change
	// the block between calls without changing the apparent governor address.
	if _, err := ReadPolicyAt(ctx, rpc, scope, runtime, height, hash); err != nil {
		return errors.New("V3 scanner governor block changed")
	}
	return nil
}

// ReadScanOperatorAt checks the release-pinned operator getter at the same
// confirmed block as the runtime, independently of any outbox submission.
// It does not authorize a signer or establish custody of the private key.
func ReadScanOperatorAt(ctx context.Context, rpc PolicyRPC, scope Scope, runtime RuntimeIdentity,
	operator common.Address, height uint64, hash common.Hash) error {
	if operator == (common.Address{}) || operator == scope.ProxyAddress || operator == runtime.Implementation {
		return errors.New("V3 creation operator invalid")
	}
	if _, err := ReadPolicyAt(ctx, rpc, scope, runtime, height, hash); err != nil {
		return errors.New("V3 creation operator runtime unavailable")
	}
	block := new(big.Int).SetUint64(height)
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{To: &scope.ProxyAddress, Data: v3OperatorSelector}, block)
	if err != nil || len(result) != 32 || !bytes.Equal(result[:12], make([]byte, 12)) || common.BytesToAddress(result[12:]) != operator {
		return errors.New("V3 creation operator getter mismatched")
	}
	if _, err := ReadPolicyAt(ctx, rpc, scope, runtime, height, hash); err != nil {
		return errors.New("V3 creation operator block changed")
	}
	return nil
}
