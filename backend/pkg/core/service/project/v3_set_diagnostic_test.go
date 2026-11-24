package project

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/constant"
)

func TestV3DiagnosticSetupRequiresUnassignedProjectAndExactToken(t *testing.T) {
	for _, tc := range []struct {
		name, state, version, mode, token, owner string
		locked                                   bool
		chain                                    uint64
		binding                                  any
		want                                     bool
	}{
		{"exact", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "", "", false, 0, nil, true},
		{"V2", constant.ProjectStatusSetup, constant.ContractVersionV2Stablecoin, constant.FundingModeCappedPresale, "", "", false, 0, nil, false},
		{"locked", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "", "", true, 0, nil, false},
		{"bound", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "", "", false, 0, 19, false},
		{"other token", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "0x1111111111111111111111111111111111111111", "", false, 0, nil, false},
		{"new project token", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "0x1234567890123456789012345678901234567890", "", false, 0, nil, true},
		{"other owner", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "", "0x1111111111111111111111111111111111111111", false, 0, nil, false},
		{"foreign chain", constant.ProjectStatusSetup, constant.ContractVersionUnassigned, constant.FundingModeCappedPresale, "", "", false, 56, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			mock.ExpectQuery("SELECT p.status,p.funding_mode,p.contract_version").WithArgs(int64(19)).
				WillReturnRows(sqlmock.NewRows([]string{"status", "funding_mode", "contract_version", "onchain_locked", "chain_id", "contract_address", "token_address", "owner_address", "binding_id"}).
					AddRow(tc.state, tc.mode, tc.version, tc.locked, tc.chain, "", tc.token, tc.owner, tc.binding))
			req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin, PaymentToken: "USDT", TokenAddress: v3TokenTESTCAT, OwnerAddress: v3DiagnosticOperator}
			if tc.name == "new project token" {
				req.TokenAddress = tc.token
			}
			if (v3DiagnosticSetup(database, 19, req) == nil) != tc.want {
				t.Fatal("diagnostic project identity incorrectly admitted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Reject a foreign token before even consulting the project table.
	database, mock := v2AdmissionMock(t)
	if v3DiagnosticSetup(database, 19, OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin, PaymentToken: "USDT", TokenAddress: v3TokenUSDT, OwnerAddress: v3DiagnosticOperator}) == nil {
		t.Fatal("payment token admitted as project token")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type v3DiagnosticTokenStub struct {
	token    common.Address
	proxy    common.Address
	block    *types.Header
	decimals uint64
	allowed  bool
	changed  bool
}

func (s *v3DiagnosticTokenStub) CallContext(_ context.Context, result interface{}, method string, args ...interface{}) error {
	if method != "eth_call" || len(args) != 2 || args[1] != hexutil.EncodeBig(s.block.Number) {
		return errors.New("wrong block")
	}
	call, ok := args[0].(map[string]interface{})
	if !ok || len(call) != 2 || call["to"] != s.token {
		return errors.New("wrong token")
	}
	data, ok := call["data"].(hexutil.Bytes)
	if !ok || !bytes.Equal(data, crypto.Keccak256([]byte("decimals()"))[:4]) {
		return errors.New("wrong calldata")
	}
	*result.(*hexutil.Bytes) = hexutil.Bytes(common.LeftPadBytes(new(big.Int).SetUint64(s.decimals).Bytes(), 32))
	return nil
}

func (s *v3DiagnosticTokenStub) HeaderByNumber(_ context.Context, _ *big.Int) (*types.Header, error) {
	if s.changed {
		return &types.Header{Number: s.block.Number, Extra: []byte("changed")}, nil
	}
	return s.block, nil
}

func (s *v3DiagnosticTokenStub) CallContract(_ context.Context, call ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if call.To == nil || len(call.Data) < 4 {
		return nil, errors.New("invalid call")
	}
	word := func(value uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(value).Bytes(), 32) }
	if *call.To == s.token && bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("decimals()"))[:4]) {
		return word(s.decimals), nil
	}
	if *call.To == s.proxy && bytes.Equal(call.Data[:4], crypto.Keccak256([]byte("projectTokenConfigs(address)"))[:4]) && len(call.Data) == 36 && bytes.Equal(call.Data[16:36], s.token.Bytes()) {
		allowed := uint64(0)
		if s.allowed {
			allowed = 1
		}
		return append(word(allowed), word(s.decimals)...), nil
	}
	return nil, errors.New("foreign token or method")
}

func TestV3DiagnosticChecksArbitraryConfiguredProjectToken(t *testing.T) {
	token := common.HexToAddress("0x1234567890123456789012345678901234567890")
	block := new(big.Int).SetUint64(100)
	s := &v3DiagnosticTokenStub{token: token, proxy: common.HexToAddress(v3TokenProxy), block: &types.Header{Number: block}, decimals: 6, allowed: true}
	if got, err := inspectV3DiagnosticProjectToken(context.Background(), s, s, token, block, s.block); err != nil || got != 6 {
		t.Fatalf("new token rejected: %d %v", got, err)
	}
	s.allowed = false
	if _, err := inspectV3DiagnosticProjectToken(context.Background(), s, s, token, block, s.block); err == nil {
		t.Fatal("unapproved token accepted")
	}
	s.allowed = true
	s.changed = true
	if _, err := inspectV3DiagnosticProjectToken(context.Background(), s, s, token, block, s.block); err == nil {
		t.Fatal("changed block accepted")
	}
	s.changed = false
	if _, err := inspectV3DiagnosticProjectToken(context.Background(), s, s, common.HexToAddress(v3TokenUSDT), block, s.block); err == nil {
		t.Fatal("payment token accepted as project token")
	}
}
