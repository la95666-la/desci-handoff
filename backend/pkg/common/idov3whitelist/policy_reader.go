package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// PolicyRPC is deliberately read-only; *ethclient.Client satisfies this
// interface. No signing, transaction submission or connection ownership is
// granted to a policy reader.
type PolicyRPC interface {
	ChainID(context.Context) (*big.Int, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
	StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
}

// RuntimeIdentity must come from separately approved, versioned V3 runtime
// configuration, never from the policy RPC response or a global V2 setting.
type RuntimeIdentity struct {
	Implementation common.Address
	ProxyCodeHash  common.Hash
	CodeHash       common.Hash
}

// PolicyObservation is a block-pinned observation, NOT a finality certificate
// or evidence that a four-eye approval has authorized the stored member set.
type PolicyObservation struct {
	Policy      Policy
	BlockNumber uint64
	BlockHash   common.Hash
}

var v3ContractVersion = crypto.Keccak256Hash([]byte("joincare.ido_v3_stablecoin.v1"))
var v3VersionSelector = crypto.Keccak256([]byte("contractVersion()"))[:4]
var v3PolicySelector = crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4]
var eip1967ImplementationSlot = common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")

// ReadPolicyAt matches the configured chain, proxy, contractVersion and block
// hash before accepting a fixed-block policy. Callers must select a finalized
// block under their chain's confirmation rules and independently bind the
// returned snapshot to the immutable approval record. Supplying a latest or
// pending block does NOT make it final. A post-read header check detects an
// in-flight replacement; a later reorg must still invalidate the observation.
// No raw RPC failure text escapes this helper to a user-facing API.
func ReadPolicyAt(ctx context.Context, rpc PolicyRPC, scope Scope, runtime RuntimeIdentity, blockNumber uint64, expectedBlockHash common.Hash) (PolicyObservation, error) {
	if rpc == nil || blockNumber == 0 || expectedBlockHash == (common.Hash{}) {
		return PolicyObservation{}, errors.New("invalid V3 policy snapshot request")
	}
	if err := validateScope(scope); err != nil {
		return PolicyObservation{}, err
	}
	if runtime.Implementation == (common.Address{}) || runtime.ProxyCodeHash == (common.Hash{}) || runtime.CodeHash == (common.Hash{}) || runtime.Implementation == scope.ProxyAddress {
		return PolicyObservation{}, errors.New("invalid V3 implementation identity")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || chainID.Cmp(scope.ChainID) != 0 {
		return PolicyObservation{}, errors.New("V3 policy chain identity unavailable or mismatched")
	}
	block := new(big.Int).SetUint64(blockNumber)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 || header.Hash() != expectedBlockHash {
		return PolicyObservation{}, errors.New("V3 policy snapshot block unavailable or mismatched")
	}
	code, err := rpc.CodeAt(ctx, scope.ProxyAddress, block)
	if err != nil || len(code) == 0 || crypto.Keccak256Hash(code) != runtime.ProxyCodeHash {
		return PolicyObservation{}, errors.New("V3 policy proxy code unavailable or mismatched")
	}
	slot, err := rpc.StorageAt(ctx, scope.ProxyAddress, eip1967ImplementationSlot, block)
	if err != nil || len(slot) != 32 || !bytes.Equal(slot[:12], make([]byte, 12)) || !bytes.Equal(slot[12:], runtime.Implementation[:]) {
		return PolicyObservation{}, errors.New("V3 policy proxy implementation unavailable or mismatched")
	}
	implementationCode, err := rpc.CodeAt(ctx, runtime.Implementation, block)
	if err != nil || len(implementationCode) == 0 || crypto.Keccak256Hash(implementationCode) != runtime.CodeHash {
		return PolicyObservation{}, errors.New("V3 policy implementation code unavailable or mismatched")
	}
	read := func(data []byte) ([]byte, error) {
		result, callErr := rpc.CallContract(ctx, ethereum.CallMsg{To: &scope.ProxyAddress, Data: data}, block)
		if callErr != nil {
			return nil, errors.New("V3 policy contract read unavailable")
		}
		return result, nil
	}
	version, err := read(v3VersionSelector)
	if err != nil || len(version) != 32 || !bytes.Equal(version, v3ContractVersion[:]) {
		return PolicyObservation{}, errors.New("V3 policy contract version unavailable or mismatched")
	}
	data := make([]byte, 4+32)
	copy(data, v3PolicySelector)
	scope.ProjectID.FillBytes(data[4:])
	encoded, err := read(data)
	if err != nil || len(encoded) != 96 {
		return PolicyObservation{}, errors.New("V3 policy read unavailable or malformed")
	}
	// The ABI return struct is (bool enabled, bytes32 root, uint64 version).
	// Decode strictly, including zero padding on the bool and uint64 words.
	if new(big.Int).SetBytes(encoded[:32]).Cmp(big.NewInt(1)) > 0 ||
		!bytes.Equal(encoded[64:88], make([]byte, 24)) {
		return PolicyObservation{}, errors.New("V3 policy read unavailable or malformed")
	}
	policy := Policy{
		Enabled: encoded[31] == 1,
		Root:    common.BytesToHash(encoded[32:64]),
		Version: new(big.Int).SetBytes(encoded[88:96]).Uint64(),
	}
	if policy.Enabled == (policy.Root == (common.Hash{})) || (policy.Enabled && policy.Version == 0) {
		return PolicyObservation{}, errors.New("V3 policy state is inconsistent")
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != expectedBlockHash {
		return PolicyObservation{}, errors.New("V3 policy snapshot block changed during read")
	}
	return PolicyObservation{Policy: policy, BlockNumber: blockNumber, BlockHash: expectedBlockHash}, nil
}
