package chain

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func (c *Client) RPCChainID(ctx context.Context) (*big.Int, error) {
	if c == nil || c.Client == nil {
		return nil, errors.New("V2 RPC client is unavailable")
	}
	return c.Client.ChainID(ctx)
}

func (c *Client) IDOV2ContractAddress() common.Address {
	if c == nil {
		return common.Address{}
	}
	return c.ContractAddress
}

func (c *Client) LatestBlockNumber(ctx context.Context) (uint64, error) {
	if c == nil || c.Client == nil {
		return 0, errors.New("V2 RPC client is unavailable")
	}
	return c.Client.BlockNumber(ctx)
}

func (c *Client) HeaderAt(ctx context.Context, blockNumber *big.Int) (*types.Header, error) {
	if c == nil || c.Client == nil || blockNumber == nil || blockNumber.Sign() <= 0 {
		return nil, errors.New("invalid V2 header snapshot request")
	}
	return c.Client.HeaderByNumber(ctx, blockNumber)
}

func (c *Client) ReadIDOV2ProjectAt(ctx context.Context, projectID, blockNumber *big.Int) (IDOV2ProjectSnapshot, error) {
	values, err := c.callIDOV2At(ctx, "getProject", blockNumber, projectID)
	if err != nil {
		return IDOV2ProjectSnapshot{}, err
	}
	if len(values) != 1 {
		return IDOV2ProjectSnapshot{}, fmt.Errorf("unexpected getProject result count %d", len(values))
	}
	converted := *abiConvertIDOV2Project(values[0])
	if converted.ProjectID == nil || converted.ProjectID.Sign() <= 0 {
		return IDOV2ProjectSnapshot{}, errors.New("V2 project does not exist at snapshot block")
	}
	return converted, nil
}

func (c *Client) ReadIDOV2UserPurchaseAt(ctx context.Context, projectID *big.Int, user common.Address, blockNumber *big.Int) (IDOV2UserPurchaseSnapshot, error) {
	values, err := c.callIDOV2At(ctx, "getUserPurchase", blockNumber, projectID, user)
	if err != nil {
		return IDOV2UserPurchaseSnapshot{}, err
	}
	if len(values) != 1 {
		return IDOV2UserPurchaseSnapshot{}, fmt.Errorf("unexpected getUserPurchase result count %d", len(values))
	}
	return *abiConvertIDOV2UserPurchase(values[0]), nil
}

func (c *Client) ReadIDOV2ParticipantsAt(ctx context.Context, projectID, blockNumber *big.Int) ([]common.Address, error) {
	values, err := c.callIDOV2At(ctx, "getProjectParticipants", blockNumber, projectID)
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("unexpected getProjectParticipants result count %d", len(values))
	}
	participants, ok := values[0].([]common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected participants result type %T", values[0])
	}
	return append([]common.Address(nil), participants...), nil
}

func (c *Client) ReadIDOV2TokenConfigAt(ctx context.Context, projectToken bool, token common.Address, blockNumber *big.Int) (IDOV2TokenConfigSnapshot, error) {
	method := "paymentTokenConfigs"
	if projectToken {
		method = "projectTokenConfigs"
	}
	values, err := c.callIDOV2At(ctx, method, blockNumber, token)
	if err != nil {
		return IDOV2TokenConfigSnapshot{}, err
	}
	if len(values) != 2 {
		return IDOV2TokenConfigSnapshot{}, fmt.Errorf("unexpected %s result count %d", method, len(values))
	}
	allowed, allowedOK := values[0].(bool)
	decimals, decimalsOK := values[1].(uint8)
	if !allowedOK || !decimalsOK {
		return IDOV2TokenConfigSnapshot{}, fmt.Errorf("unexpected %s result types %T/%T", method, values[0], values[1])
	}
	return IDOV2TokenConfigSnapshot{Allowed: allowed, Decimals: decimals}, nil
}

func (c *Client) ReadIDOV2ReservedBalanceAt(ctx context.Context, token common.Address, blockNumber *big.Int) (*big.Int, error) {
	values, err := c.callIDOV2At(ctx, "reservedBalances", blockNumber, token)
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("unexpected reservedBalances result count %d", len(values))
	}
	value, ok := values[0].(*big.Int)
	if !ok || value == nil || value.Sign() < 0 {
		return nil, fmt.Errorf("unexpected reservedBalances result type %T", values[0])
	}
	return new(big.Int).Set(value), nil
}

func (c *Client) ReadERC20BalanceAt(ctx context.Context, token, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	data := make([]byte, 4+32)
	copy(data[:4], crypto.Keccak256([]byte("balanceOf(address)"))[:4])
	copy(data[4+12:], account.Bytes())
	result, err := c.Client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("call ERC20 balanceOf at block %s: %w", blockNumber, err)
	}
	if len(result) != 32 {
		return nil, fmt.Errorf("unexpected ERC20 balanceOf result length %d", len(result))
	}
	return new(big.Int).SetBytes(result), nil
}

