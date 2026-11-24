package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// GovernanceIdentity is supplied by an independently reviewed configuration.
// Matching this read-only observation cannot authenticate signer custody,
// threshold rules, Admin approvals or a production deployment.
type GovernanceIdentity struct {
	Owner, Governor                 common.Address
	OwnerCodeHash, GovernorCodeHash common.Hash
	// FollowOwner is explicit for production approved-mode reads. Legacy signed
	// manifests continue to bind their exact historical Owner address.
	FollowOwner bool
}

// ReadOwnerSafeAt reads the actual owner at the caller's pinned block. The
// independent Safe code trust root is retained; no EOA/zero/proxy/implementation
// or governor/operator collision is accepted. pendingOwner is not authority.
func ReadOwnerSafeAt(ctx context.Context, rpc PolicyRPC, proxy, implementation common.Address, codeHash common.Hash, block *big.Int) (common.Address, error) {
	invalid := errors.New("V3 current Owner Safe unavailable")
	if ctx == nil || rpc == nil || proxy == (common.Address{}) || implementation == (common.Address{}) || codeHash == (common.Hash{}) {
		return common.Address{}, invalid
	}
	before, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || before == nil || before.Number == nil || !before.Number.IsUint64() || (block != nil && before.Number.Cmp(block) != 0) {
		return common.Address{}, invalid
	}
	n := new(big.Int).Set(before.Number)
	read := func(selector []byte) (common.Address, error) {
		v, e := rpc.CallContract(ctx, ethereum.CallMsg{To: &proxy, Data: selector}, n)
		if e != nil || len(v) != 32 || !bytes.Equal(v[:12], make([]byte, 12)) {
			return common.Address{}, invalid
		}
		return common.BytesToAddress(v[12:]), nil
	}
	owner, err := read(v3OwnerSelector)
	if err != nil || owner == (common.Address{}) || owner == proxy || owner == implementation {
		return common.Address{}, invalid
	}
	for _, selector := range [][]byte{v3GovernorSelector, v3OperatorSelector} {
		a, e := read(selector)
		if e != nil || owner == a {
			return common.Address{}, invalid
		}
	}
	code, err := rpc.CodeAt(ctx, owner, n)
	if err != nil || len(code) == 0 || crypto.Keccak256Hash(code) != codeHash {
		return common.Address{}, invalid
	}
	again, e := read(v3OwnerSelector)
	after, err := rpc.HeaderByNumber(ctx, n)
	if e != nil || again != owner || err != nil || after == nil || after.Number == nil || after.Number.Cmp(n) != 0 || after.Hash() != before.Hash() {
		return common.Address{}, invalid
	}
	return owner, nil
}

// GovernanceObservation intentionally carries no Policy or Tree. An identity
// check must not become an accidental input to ProofForPolicy.
type GovernanceObservation struct {
	BlockNumber uint64
	BlockHash   common.Hash
	Owner       common.Address
	Governor    common.Address
}

// LifecycleCallerObservation records only a momentary fixed-block getter read.
// It is neither a Safe threshold/custody attestation nor signer admission.
type LifecycleCallerObservation struct {
	BlockNumber uint64
	BlockHash   common.Hash
	Owner       common.Address
	Operator    common.Address
}

var v3OwnerSelector = crypto.Keccak256([]byte("owner()"))[:4]
var v3GovernorSelector = crypto.Keccak256([]byte("whitelistGovernor()"))[:4]
var v3OperatorSelector = crypto.Keccak256([]byte("operator()"))[:4]

