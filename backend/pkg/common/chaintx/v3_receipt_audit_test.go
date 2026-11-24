package chaintx

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

type v3ReceiptFake struct {
	receipts []*types.Receipt
	head     *types.Header
	block    uint64
	chain    uint64
	err      error
	calls    int
}

func (f *v3ReceiptFake) ChainID(context.Context) (*big.Int, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.chain == 0 {
		return big.NewInt(31337), nil
	}
	return new(big.Int).SetUint64(f.chain), nil
}

func (f *v3ReceiptFake) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if len(f.receipts) == 0 {
		return nil, nil
	}
	i := f.calls - 1
	if i >= len(f.receipts) {
		i = len(f.receipts) - 1
	}
	return f.receipts[i], nil
}
func (f *v3ReceiptFake) HeaderByNumber(_ context.Context, _ *big.Int) (*types.Header, error) {
	return f.head, f.err
}
func (f *v3ReceiptFake) BlockNumber(context.Context) (uint64, error) { return f.block, f.err }

func TestAuditV3PersistedReceiptNeverBroadcastsOrMutates(t *testing.T) {
	key, err := crypto.HexToECDSA(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	client, err := basechain.NewClient("http://127.0.0.1:1", key, to, big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	data, err := basechain.PackV3Lifecycle("activateProject", big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	raw, hash, err := client.SignLegacyTransaction(7, 90000, big.NewInt(1), to, data)
	if err != nil {
		t.Fatal(err)
	}
	nonce := uint64(7)
	slot := uint8(1)
	row := entity.ChainTxSubmission{ProjectID: 42, Action: "ido_v3_activate", ChainID: 31337, SenderAddress: client.Address.Hex(), ContractAddress: to.Hex(), CallData: data, RawTx: raw, TxHash: &hash, Nonce: &nonce, ActiveSlot: &slot}
	binding := idov3whitelist.ExpectedProjectBinding{ProjectID: 42, ChainID: 31337, ProxyAddress: to,
		ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333"),
		PaymentToken: common.HexToAddress("0x4444444444444444444444444444444444444444")}
	header := &types.Header{Number: big.NewInt(42), Time: 1}
	receipt := &types.Receipt{TxHash: common.HexToHash(hash), BlockHash: header.Hash(), BlockNumber: big.NewInt(42), Status: types.ReceiptStatusSuccessful}
	for _, tc := range []struct {
		name     string
		f        v3ReceiptFake
		minimum  uint64
		ok       bool
		reverted bool
	}{
		{"success", v3ReceiptFake{receipts: []*types.Receipt{receipt}, head: header, block: 44}, 3, true, false},
		{"revert", v3ReceiptFake{receipts: []*types.Receipt{v3ReceiptWithStatus(receipt, 0)}, head: header, block: 44}, 3, true, true},
		{"not confirmed", v3ReceiptFake{receipts: []*types.Receipt{receipt}, head: header, block: 43}, 3, false, false},
		{"missing", v3ReceiptFake{head: header, block: 44}, 3, false, false},
		{"unknown", v3ReceiptFake{err: errors.New("provider failed")}, 3, false, false},
		{"wrong chain", v3ReceiptFake{chain: 56, receipts: []*types.Receipt{receipt}, head: header, block: 44}, 3, false, false},
		{"reorg", v3ReceiptFake{receipts: []*types.Receipt{receipt}, head: &types.Header{Number: big.NewInt(42), Time: 2}, block: 44}, 3, false, false},
		{"receipt changed", v3ReceiptFake{receipts: []*types.Receipt{receipt, v3ReceiptWithStatus(receipt, 0)}, head: header, block: 44}, 3, false, false},
		{"zero confirmations", v3ReceiptFake{receipts: []*types.Receipt{receipt}, head: header, block: 44}, 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := AuditV3PersistedReceipt(context.Background(), row, binding, &tc.f, tc.minimum)
			if (e == nil) != tc.ok {
				t.Fatalf("audit=%+v err=%v", got, e)
			}
			if tc.ok && (got.Hash != common.HexToHash(hash) || got.Block != 42 || got.Nonce != 7 || got.Reverted != tc.reverted) {
				t.Fatalf("audit=%+v", got)
			}
		})
	}
	invalid := row
	invalid.CallData = []byte{9}
	f := &v3ReceiptFake{}
	if _, e := AuditV3PersistedReceipt(context.Background(), invalid, binding, f, 1); e == nil || f.calls != 0 {
		t.Fatalf("invalid raw reached RPC: err=%v calls=%d", e, f.calls)
	}
	unknown := row
	unknown.Action = "ido_v3_future"
	if _, e := AuditV3PersistedReceipt(context.Background(), unknown, binding, f, 1); e == nil || f.calls != 0 {
		t.Fatalf("unknown V3 action reached RPC: err=%v calls=%d", e, f.calls)
	}
	foreign := binding
	foreign.ProxyAddress = common.HexToAddress("0x5555555555555555555555555555555555555555")
	if _, e := AuditV3PersistedReceipt(context.Background(), row, foreign, f, 1); e == nil || f.calls != 0 {
		t.Fatalf("foreign proxy reached receipt RPC: err=%v calls=%d", e, f.calls)
	}
}

func v3ReceiptWithStatus(r *types.Receipt, status uint64) *types.Receipt {
	copy := *r
	copy.Status = status
	return &copy
}
