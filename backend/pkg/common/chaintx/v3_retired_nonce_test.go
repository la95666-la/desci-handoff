package chaintx

import (
	"context"
	"encoding/json"
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

type retiredNonceFake struct {
	tx                        *types.Transaction
	receipt                   *types.Receipt
	header, confirmed         *types.Header
	consumed, latest, pending uint64
	reads                     int
	change, reorg, wrongTx    bool
}

func (f *retiredNonceFake) ChainID(context.Context) (*big.Int, error) { return big.NewInt(31337), nil }
func (f *retiredNonceFake) BlockNumber(context.Context) (uint64, error) {
	return f.confirmed.Number.Uint64() + 14, nil
}
func (f *retiredNonceFake) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n.Cmp(f.header.Number) == 0 {
		h := types.CopyHeader(f.header)
		if f.reorg {
			h.Extra = []byte{9}
		}
		return h, nil
	}
	return types.CopyHeader(f.confirmed), nil
}
func (f *retiredNonceFake) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	f.reads++
	if f.receipt == nil {
		return nil, nil
	}
	v := *f.receipt
	if f.change && f.reads >= 2 {
		v.Status = 0
	}
	return &v, nil
}
func (f *retiredNonceFake) NonceAt(_ context.Context, _ common.Address, n *big.Int) (uint64, error) {
	if n == nil {
		return f.latest, nil
	}
	return f.consumed, nil
}
func (f *retiredNonceFake) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	return f.pending, nil
}
func (f *retiredNonceFake) TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error) {
	return f.tx, f.wrongTx, nil
}

func retiredNonceFixture(t *testing.T) (entity.ChainTxSubmission, idov3whitelist.ExpectedProjectBinding, string, *retiredNonceFake) {
	t.Helper()
	key, e := crypto.HexToECDSA(strings.Repeat("7", 64))
	if e != nil {
		t.Fatal(e)
	}
	proxy := common.HexToAddress("0x7777777777777777777777777777777777777771")
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 997, ProxyAddress: proxy, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
	call, e := basechain.PackV3Lifecycle("activateProject", big.NewInt(997))
	if e != nil {
		t.Fatal(e)
	}
	tx, e := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 7, To: &proxy, Gas: 100000, GasPrice: big.NewInt(1), Data: call}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := tx.MarshalBinary()
	if e != nil {
		t.Fatal(e)
	}
	nonce, gas, status, block, jid := uint64(7), uint64(100000), uint64(1), uint64(90), uint64(1)
	hash := tx.Hash().Hex()
	header := &types.Header{Number: big.NewInt(90), ParentHash: common.HexToHash("0xa1")}
	l := types.Log{Address: proxy, BlockNumber: 90, BlockHash: header.Hash(), TxHash: tx.Hash(), Index: 1, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), common.BigToHash(big.NewInt(997))}}
	replay, e := basechain.DecodeV3ReplayLog(l, b)
	if e != nil {
		t.Fatal(e)
	}
	payload, e := json.Marshal(replay)
	if e != nil {
		t.Fatal(e)
	}
	row := entity.ChainTxSubmission{ID: 1, ProjectID: 997, Action: "ido_v3_activate", ChainID: 31337, SenderAddress: crypto.PubkeyToAddress(key.PublicKey).Hex(), ContractAddress: strings.ToLower(proxy.Hex()), CallData: call, ValueRaw: "0", State: ChainTxStateV3ReorgSuspended, Nonce: &nonce, GasLimit: &gas, GasPriceRaw: "1", RawTx: raw, TxHash: &hash, ReceiptStatus: &status, ReceiptBlockNumber: &block, ReceiptBlockHash: header.Hash().Hex(), FinalEventRecordID: &jid}
	f := &retiredNonceFake{tx: tx, header: header, confirmed: &types.Header{Number: big.NewInt(110)}, consumed: 8, latest: 8, pending: 8, receipt: &types.Receipt{Status: 1, TxHash: tx.Hash(), BlockNumber: big.NewInt(90), BlockHash: header.Hash(), Logs: []*types.Log{&l}}}
	return row, b, string(payload), f
}

func TestRetiredNonceAuditOnlyAcceptsExactConsumedOriginalSuccess(t *testing.T) {
	for _, name := range []string{"success", "missing", "receipt-change", "reorg", "unconsumed", "pending-drift", "pending-tx", "raw-change", "unsigned", "failed-receipt", "payload-change", "wrong-binding", "nonce-reused"} {
		t.Run(name, func(t *testing.T) {
			row, b, payload, f := retiredNonceFixture(t)
			before := row.State
			switch name {
			case "missing":
				f.receipt = nil
			case "receipt-change":
				f.change = true
			case "reorg":
				f.reorg = true
			case "unconsumed":
				f.consumed = 7
			case "pending-drift":
				f.pending = 9
			case "pending-tx":
				f.wrongTx = true
			case "raw-change":
				row.RawTx = []byte{1}
			case "unsigned":
				row.State = "ready"
			case "failed-receipt":
				f.receipt.Status = 0
			case "payload-change":
				payload = `{}`
			case "wrong-binding":
				b.ProjectToken = b.PaymentToken
			case "nonce-reused":
				v := uint64(6)
				row.Nonce = &v
			}
			proof, e := auditRetiredNonce(context.Background(), f, row, b, payload)
			if (e == nil) != (name == "success") {
				t.Fatalf("audit %+v err=%v", proof, e)
			}
			if name == "success" && (proof.Nonce != 7 || proof.ConsumedNonce != 8 || proof.SubmissionID != 1 || len(proof.EvidenceDigest) != 64 || row.State != before) {
				t.Fatal("audit mutated row or lost exact binding")
			}
		})
	}
}

func TestRetiredNonceAuditCanceledDoesNotReturnProof(t *testing.T) {
	row, b, payload, f := retiredNonceFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := auditRetiredNonce(ctx, f, row, b, payload); e == nil {
		t.Fatal("canceled audit accepted")
	}
}
