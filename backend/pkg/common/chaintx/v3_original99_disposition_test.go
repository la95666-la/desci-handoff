package chaintx

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm/schema"
	"joincare_server/pkg/common/entity"
	"math/big"
	"sync"
	"testing"
	"time"
)

type original99TestRPC struct {
	nonce                             uint64
	unknown, present, hasCode, badGas bool
	calls                             int
}

func TestOriginal99ColumnNamingMatchesMigration(t *testing.T) {
	s, e := schema.Parse(&Original99Disposition{}, &sync.Map{}, schema.NamingStrategy{})
	if e != nil {
		t.Fatal(e)
	}
	for field, col := range map[string]string{"OriginalRawSHA256": "original_raw_sha256", "OriginalID": "original_id", "TxHash": "tx_hash", "ChainID": "chain_id"} {
		if s.FieldsByName[field] == nil || s.FieldsByName[field].DBName != col {
			t.Fatal(field, s.FieldsByName[field])
		}
	}
}

func (r *original99TestRPC) ChainID(context.Context) (*big.Int, error) { return big.NewInt(56), nil }
func (r *original99TestRPC) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	return r.nonce, nil
}
func (r *original99TestRPC) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	return r.nonce, nil
}
func (r *original99TestRPC) BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	return big.NewInt(1000000000000000000), nil
}
func (r *original99TestRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	if r.unknown {
		return nil, errors.New("SECRET timeout")
	}
	return nil, ethereum.NotFound
}
func (r *original99TestRPC) TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error) {
	if r.present {
		return new(types.Transaction), true, nil
	}
	return nil, false, ethereum.NotFound
}
func (r *original99TestRPC) BlockNumber(context.Context) (uint64, error) { return 100, nil }
func (r *original99TestRPC) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	return &types.Header{Number: n, Time: uint64(time.Now().Unix())}, nil
}
func (r *original99TestRPC) CodeAt(context.Context, common.Address, *big.Int) ([]byte, error) {
	if r.hasCode {
		return []byte{1}, nil
	}
	return nil, nil
}
func (r *original99TestRPC) EstimateGas(context.Context, ethereum.CallMsg) (uint64, error) {
	if r.badGas {
		return 22000, nil
	}
	return 21000, nil
}
func (r *original99TestRPC) CallContract(_ context.Context, m ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	r.calls++
	if m.To == nil || *m.To != m.From || m.From != common.HexToAddress(original99Sender) || len(m.Data) != 0 || m.Value.Sign() != 0 || m.Gas != 21000 || m.GasPrice.Int64() != Original99DispositionGasPrice {
		return nil, errors.New("wrong exact disposition")
	}
	return nil, nil
}
func TestOriginal99DualReadChecksNeverTreatUnknownAsAbsence(t *testing.T) {
	hash := Original99Hash
	row := entity.ChainTxSubmission{TxHash: &hash}
	for _, tc := range []struct {
		name   string
		change func(*original99TestRPC)
		ok     bool
	}{
		{"exact", func(*original99TestRPC) {}, true}, {"nonce consumed", func(r *original99TestRPC) { r.nonce = 82 }, false},
		{"unknown receipt", func(r *original99TestRPC) { r.unknown = true }, false}, {"original pending", func(r *original99TestRPC) { r.present = true }, false},
		{"target not EOA", func(r *original99TestRPC) { r.hasCode = true }, false}, {"gas changed", func(r *original99TestRPC) { r.badGas = true }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &original99TestRPC{nonce: 81}
			tc.change(r)
			e := checkOriginal99Node(context.Background(), row, time.Now().Add(-time.Hour).Unix(), r)
			if (e == nil) != tc.ok {
				t.Fatal(e)
			}
		})
	}
}
func TestOriginal99DispositionCannotUseArbitrarySignature(t *testing.T) {
	d := Original99Disposition{OriginalID: 99, OriginalHash: Original99Hash, ChainID: 56, SenderAddress: original99Sender, Nonce: 81, TargetAddress: original99Sender, GasLimit: 21000, GasPriceRaw: "100000000", ValueRaw: "0", AuthorizationRef: original99Authorization, RawTx: []byte{1}}
	if verifyOriginal99Disposition(d) == nil {
		t.Fatal("malformed signed bytes accepted")
	}
	if _, e := PrepareOriginal99Disposition(nil, nil, nil, nil, false, nil); e == nil {
		t.Fatal("missing authority")
	}
	if _, e := BroadcastOriginal99Disposition(nil, nil, nil, nil, "", nil); e == nil {
		t.Fatal("missing send authority")
	}
	if _, e := ReconcileOriginal99Disposition(nil, nil, nil, nil); e == nil {
		t.Fatal("missing reconcile authority")
	}
}