func (c *Client) ReadERC20AllowanceAt(ctx context.Context, token, owner, spender common.Address, blockNumber *big.Int) (*big.Int, error) {
	data := make([]byte, 4+32+32)
	copy(data[:4], crypto.Keccak256([]byte("allowance(address,address)"))[:4])
	copy(data[4+12:36], owner.Bytes())
	copy(data[36+12:], spender.Bytes())
	result, err := c.Client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("call ERC20 allowance at block %s: %w", blockNumber, err)
	}
	if len(result) != 32 {
		return nil, fmt.Errorf("unexpected ERC20 allowance result length %d", len(result))
	}
	return new(big.Int).SetBytes(result), nil
}

func (c *Client) ReadERC20DecimalsAt(ctx context.Context, token common.Address, blockNumber *big.Int) (uint8, error) {
	if c == nil || c.Client == nil || blockNumber == nil || blockNumber.Sign() <= 0 {
		return 0, errors.New("invalid ERC20 decimals snapshot request")
	}
	result, err := c.Client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: crypto.Keccak256([]byte("decimals()"))[:4]}, blockNumber)
	if err != nil {
		return 0, fmt.Errorf("call ERC20 decimals at block %s: %w", blockNumber, err)
	}
	if len(result) != 32 {
		return 0, fmt.Errorf("unexpected ERC20 decimals result length %d", len(result))
	}
	value := new(big.Int).SetBytes(result)
	if !value.IsUint64() || value.Uint64() == 0 || value.Uint64() > 18 {
		return 0, fmt.Errorf("unsupported ERC20 decimals %s", value)
	}
	return uint8(value.Uint64()), nil
}

func (c *Client) VerifyIDOV2RuntimeAt(ctx context.Context, blockNumber *big.Int, expectedImplementation common.Address, expectedCodeHash common.Hash) error {
	if c == nil || c.Client == nil || blockNumber == nil || blockNumber.Sign() <= 0 {
		return errors.New("invalid V2 runtime snapshot request")
	}
	proxyCode, err := c.Client.CodeAt(ctx, c.ContractAddress, blockNumber)
	if err != nil {
		return fmt.Errorf("read V2 proxy code at block %s: %w", blockNumber, err)
	}
	if len(proxyCode) == 0 {
		return errors.New("V2 proxy has no bytecode at snapshot block")
	}
	slot := common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
	stored, err := c.Client.StorageAt(ctx, c.ContractAddress, slot, blockNumber)
	if err != nil {
		return fmt.Errorf("read V2 implementation slot at block %s: %w", blockNumber, err)
	}
	actualImplementation := common.BytesToAddress(stored)
	if actualImplementation != expectedImplementation {
		return fmt.Errorf("V2 implementation mismatch: expected %s actual %s", expectedImplementation.Hex(), actualImplementation.Hex())
	}
	code, err := c.Client.CodeAt(ctx, actualImplementation, blockNumber)
	if err != nil {
		return fmt.Errorf("read V2 implementation code at block %s: %w", blockNumber, err)
	}
	if len(code) == 0 {
		return errors.New("V2 implementation has no bytecode at snapshot block")
	}
	if actualHash := crypto.Keccak256Hash(code); actualHash != expectedCodeHash {
		return fmt.Errorf("V2 implementation runtime hash mismatch: expected %s actual %s", expectedCodeHash.Hex(), actualHash.Hex())
	}
	return nil
}

func (c *Client) callIDOV2At(ctx context.Context, method string, blockNumber *big.Int, args ...interface{}) ([]interface{}, error) {
	if c == nil || c.Client == nil || blockNumber == nil || blockNumber.Sign() <= 0 {
		return nil, errors.New("invalid V2 snapshot call")
	}
	data, err := c.ContractABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}
	result, err := c.Client.CallContract(ctx, ethereum.CallMsg{To: &c.ContractAddress, Data: data}, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("call %s at block %s: %w", method, blockNumber, err)
	}
	values, err := c.ContractABI.Unpack(method, result)
	if err != nil {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}
	return values, nil
}

func abiConvertIDOV2Project(value interface{}) *IDOV2ProjectSnapshot {
	return abi.ConvertType(value, new(IDOV2ProjectSnapshot)).(*IDOV2ProjectSnapshot)
}

func abiConvertIDOV2UserPurchase(value interface{}) *IDOV2UserPurchaseSnapshot {
	return abi.ConvertType(value, new(IDOV2UserPurchaseSnapshot)).(*IDOV2UserPurchaseSnapshot)
}
