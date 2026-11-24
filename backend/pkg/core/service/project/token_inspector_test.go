package project

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type fakeTokenConfig struct {
	allowed  bool
	decimals uint8
}

type fakeTokenReadClient struct {
	chainID        *big.Int
	code           map[common.Address][]byte
	decimals       map[common.Address]uint8
	projectConfig  map[common.Address]fakeTokenConfig
	paymentConfig  map[common.Address]fakeTokenConfig
	callErr        error
	storage        map[common.Address]map[common.Hash][]byte
	owner          common.Address
	feeReceiver    common.Address
	successFeeRate uint16
}

func (f *fakeTokenReadClient) StorageAt(_ context.Context, address common.Address, slot common.Hash, _ *big.Int) ([]byte, error) {
	if slots := f.storage[address]; slots != nil {
		if value, ok := slots[slot]; ok {
			return value, nil
		}
	}
	return nil, errors.New("unknown storage slot")
}

func (f *fakeTokenReadClient) ChainID(context.Context) (*big.Int, error) {
	if f.chainID == nil {
		return nil, errors.New("chain unavailable")
	}
	return new(big.Int).Set(f.chainID), nil
}

func (f *fakeTokenReadClient) CodeAt(_ context.Context, address common.Address, _ *big.Int) ([]byte, error) {
	return f.code[address], nil
}

func (f *fakeTokenReadClient) CallContract(_ context.Context, call ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if f.callErr != nil {
		return nil, f.callErr
	}
	if call.To == nil || len(call.Data) < 4 {
		return nil, errors.New("invalid call")
	}
	if bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("decimals()"))[:4]) {
		value, ok := f.decimals[*call.To]
		if !ok {
			return nil, errors.New("unknown decimals target")
		}
		return abiWord(uint64(value)), nil
	}
	if bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("operator()"))[:4]) {
		return common.LeftPadBytes(f.owner.Bytes(), 32), nil
	}
	if bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("feeReceiver()"))[:4]) {
		return common.LeftPadBytes(f.feeReceiver.Bytes(), 32), nil
	}
	if bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("successFeeRate()"))[:4]) {
		return abiWord(uint64(f.successFeeRate)), nil
	}
	if len(call.Data) != 36 {
		return nil, errors.New("invalid config call")
	}
	token := common.BytesToAddress(call.Data[16:36])
	var config fakeTokenConfig
	var ok bool
	switch {
	case bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("projectTokenConfigs(address)"))[:4]):
		config, ok = f.projectConfig[token]
	case bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("paymentTokenConfigs(address)"))[:4]):
		config, ok = f.paymentConfig[token]
	default:
		return nil, errors.New("unknown selector")
	}
	if !ok {
		return nil, errors.New("unknown token config")
	}
	allowed := uint64(0)
	if config.allowed {
		allowed = 1
	}
	return append(abiWord(allowed), abiWord(uint64(config.decimals))...), nil
}

func (*fakeTokenReadClient) Close() {}

func abiWord(value uint64) []byte {
	word := make([]byte, 32)
	new(big.Int).SetUint64(value).FillBytes(word)
	return word
}

func validTokenInspectorFixture() (*fakeTokenReadClient, trustedPaymentToken, common.Address) {
	projectToken := common.HexToAddress("0x4444444444444444444444444444444444444444")
	paymentToken := common.HexToAddress("0x2222222222222222222222222222222222222222")
	v2Contract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	implementation := common.HexToAddress("0x5555555555555555555555555555555555555555")
	implementationCode := []byte{0x60, 0x00, 0x60, 0x00}
	implementationSlot := common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
	client := &fakeTokenReadClient{
		chainID: big.NewInt(97),
		code: map[common.Address][]byte{
			projectToken:   {1},
			paymentToken:   {1},
			v2Contract:     {1},
			implementation: implementationCode,
		},
		decimals: map[common.Address]uint8{
			projectToken: 18,
			paymentToken: 6,
		},
		projectConfig: map[common.Address]fakeTokenConfig{
			projectToken: {allowed: true, decimals: 18},
		},
		paymentConfig: map[common.Address]fakeTokenConfig{
			paymentToken: {allowed: true, decimals: 6},
		},
		storage:     map[common.Address]map[common.Hash][]byte{v2Contract: {implementationSlot: common.LeftPadBytes(implementation.Bytes(), 32)}},
		owner:       common.HexToAddress("0x9999999999999999999999999999999999999999"),
		feeReceiver: common.HexToAddress("0x8888888888888888888888888888888888888888"), successFeeRate: 200,
	}
	trusted := trustedPaymentToken{
		Symbol: "USDT", Address: paymentToken.Hex(), Decimals: 6,
		ChainID: 97, Contract: v2Contract.Hex(),
		ImplementationAddress: implementation.Hex(), ImplementationCodeHash: crypto.Keccak256Hash(implementationCode).Hex(),
		FeeReceiver: common.HexToAddress("0x8888888888888888888888888888888888888888").Hex(), SuccessFeeRate: 200,
	}
	return client, trusted, projectToken
}

func TestInspectProjectTokensActivationAllowsDisabledPaymentButRequiresProjectToken(t *testing.T) {
	client, trusted, projectToken := validTokenInspectorFixture()
	client.paymentConfig[common.HexToAddress(trusted.Address)] = fakeTokenConfig{allowed: false, decimals: 6}
	if _, _, err := inspectProjectTokensWithPolicy(context.Background(), client, projectToken.Hex(), 18, trusted, true, false); err != nil {
		t.Fatalf("activation policy rejected disabled payment token: %v", err)
	}
	client.projectConfig[projectToken] = fakeTokenConfig{allowed: false, decimals: 18}
	if _, _, err := inspectProjectTokensWithPolicy(context.Background(), client, projectToken.Hex(), 18, trusted, true, false); err == nil {
		t.Fatal("activation policy accepted disabled project token")
	}
}

