package chain

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

type v3DepositReadStub struct {
	chain, balance, allowance                                                                     uint64
	header                                                                                        *types.Header
	proxy, token, owner                                                                           common.Address
	malformed, noCode, replace, fail, wrongTarget, changeCode, simulationRevert, simulationReturn bool
	calls, codeReads                                                                              int
}

func (s *v3DepositReadStub) ChainID(context.Context) (*big.Int, error) {
	return new(big.Int).SetUint64(s.chain), nil
}
func (s *v3DepositReadStub) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n.Cmp(s.header.Number) != 0 {
		return nil, errors.New("wrong block")
	}
	if s.replace && s.calls > 0 {
		changed := *s.header
		changed.Extra = []byte("replacement")
		return &changed, nil
	}
	return s.header, nil
}
func (s *v3DepositReadStub) CodeAt(_ context.Context, token common.Address, _ *big.Int) ([]byte, error) {
	s.codeReads++
	if token != s.token || s.noCode {
		return nil, errors.New("token code absent")
	}
	if s.changeCode && s.codeReads > 1 {
		return []byte{0x60, 0x02}, nil
	}
	return []byte{0x60, 0x01}, nil
}
func (*v3DepositReadStub) StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error) {
	panic("storage must not be read")
}
func (s *v3DepositReadStub) CallContract(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("sensitive provider failure")
	}
	if call.To != nil && *call.To == s.proxy {
		if call.From != s.owner || block.Cmp(s.header.Number) != 0 || len(call.Data) != 68 ||
			!bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("depositProjectTokens(uint256,uint256)"))[:4]) {
			return nil, errors.New("wrong deposit simulation target")
		}
		if s.simulationRevert {
			return nil, errors.New("sensitive simulation revert")
		}
		if s.simulationReturn {
			return []byte{1}, nil
		}
		return nil, nil
	}
	if call.To == nil || *call.To != s.token || block.Cmp(s.header.Number) != 0 {
		return nil, errors.New("wrong token/block")
	}
	if s.malformed {
		return []byte{1}, nil
	}
	selector := call.Data[:4]
	switch {
	case bytes.Equal(selector, v3BalanceSelector) && len(call.Data) == 36 && common.BytesToAddress(call.Data[16:36]) == s.owner:
	case bytes.Equal(selector, v3AllowanceSelector) && len(call.Data) == 68 && common.BytesToAddress(call.Data[16:36]) == s.owner && common.BytesToAddress(call.Data[48:68]) == s.proxy && !s.wrongTarget:
	default:
		return nil, errors.New("wrong selector or spender")
	}
	value := s.balance
	if bytes.Equal(selector, v3AllowanceSelector) {
		value = s.allowance
	}
	return common.LeftPadBytes(new(big.Int).SetUint64(value).Bytes(), 32), nil
}

func TestV3ProjectDepositFundingPreflightDoesNotAuthorizeOutbox(t *testing.T) {
	addr := common.HexToAddress
	proxy, token, pay, owner := addr("0x1111111111111111111111111111111111111111"), addr("0x2222222222222222222222222222222222222222"), addr("0x3333333333333333333333333333333333333333"), addr("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, ProjectToken: token, PaymentToken: pay}
	header := &types.Header{Number: big.NewInt(90)}
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, ProjectToken: token, PaymentToken: pay, ProjectOwner: owner, FeeReceiver: pay, State: idov3whitelist.ProjectPending, BlockNumber: 90, BlockHash: header.Hash()}
	newStub := func() *v3DepositReadStub {
		return &v3DepositReadStub{chain: 31337, balance: 10, allowance: 10, header: header, proxy: proxy, token: token, owner: owner}
	}
	codeHash := crypto.Keccak256Hash([]byte{0x60, 0x01})
	good := newStub()
	data, err := PreviewV3ProjectDepositFundingAt(context.Background(), good, binding, observed, owner, big.NewInt(10), codeHash)
	if err != nil || len(data) != 68 || !bytes.Equal(data[:4], crypto.Keccak256([]byte("depositProjectTokens(uint256,uint256)"))[:4]) || good.calls != 3 {
		t.Fatalf("valid read-only deposit rejected: calls=%d err=%v", good.calls, err)
	}
	for name, change := range map[string]func(*v3DepositReadStub){
		"wrong chain":            func(s *v3DepositReadStub) { s.chain = 56 },
		"insufficient balance":   func(s *v3DepositReadStub) { s.balance = 9 },
		"insufficient allowance": func(s *v3DepositReadStub) { s.allowance = 9 },
		"no code":                func(s *v3DepositReadStub) { s.noCode = true },
		"changed code":           func(s *v3DepositReadStub) { s.changeCode = true },
		"malformed":              func(s *v3DepositReadStub) { s.malformed = true },
		"rpc failure":            func(s *v3DepositReadStub) { s.fail = true },
		"wrong spender":          func(s *v3DepositReadStub) { s.wrongTarget = true },
		"replacement":            func(s *v3DepositReadStub) { s.replace = true },
		"simulation revert":      func(s *v3DepositReadStub) { s.simulationRevert = true },
		"simulation nonempty":    func(s *v3DepositReadStub) { s.simulationReturn = true },
	} {
		t.Run(name, func(t *testing.T) {
			stub := newStub()
			change(stub)
			if _, err := PreviewV3ProjectDepositFundingAt(context.Background(), stub, binding, observed, owner, big.NewInt(10), codeHash); err == nil || bytes.Contains([]byte(err.Error()), []byte("sensitive provider")) {
				t.Fatalf("unsafe preflight accepted/leaked: %v", err)
			}
		})
	}
	stub := newStub()
	if _, err := PreviewV3ProjectDepositFundingAt(context.Background(), stub, binding, observed, pay, big.NewInt(10), codeHash); err == nil || stub.calls != 0 {
		t.Fatal("wrong wallet reached token RPC")
	}
	stub = newStub()
	if _, err := PreviewV3ProjectDepositFundingAt(context.Background(), stub, binding, observed, owner, big.NewInt(10), common.Hash{}); err == nil || stub.calls != 0 {
		t.Fatal("missing reviewed token code hash reached token RPC")
	}
	stub = newStub()
	if _, err := PreviewV3ProjectDepositFundingAt(context.Background(), stub, binding, observed, owner, big.NewInt(10), crypto.Keccak256Hash([]byte{0x60, 0x02})); err == nil || stub.calls != 0 {
		t.Fatal("wrong project token code identity reached allowance reads")
	}
}
