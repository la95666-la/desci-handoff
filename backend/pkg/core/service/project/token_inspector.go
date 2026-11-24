package project

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/chainrpc"
)

const projectTokenInspectionTimeout = 8 * time.Second

type tokenReadClient interface {
	ChainID(context.Context) (*big.Int, error)
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
	StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	Close()
}

var dialTokenReadClient = func(ctx context.Context, url string) (tokenReadClient, error) {
	return chainrpc.Dial(ctx, url)
}

func verifyProjectTokensOnChain(projectTokenAddress string, claimedProjectDecimals int, trusted trustedPaymentToken) (string, int, error) {
	rpcURL := strings.TrimSpace(viper.GetString("chain.project.url"))
	if rpcURL == "" {
		return "", 0, errors.New("project chain RPC is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), projectTokenInspectionTimeout)
	defer cancel()
	client, err := dialTokenReadClient(ctx, rpcURL)
	if err != nil {
		return "", 0, fmt.Errorf("connect project chain RPC: %w", err)
	}
	defer client.Close()
	return inspectProjectTokensWithPolicy(ctx, client, projectTokenAddress, claimedProjectDecimals, trusted, false, true)
}

func verifyProjectTokensForCreate(projectTokenAddress string, claimedProjectDecimals int, trusted trustedPaymentToken, expectedOwner common.Address) error {
	rpcURL := strings.TrimSpace(viper.GetString("chain.project.url"))
	if rpcURL == "" {
		return errors.New("project chain RPC is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), projectTokenInspectionTimeout)
	defer cancel()
	client, err := dialTokenReadClient(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("connect project chain RPC: %w", err)
	}
	defer client.Close()
	if _, _, err := inspectProjectTokensWithPolicy(ctx, client, projectTokenAddress, claimedProjectDecimals, trusted, true, true); err != nil {
		return err
	}
	return verifyV2Operator(ctx, client, common.HexToAddress(trusted.Contract), expectedOwner)
}

func verifyProjectTokensForActivation(projectTokenAddress string, claimedProjectDecimals int, trusted trustedPaymentToken, expectedOwner common.Address) error {
	rpcURL := strings.TrimSpace(viper.GetString("chain.project.url"))
	if rpcURL == "" {
		return errors.New("project chain RPC is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), projectTokenInspectionTimeout)
	defer cancel()
	client, err := dialTokenReadClient(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("connect project chain RPC: %w", err)
	}
	defer client.Close()
	if _, _, err := inspectProjectTokensWithPolicy(ctx, client, projectTokenAddress, claimedProjectDecimals, trusted, true, false); err != nil {
		return err
	}
	return verifyV2Operator(ctx, client, common.HexToAddress(trusted.Contract), expectedOwner)
}

// inspectProjectTokens verifies chain facts before a review is persisted. A future V2
// create relayer must call it again with requireProjectAllowed=true immediately before
// signing, because governance may whitelist or revoke a project token after review.
func inspectProjectTokens(ctx context.Context, client tokenReadClient, projectTokenAddress string, claimedProjectDecimals int, trusted trustedPaymentToken, requireProjectAllowed bool) (string, int, error) {
	return inspectProjectTokensWithPolicy(ctx, client, projectTokenAddress, claimedProjectDecimals, trusted, requireProjectAllowed, true)
}

func inspectProjectTokensWithPolicy(ctx context.Context, client tokenReadClient, projectTokenAddress string, claimedProjectDecimals int, trusted trustedPaymentToken, requireProjectAllowed, requirePaymentAllowed bool) (string, int, error) {
	actualChainID, err := client.ChainID(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("read project chain ID: %w", err)
	}
	if actualChainID == nil {
		return "", 0, errors.New("project chain RPC returned no chain ID")
	}
	if actualChainID.Sign() <= 0 || actualChainID.Cmp(new(big.Int).SetUint64(trusted.ChainID)) != 0 {
		return "", 0, fmt.Errorf("project chain ID mismatch: configured %d, RPC returned %s", trusted.ChainID, actualChainID.String())
	}

	projectToken := common.HexToAddress(projectTokenAddress)
	paymentToken := common.HexToAddress(trusted.Address)
	v2Contract := common.HexToAddress(trusted.Contract)
	for label, address := range map[string]common.Address{
		"project token": projectToken,
		"payment token": paymentToken,
		"IDO V2 proxy":  v2Contract,
	} {
		code, codeErr := client.CodeAt(ctx, address, nil)
		if codeErr != nil {
			return "", 0, fmt.Errorf("read %s bytecode: %w", label, codeErr)
		}
		if len(code) == 0 {
			return "", 0, fmt.Errorf("%s has no bytecode at %s", label, address.Hex())
		}
	}
	if err := verifyV2ProxyImplementation(ctx, client, trusted); err != nil {
		return "", 0, err
	}
	if err := verifyV2FeeSnapshot(ctx, client, trusted); err != nil {
		return "", 0, err
	}

	projectDecimals, err := readERC20Decimals(ctx, client, projectToken)
	if err != nil {
		return "", 0, fmt.Errorf("read project token decimals: %w", err)
	}
	// claimedProjectDecimals == 0 表示调用方未预声明（以链上读取为准），跳过严格比对。
	// 0 不是合法的 ERC20 decimals，作哨兵值安全。
	if claimedProjectDecimals != 0 && int(projectDecimals) != claimedProjectDecimals {
		return "", 0, fmt.Errorf("project token decimals mismatch: submitted %d, chain returned %d", claimedProjectDecimals, projectDecimals)
	}
	paymentDecimals, err := readERC20Decimals(ctx, client, paymentToken)
	if err != nil {
		return "", 0, fmt.Errorf("read payment token decimals: %w", err)
	}
	if int(paymentDecimals) != trusted.Decimals {
		return "", 0, fmt.Errorf("payment token decimals mismatch: configured %d, chain returned %d", trusted.Decimals, paymentDecimals)
	}

	if requireProjectAllowed {
		projectAllowed, configuredProjectDecimals, configErr := readV2TokenConfig(ctx, client, v2Contract, "projectTokenConfigs(address)", projectToken)
		if configErr != nil {
			return "", 0, fmt.Errorf("read V2 project token configuration: %w", configErr)
		}
		if !projectAllowed {
			return "", 0, errors.New("project token is not allowed by the configured IDO V2 proxy")
		}
		if configuredProjectDecimals != projectDecimals {
			return "", 0, fmt.Errorf("V2 project token decimals mismatch: proxy %d, token %d", configuredProjectDecimals, projectDecimals)
		}
	}
	paymentAllowed, configuredPaymentDecimals, err := readV2TokenConfig(ctx, client, v2Contract, "paymentTokenConfigs(address)", paymentToken)
	if err != nil {
		return "", 0, fmt.Errorf("read V2 payment token configuration: %w", err)
	}
	if requirePaymentAllowed && !paymentAllowed {
		return "", 0, fmt.Errorf("%s is not allowed by the configured IDO V2 proxy", trusted.Symbol)
	}
	if configuredPaymentDecimals != paymentDecimals {
		return "", 0, fmt.Errorf("V2 payment token decimals mismatch: proxy %d, token %d", configuredPaymentDecimals, paymentDecimals)
	}
	return projectToken.Hex(), int(projectDecimals), nil
}

func verifyV2ProxyImplementation(ctx context.Context, client tokenReadClient, trusted trustedPaymentToken) error {
	proxy := common.HexToAddress(trusted.Contract)
	approvedImplementation := common.HexToAddress(trusted.ImplementationAddress)
	slot := common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
	stored, err := client.StorageAt(ctx, proxy, slot, nil)
	if err != nil {
		return fmt.Errorf("read IDO V2 implementation slot: %w", err)
	}
	actualImplementation := common.BytesToAddress(stored)
	if actualImplementation != approvedImplementation {
		return fmt.Errorf("IDO V2 implementation mismatch: approved %s, actual %s", approvedImplementation.Hex(), actualImplementation.Hex())
	}
	code, err := client.CodeAt(ctx, actualImplementation, nil)
	if err != nil {
		return fmt.Errorf("read IDO V2 implementation bytecode: %w", err)
	}
	if len(code) == 0 {
		return errors.New("IDO V2 implementation has no bytecode")
	}
	actualHash := crypto.Keccak256Hash(code)
	if actualHash != common.HexToHash(trusted.ImplementationCodeHash) {
		return fmt.Errorf("IDO V2 implementation runtime code hash mismatch: approved %s, actual %s", trusted.ImplementationCodeHash, actualHash.Hex())
	}
	return nil
}

func verifyV2Operator(ctx context.Context, client tokenReadClient, contract, expectedOperator common.Address) error {
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: crypto.Keccak256([]byte("operator()"))[:4]}, nil)
	if err != nil {
		return fmt.Errorf("read IDO V2 operator: %w", err)
	}
	if len(result) != 32 {
		return fmt.Errorf("unexpected operator ABI result length %d", len(result))
	}
	operator := common.BytesToAddress(result)
	if operator == (common.Address{}) || operator != expectedOperator {
		return fmt.Errorf("IDO V2 operator mismatch: expected relayer %s, actual %s", expectedOperator.Hex(), operator.Hex())
	}
	return nil
}

func verifyV2FeeSnapshot(ctx context.Context, client tokenReadClient, trusted trustedPaymentToken) error {
	contract := common.HexToAddress(trusted.Contract)
	feeReceiverResult, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: crypto.Keccak256([]byte("feeReceiver()"))[:4]}, nil)
	if err != nil {
		return fmt.Errorf("read IDO V2 feeReceiver: %w", err)
	}
	if len(feeReceiverResult) != 32 {
		return fmt.Errorf("unexpected feeReceiver ABI result length %d", len(feeReceiverResult))
	}
	actualReceiver := common.BytesToAddress(feeReceiverResult)
	if actualReceiver != common.HexToAddress(trusted.FeeReceiver) {
		return fmt.Errorf("IDO V2 feeReceiver mismatch: expected %s, actual %s", trusted.FeeReceiver, actualReceiver.Hex())
	}
	rateResult, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: crypto.Keccak256([]byte("successFeeRate()"))[:4]}, nil)
	if err != nil {
		return fmt.Errorf("read IDO V2 successFeeRate: %w", err)
	}
	if len(rateResult) != 32 {
		return fmt.Errorf("unexpected successFeeRate ABI result length %d", len(rateResult))
	}
	rate := new(big.Int).SetBytes(rateResult)
	if !rate.IsUint64() || rate.Uint64() != uint64(trusted.SuccessFeeRate) {
		return fmt.Errorf("IDO V2 successFeeRate mismatch: expected %d, actual %s", trusted.SuccessFeeRate, rate.String())
	}
	return nil
}