func TestVerifyV2Owner(t *testing.T) {
	client, trusted, _ := validTokenInspectorFixture()
	contract := common.HexToAddress(trusted.Contract)
	if err := verifyV2Operator(context.Background(), client, contract, client.owner); err != nil {
		t.Fatalf("matching owner rejected: %v", err)
	}
	if err := verifyV2Operator(context.Background(), client, contract, common.HexToAddress("0x8888888888888888888888888888888888888888")); err == nil {
		t.Fatal("wrong relayer owner accepted")
	}
}

func TestInspectProjectTokensAcceptsGovernedChainFacts(t *testing.T) {
	client, trusted, projectToken := validTokenInspectorFixture()
	address, decimals, err := inspectProjectTokens(context.Background(), client, projectToken.Hex(), 18, trusted, true)
	if err != nil {
		t.Fatalf("valid governed token rejected: %v", err)
	}
	if address != projectToken.Hex() || decimals != 18 {
		t.Fatalf("unexpected verified token identity: %s/%d", address, decimals)
	}
}

func TestInspectProjectTokensFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*fakeTokenReadClient, *trustedPaymentToken, common.Address) int
		message string
	}{
		{name: "wrong chain", mutate: func(c *fakeTokenReadClient, _ *trustedPaymentToken, _ common.Address) int {
			c.chainID = big.NewInt(56)
			return 18
		}, message: "chain ID mismatch"},
		{name: "project EOA", mutate: func(c *fakeTokenReadClient, _ *trustedPaymentToken, p common.Address) int {
			delete(c.code, p)
			return 18
		}, message: "no bytecode"},
		{name: "wrong implementation", mutate: func(c *fakeTokenReadClient, trusted *trustedPaymentToken, _ common.Address) int {
			proxy := common.HexToAddress(trusted.Contract)
			slot := common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
			c.storage[proxy][slot] = common.LeftPadBytes(common.HexToAddress("0x6666666666666666666666666666666666666666").Bytes(), 32)
			return 18
		}, message: "implementation mismatch"},
		{name: "implementation code hash drift", mutate: func(c *fakeTokenReadClient, trusted *trustedPaymentToken, _ common.Address) int {
			c.code[common.HexToAddress(trusted.ImplementationAddress)] = []byte{0x60, 0x01}
			return 18
		}, message: "runtime code hash mismatch"},
		{name: "fee receiver drift", mutate: func(c *fakeTokenReadClient, _ *trustedPaymentToken, _ common.Address) int {
			c.feeReceiver = common.HexToAddress("0x7777777777777777777777777777777777777777")
			return 18
		}, message: "feeReceiver mismatch"},
		{name: "fee rate drift", mutate: func(c *fakeTokenReadClient, _ *trustedPaymentToken, _ common.Address) int {
			c.successFeeRate = 300
			return 18
		}, message: "successFeeRate mismatch"},
		{name: "submitted decimals mismatch", mutate: func(_ *fakeTokenReadClient, _ *trustedPaymentToken, _ common.Address) int { return 6 }, message: "submitted 6"},
		{name: "project token disabled", mutate: func(c *fakeTokenReadClient, _ *trustedPaymentToken, p common.Address) int {
			c.projectConfig[p] = fakeTokenConfig{allowed: false, decimals: 18}
			return 18
		}, message: "not allowed"},
		{name: "project governance decimals drift", mutate: func(c *fakeTokenReadClient, _ *trustedPaymentToken, p common.Address) int {
			c.projectConfig[p] = fakeTokenConfig{allowed: true, decimals: 6}
			return 18
		}, message: "proxy 6"},
		{name: "payment token disabled", mutate: func(c *fakeTokenReadClient, trusted *trustedPaymentToken, _ common.Address) int {
			c.paymentConfig[common.HexToAddress(trusted.Address)] = fakeTokenConfig{allowed: false, decimals: 6}
			return 18
		}, message: "USDT is not allowed"},
		{name: "payment token decimals drift", mutate: func(c *fakeTokenReadClient, trusted *trustedPaymentToken, _ common.Address) int {
			c.decimals[common.HexToAddress(trusted.Address)] = 18
			return 18
		}, message: "configured 6"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, trusted, projectToken := validTokenInspectorFixture()
			claimedDecimals := test.mutate(client, &trusted, projectToken)
			_, _, err := inspectProjectTokens(context.Background(), client, projectToken.Hex(), claimedDecimals, trusted, true)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error containing %q, got %v", test.message, err)
			}
		})
	}
}

func TestInspectProjectTokensReviewDoesNotRequirePrematureProjectWhitelist(t *testing.T) {
	client, trusted, projectToken := validTokenInspectorFixture()
	client.projectConfig[projectToken] = fakeTokenConfig{allowed: false, decimals: 0}
	address, decimals, err := inspectProjectTokens(context.Background(), client, projectToken.Hex(), 18, trusted, false)
	if err != nil {
		t.Fatalf("review should accept verified ERC20 metadata before Safe whitelist action: %v", err)
	}
	if address != projectToken.Hex() || decimals != 18 {
		t.Fatalf("unexpected verified token identity: %s/%d", address, decimals)
	}
}
