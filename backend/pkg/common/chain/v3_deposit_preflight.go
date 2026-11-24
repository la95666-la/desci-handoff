package chain

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

var v3AllowanceSelector = crypto.Keccak256([]byte("allowance(address,address)"))[:4]
var v3BalanceSelector = crypto.Keccak256([]byte("balanceOf(address)"))[:4]

// PreviewV3ProjectDepositFundingAt checks a project owner's token balance and
// allowance against the distinct V3 proxy at the SAME fixed block as an
// independently supplied project observation. It returns calldata, never a
// transaction or permission to use the shared relayer. This is only a
// point-in-time preflight: the wallet must recheck, approve if necessary,
// simulate, sign and broadcast independently; a later token change/reorg can
// invalidate the result. The code hash must be supplied from an independently
// reviewed source, not learned from this token RPC. The RPC itself must also
// be a separately trusted read source.
func PreviewV3ProjectDepositFundingAt(ctx context.Context, rpc idov3whitelist.PolicyRPC,
	binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation,
	caller common.Address, amount *big.Int, expectedTokenCodeHash common.Hash) ([]byte, error) {
	if ctx == nil || rpc == nil || observed.BlockNumber == 0 || observed.BlockHash == (common.Hash{}) || expectedTokenCodeHash == (common.Hash{}) {
		return nil, errors.New("V3 deposit funding preflight scope invalid")
	}
	data, err := PreviewV3ProjectDepositCall(binding, observed, caller, amount, V3ProjectOwner)
	if err != nil {
		return nil, err
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		return nil, errors.New("V3 deposit funding chain mismatched")
	}
	block := new(big.Int).SetUint64(observed.BlockNumber)
	anchor := func() error {
		header, readErr := rpc.HeaderByNumber(ctx, block)
		if readErr != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 || header.Hash() != observed.BlockHash {
			return errors.New("V3 deposit funding block changed")
		}
		return nil
	}
	if err := anchor(); err != nil {
		return nil, err
	}
	code, err := rpc.CodeAt(ctx, binding.ProjectToken, block)
	if err != nil || len(code) == 0 || crypto.Keccak256Hash(code) != expectedTokenCodeHash {
		return nil, errors.New("V3 deposit project token code unavailable or mismatched")
	}
	readUint := func(selector []byte, addresses ...common.Address) (*big.Int, error) {
		input := make([]byte, 4+32*len(addresses))
		copy(input, selector)
		for i, address := range addresses {
			copy(input[4+32*i+12:], address[:])
		}
		result, callErr := rpc.CallContract(ctx, ethereum.CallMsg{To: &binding.ProjectToken, Data: input}, block)
		if callErr != nil || len(result) != 32 {
			return nil, errors.New("V3 deposit token getter unavailable or malformed")
		}
		return new(big.Int).SetBytes(result), nil
	}
	balance, err := readUint(v3BalanceSelector, caller)
	if err != nil {
		return nil, err
	}
	allowance, err := readUint(v3AllowanceSelector, caller, binding.ProxyAddress)
	if err != nil {
		return nil, err
	}
	// The token identity is an independent expected hash, not a property
	// learned from the RPC. Reject a provider that changes its code answer
	// while the funding reads are in flight, even with a stable block header.
	again, err := rpc.CodeAt(ctx, binding.ProjectToken, block)
	if err != nil || len(again) == 0 || crypto.Keccak256Hash(again) != expectedTokenCodeHash {
		return nil, errors.New("V3 deposit project token code changed")
	}
	if err := anchor(); err != nil {
		return nil, err
	}
	if balance.Cmp(amount) < 0 || allowance.Cmp(amount) < 0 {
		return nil, errors.New("V3 deposit project owner balance or allowance insufficient")
	}
	// Token getters cannot detect a paused project or a transferFrom that
	// reverts/returns false. Simulate the actual owner -> distinct V3 proxy
	// call at the pinned block as a final read-only gate. A later wallet
	// transaction still needs a fresh simulation and its own signature.
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{From: caller, To: &binding.ProxyAddress, Data: data}, block)
	if err != nil || len(result) != 0 {
		return nil, errors.New("V3 deposit simulation unavailable or failed")
	}
	if err := anchor(); err != nil {
		return nil, err
	}
	return data, nil
}