func readERC20Decimals(ctx context.Context, client tokenReadClient, token common.Address) (uint8, error) {
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: crypto.Keccak256([]byte("decimals()"))[:4],
	}, nil)
	if err != nil {
		return 0, err
	}
	value, err := decodeABIUint8(result)
	if err != nil {
		return 0, err
	}
	if value == 0 || value > 18 {
		return 0, fmt.Errorf("unsupported decimals %d", value)
	}
	return value, nil
}

func readV2TokenConfig(ctx context.Context, client tokenReadClient, contract common.Address, signature string, token common.Address) (bool, uint8, error) {
	data := make([]byte, 4+32)
	copy(data[:4], crypto.Keccak256([]byte(signature))[:4])
	copy(data[4+12:], token.Bytes())
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
	if err != nil {
		return false, 0, err
	}
	if len(result) != 64 {
		return false, 0, fmt.Errorf("unexpected ABI result length %d", len(result))
	}
	allowedWord := new(big.Int).SetBytes(result[:32])
	if allowedWord.Sign() < 0 || allowedWord.Cmp(big.NewInt(1)) > 0 {
		return false, 0, errors.New("invalid ABI boolean")
	}
	decimals, err := decodeABIUint8(result[32:])
	if err != nil {
		return false, 0, err
	}
	return allowedWord.Sign() == 1, decimals, nil
}

func decodeABIUint8(result []byte) (uint8, error) {
	if len(result) != 32 {
		return 0, fmt.Errorf("unexpected ABI uint8 result length %d", len(result))
	}
	value := new(big.Int).SetBytes(result)
	if !value.IsUint64() || value.Uint64() > 255 {
		return 0, errors.New("ABI uint8 result is out of range")
	}
	return uint8(value.Uint64()), nil
}