// ReadLifecycleCallersAt compares an independently supplied operator and
// governance identity with the pinned V3 contract. The contract getter, not
// a role label, must identify the proposed lifecycle caller. This does not
// authenticate the independently supplied expectation or release any outbox.
func ReadLifecycleCallersAt(ctx context.Context, rpc PolicyRPC, scope Scope, runtime RuntimeIdentity,
	expected GovernanceIdentity, operator common.Address, blockNumber uint64, blockHash common.Hash) (LifecycleCallerObservation, error) {
	if operator == (common.Address{}) || operator == scope.ProxyAddress || operator == runtime.Implementation ||
		operator == expected.Owner || operator == expected.Governor {
		return LifecycleCallerObservation{}, errors.New("V3 lifecycle operator identity invalid")
	}
	governance, err := ReadGovernanceAt(ctx, rpc, scope, runtime, expected, blockNumber, blockHash)
	if err != nil {
		return LifecycleCallerObservation{}, err
	}
	if operator == governance.Owner {
		return LifecycleCallerObservation{}, errors.New("V3 lifecycle operator identity invalid")
	}
	block := new(big.Int).SetUint64(blockNumber)
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{To: &scope.ProxyAddress, Data: v3OperatorSelector}, block)
	if err != nil || len(result) != 32 || !bytes.Equal(result[:12], make([]byte, 12)) || common.BytesToAddress(result[12:]) != operator {
		return LifecycleCallerObservation{}, errors.New("V3 lifecycle operator getter unavailable or mismatched")
	}
	// A later replacement invalidates the observation, even if owner/governor
	// and operator individually matched before the replacement.
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != blockHash {
		return LifecycleCallerObservation{}, errors.New("V3 lifecycle caller block changed during read")
	}
	return LifecycleCallerObservation{BlockNumber: governance.BlockNumber, BlockHash: governance.BlockHash,
		Owner: governance.Owner, Operator: operator}, nil
}

// ReadGovernanceAt checks the owner and policy governor at the same pinned
// block as a policy observation. It never submits a call or authorizes a proof.
// A later reorg or upgrade must be checked independently before any use.
func ReadGovernanceAt(ctx context.Context, rpc PolicyRPC, scope Scope, runtime RuntimeIdentity, expected GovernanceIdentity, blockNumber uint64, blockHash common.Hash) (GovernanceObservation, error) {
	if expected.FollowOwner {
		owner, err := ReadOwnerSafeAt(ctx, rpc, scope.ProxyAddress, runtime.Implementation, expected.OwnerCodeHash, new(big.Int).SetUint64(blockNumber))
		if err != nil {
			return GovernanceObservation{}, err
		}
		expected.Owner = owner
	}
	if expected.Owner == (common.Address{}) || expected.Governor == (common.Address{}) || expected.Owner == expected.Governor ||
		expected.Owner == scope.ProxyAddress || expected.Governor == scope.ProxyAddress ||
		expected.Owner == runtime.Implementation || expected.Governor == runtime.Implementation ||
		expected.OwnerCodeHash == (common.Hash{}) || expected.GovernorCodeHash == (common.Hash{}) {
		return GovernanceObservation{}, errors.New("V3 governance identity invalid")
	}
	// Reuse the strict proxy/implementation/version/policy check, but do not
	// return its policy: governance code identity is not member approval.
	_, err := ReadPolicyAt(ctx, rpc, scope, runtime, blockNumber, blockHash)
	if err != nil {
		return GovernanceObservation{}, err
	}
	block := new(big.Int).SetUint64(blockNumber)
	for _, contract := range []struct {
		address common.Address
		digest  common.Hash
	}{
		{expected.Owner, expected.OwnerCodeHash}, {expected.Governor, expected.GovernorCodeHash},
	} {
		code, codeErr := rpc.CodeAt(ctx, contract.address, block)
		if codeErr != nil || len(code) == 0 || crypto.Keccak256Hash(code) != contract.digest {
			return GovernanceObservation{}, errors.New("V3 governance contract code unavailable or mismatched")
		}
	}
	for _, read := range []struct {
		selector []byte
		expected common.Address
	}{
		{v3OwnerSelector, expected.Owner}, {v3GovernorSelector, expected.Governor},
	} {
		encoded, callErr := rpc.CallContract(ctx, ethereum.CallMsg{To: &scope.ProxyAddress, Data: read.selector}, block)
		if callErr != nil || len(encoded) != 32 || !bytes.Equal(encoded[:12], make([]byte, 12)) ||
			common.BytesToAddress(encoded[12:]) != read.expected {
			return GovernanceObservation{}, errors.New("V3 governance address unavailable or mismatched")
		}
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != blockHash {
		return GovernanceObservation{}, errors.New("V3 governance block changed during read")
	}
	return GovernanceObservation{BlockNumber: blockNumber, BlockHash: blockHash, Owner: expected.Owner, Governor: expected.Governor}, nil
}
