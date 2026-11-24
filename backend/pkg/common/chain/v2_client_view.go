package chain

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// IDOV2SnapshotView exposes only fixed-block read operations. Its underlying
// RPC client is deliberately unexported, and the view has no close, sign, or
// broadcast capability.
type IDOV2SnapshotView struct {
	reader *Client
}

// NewIDOV2SnapshotView returns a read-only capability over an existing RPC
// connection. The view does not own the shared connection and cannot close it.
func NewIDOV2SnapshotView(base *Client, contractAddress common.Address) (*IDOV2SnapshotView, error) {
	if base == nil || base.Client == nil {
		return nil, errors.New("V2 snapshot base RPC client is unavailable")
	}
	if contractAddress == (common.Address{}) {
		return nil, errors.New("V2 snapshot contract address is required")
	}
	contractABI, err := abi.JSON(strings.NewReader(idoV2ABIJson))
	if err != nil {
		return nil, err
	}
	view := *base
	view.PrivateKey = nil
	view.PublicKey = nil
	view.Address = common.Address{}
	view.Auth = nil
	view.ContractAddress = contractAddress
	view.EventAddresses = nil
	view.ContractABI = contractABI
	return &IDOV2SnapshotView{reader: &view}, nil
}

func (v *IDOV2SnapshotView) IDOV2ContractAddress() common.Address {
	if v == nil || v.reader == nil {
		return common.Address{}
	}
	return v.reader.IDOV2ContractAddress()
}

func (v *IDOV2SnapshotView) RPCChainID(ctx context.Context) (*big.Int, error) {
	return v.reader.RPCChainID(ctx)
}

func (v *IDOV2SnapshotView) LatestBlockNumber(ctx context.Context) (uint64, error) {
	return v.reader.LatestBlockNumber(ctx)
}

func (v *IDOV2SnapshotView) HeaderAt(ctx context.Context, blockNumber *big.Int) (*types.Header, error) {
	return v.reader.HeaderAt(ctx, blockNumber)
}

func (v *IDOV2SnapshotView) VerifyIDOV2RuntimeAt(ctx context.Context, blockNumber *big.Int, implementation common.Address, codeHash common.Hash) error {
	return v.reader.VerifyIDOV2RuntimeAt(ctx, blockNumber, implementation, codeHash)
}

func (v *IDOV2SnapshotView) ReadIDOV2ProjectAt(ctx context.Context, projectID, blockNumber *big.Int) (IDOV2ProjectSnapshot, error) {
	return v.reader.ReadIDOV2ProjectAt(ctx, projectID, blockNumber)
}

func (v *IDOV2SnapshotView) ReadIDOV2UserPurchaseAt(ctx context.Context, projectID *big.Int, user common.Address, blockNumber *big.Int) (IDOV2UserPurchaseSnapshot, error) {
	return v.reader.ReadIDOV2UserPurchaseAt(ctx, projectID, user, blockNumber)
}

func (v *IDOV2SnapshotView) ReadIDOV2ParticipantsAt(ctx context.Context, projectID, blockNumber *big.Int) ([]common.Address, error) {
	return v.reader.ReadIDOV2ParticipantsAt(ctx, projectID, blockNumber)
}

func (v *IDOV2SnapshotView) ReadIDOV2TokenConfigAt(ctx context.Context, projectToken bool, token common.Address, blockNumber *big.Int) (IDOV2TokenConfigSnapshot, error) {
	return v.reader.ReadIDOV2TokenConfigAt(ctx, projectToken, token, blockNumber)
}

func (v *IDOV2SnapshotView) ReadIDOV2ReservedBalanceAt(ctx context.Context, token common.Address, blockNumber *big.Int) (*big.Int, error) {
	return v.reader.ReadIDOV2ReservedBalanceAt(ctx, token, blockNumber)
}

func (v *IDOV2SnapshotView) ReadERC20BalanceAt(ctx context.Context, token, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return v.reader.ReadERC20BalanceAt(ctx, token, account, blockNumber)
}

func (v *IDOV2SnapshotView) ReadERC20DecimalsAt(ctx context.Context, token common.Address, blockNumber *big.Int) (uint8, error) {
	return v.reader.ReadERC20DecimalsAt(ctx, token, blockNumber)
}
