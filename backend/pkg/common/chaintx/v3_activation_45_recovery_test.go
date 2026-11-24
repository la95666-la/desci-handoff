package chaintx

import (
	"context"
	"errors"
	"fmt"
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

func signedActivation45Fixture(t *testing.T) (entity.ChainTxSubmission, idov3whitelist.ExpectedProjectBinding, common.Hash, []byte) {
	t.Helper()
	private, err := crypto.HexToECDSA(strings.Repeat("1", 64)) // synthetic key, never production custody
	if err != nil {
		t.Fatal(err)
	}
	proxy := common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: 43235577608273921,
		ProxyAddress: proxy, ProjectToken: common.HexToAddress("0x1111111111111111111111111111111111111111"),
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	data, err := basechain.PackV3Lifecycle("activateProject", new(big.Int).SetUint64(binding.ProjectID))
	if err != nil {
		t.Fatal(err)
	}
	gas, price, nonce := uint64(74315), big.NewInt(50000000), uint64(39)
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: nonce, Gas: gas, GasPrice: price, To: &proxy, Value: big.NewInt(0), Data: data}),
		types.NewEIP155Signer(big.NewInt(56)), private)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	hash, slot := tx.Hash().Hex(), uint8(1)
	row := entity.ChainTxSubmission{ID: 45, ProjectID: binding.ProjectID, Action: "ido_v3_activate", State: "signed",
		ChainID: 56, ContractAddress: strings.ToLower(proxy.Hex()), SenderAddress: crypto.PubkeyToAddress(private.PublicKey).Hex(),
		Nonce: &nonce, GasLimit: &gas, GasPriceRaw: price.String(), ValueRaw: "0", RawTx: raw,
		CallData: data, TxHash: &hash, ActiveSlot: &slot}
	return row, binding, tx.Hash(), data
}

func TestActivation45OriginalSignedByteGuards(t *testing.T) {
	row, binding, hash, data := signedActivation45Fixture(t)
	sender := common.HexToAddress(row.SenderAddress)
	if err := verifyActivation45SignedRowForSender(row, binding, data, hash, sender, new(types.Transaction)); err != nil {
		t.Fatalf("valid synthetic original rejected: %v", err)
	}
	// A synthetic signature must never pass the fixed production wrapper.
	if err := verifyActivation45SignedRow(row, binding, data, hash, new(types.Transaction)); err == nil {
		t.Fatal("foreign signer admitted")
	}
	for name, change := range map[string]func(*entity.ChainTxSubmission){
		"different id":       func(v *entity.ChainTxSubmission) { v.ID++ },
		"second send":        func(v *entity.ChainTxSubmission) { v.BroadcastCount++ },
		"not signed":         func(v *entity.ChainTxSubmission) { v.State = "broadcast_unknown" },
		"different nonce":    func(v *entity.ChainTxSubmission) { n := uint64(40); v.Nonce = &n },
		"no slot":            func(v *entity.ChainTxSubmission) { v.ActiveSlot = nil },
		"receipt present":    func(v *entity.ChainTxSubmission) { status := uint64(1); v.ReceiptStatus = &status },
		"different calldata": func(v *entity.ChainTxSubmission) { v.CallData = append([]byte(nil), data...); v.CallData[0] ^= 1 },
		"different bytes": func(v *entity.ChainTxSubmission) {
			v.RawTx = append([]byte(nil), v.RawTx...)
			v.RawTx[len(v.RawTx)-1] ^= 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := row
			change(&changed)
			if err := verifyActivation45SignedRowForSender(changed, binding, data, hash, sender, new(types.Transaction)); err == nil {
				t.Fatal("changed original accepted")
			}
		})
	}
}

func TestActivation45MarkedSendNeverRetries(t *testing.T) {
	hash := common.HexToHash("0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := 0
	send := func() error { called++; return errors.New("ambiguous provider result") }
	if sendMarkedActivation45(ctx, hash, send) == nil || called != 0 {
		t.Fatal("cancelled send attempted")
	}
	if sendMarkedActivation45(context.Background(), common.HexToHash("0x01"), send) == nil || called != 0 {
		t.Fatal("different hash sent")
	}
	if sendMarkedActivation45(context.Background(), hash, send) == nil || called != 1 {
		t.Fatal("ambiguous result retried or treated as success")
	}
}

func TestActivation45FailureStageNeverEchoesProviderOrUnknownValues(t *testing.T) {
	for _, phase := range []activation45StageError{"identity", "creation_admission", "reader_chain", "sender_nonce",
		"receipt_absent", "sender_transaction_absent", "confirmed_header", "governance", "project_state",
		"creation_economics", "activation_plan", "creation_recheck", "simulation", "simulation_header",
		"reorg_cursor", "project_lock", "signed_original", "nonce_fence", "marker_commit", "original_send"} {
		if got := Recovery45FailureStage(fmt.Errorf("wrapped: %w", phase)); got != string(phase) {
			t.Fatalf("phase=%q returned=%q", phase, got)
		}
	}
	for _, err := range []error{errors.New("secret RPC URL"), activation45StageError("injected_secret"), nil} {
		if got := Recovery45FailureStage(err); got != "unavailable" {
			t.Fatalf("unknown error leaked: %q", got)
		}
	}
}
