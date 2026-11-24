package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
)

func TestV3TokenAdmissionRejectsUnconfiguredRPCsAndForeignTokens(t *testing.T) {
	old := viper.Get("idoV3TokenAdmission.enabled")
	viper.Set("idoV3TokenAdmission.enabled", false)
	t.Cleanup(func() { viper.Set("idoV3TokenAdmission.enabled", old) })
	if _, err := v3TokenGateRPC(context.Background()); err == nil {
		t.Fatal("unconfigured admission read gate accepted")
	}
	if _, _, _, _, _, err := v3TokenAsset("usdc"); err == nil {
		t.Fatal("USDC is not in the V3 test-token allowlist")
	}
	for _, value := range []string{"", "0x", "0x" + strings.Repeat("g", 64), strings.Repeat("a", 64)} {
		if _, err := VerifyV3TokenGateReceipt("usdt", value); err == nil {
			t.Fatalf("bad hash accepted: %q", value)
		}
	}
}

// Opt-in read-only production-shaped probe. The caller supplies the private
// commercial endpoint in a file; neither its URL nor RPC errors are logged.
func TestV3TokenAdmissionLiveReadOnly(t *testing.T) {
	path := os.Getenv("JC_V3_RPC_FILE")
	if path == "" {
		t.Skip("set JC_V3_RPC_FILE for the optional read-only RPC probe")
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("private RPC file unavailable")
	}
	for _, key := range []string{"enabled", "primaryRPC", "ownerSafeCodeHash"} {
		name := "idoV3TokenAdmission." + key
		old := viper.Get(name)
		t.Cleanup(func() { viper.Set(name, old) })
	}
	viper.Set("idoV3TokenAdmission.enabled", true)
	viper.Set("idoV3TokenAdmission.primaryRPC", strings.TrimSpace(string(secret)))
	viper.Set("idoV3TokenAdmission.ownerSafeCodeHash", "0xd7d408ebcd99b2b70be43e20253d6d92a8ea8fab29bd3be7f55b10032331fb4c")
	started := time.Now()
	status, err := ReadV3TokenGateStatus()
	if err != nil {
		t.Fatalf("read-only preflight rejected after %s: %v", time.Since(started).Round(time.Millisecond), err)
	}
	t.Logf("read-only preflight passed in %s, block=%d, usdt=%t, testcat=%t", time.Since(started).Round(time.Millisecond), status.Block, status.USDT.Allowed, status.TESTCAT.Allowed)
}

// Optional original-hash readback of the executed TESTDOG Owner Safe call.
// This never signs, proposes, broadcasts or modifies the token configuration.
func TestV3ProjectTokenExecutedReceiptLiveReadOnly(t *testing.T) {
	path := os.Getenv("JC_V3_RPC_FILE")
	if path == "" {
		t.Skip("set JC_V3_RPC_FILE for the optional read-only receipt probe")
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("private commercial RPC file unavailable")
	}
	for _, key := range []string{"enabled", "primaryRPC", "ownerSafeCodeHash"} {
		name := "idoV3TokenAdmission." + key
		old := viper.Get(name)
		t.Cleanup(func() { viper.Set(name, old) })
	}
	viper.Set("idoV3TokenAdmission.enabled", true)
	viper.Set("idoV3TokenAdmission.primaryRPC", strings.TrimSpace(string(secret)))
	viper.Set("idoV3TokenAdmission.ownerSafeCodeHash", "0xd7d408ebcd99b2b70be43e20253d6d92a8ea8fab29bd3be7f55b10032331fb4c")
	const token = "0x5aeeC1e427586922D676f08B453ce692B13C7eD4"
	const originalTx = "0x59e1b4ef65709fc014f79d98c4bae2dfee845bdd91fb7c793a40eb7453f6b0d2"
	result, err := VerifyV3ProjectTokenReceipt(token, originalTx)
	if err != nil || result == nil || result.TxHash != originalTx || !result.Allowed || result.Decimals != 18 || result.Confirmations < 15 {
		t.Fatalf("executed Owner Safe receipt was not verified: %v", err)
	}
}

// Optional diagnostic for a repeatedly failing production-shaped decimals()
// read. Never print the RPC URL, raw error, or credentials in a test result.
func TestV3USDTDecimalsLiveReadOnly(t *testing.T) {
	path := os.Getenv("JC_V3_RPC_FILE")
	if path == "" {
		t.Skip("set JC_V3_RPC_FILE for the optional read-only RPC probe")
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("private RPC file unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	rpc, err := ethclient.DialContext(ctx, strings.TrimSpace(string(secret)))
	if err != nil {
		t.Fatal("commercial RPC connection unavailable")
	}
	defer rpc.Close()
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || head.Number.Uint64() < 15 {
		t.Fatal("confirmed head unavailable")
	}
	block := new(big.Int).SetUint64(head.Number.Uint64() - 14)
	token := common.HexToAddress(v3TokenUSDT)
	for i := 0; i < 8; i++ {
		start := time.Now()
		value, callErr := readV3TokenDecimals(ctx, rpc.Client(), token, block)
		if callErr != nil {
			t.Fatalf("attempt=%d error=%s elapsed_ms=%d", i+1, callErr.Error(), time.Since(start).Milliseconds())
		}
		t.Logf("attempt=%d decimals=%d elapsed_ms=%d", i+1, value, time.Since(start).Milliseconds())
	}
}

type v3DecimalsRPCStub struct {
	calls     int
	wantToken common.Address
	wantBlock string
	responses []struct {
		data hexutil.Bytes
		err  error
	}
	wrongShape bool
}

func (s *v3DecimalsRPCStub) CallContext(_ context.Context, result interface{}, method string, args ...interface{}) error {
	s.calls++
	if method != "eth_call" || len(args) != 2 || args[1] != s.wantBlock {
		s.wrongShape = true
	}
	call, ok := args[0].(map[string]interface{})
	if !ok || len(call) != 2 || call["to"] != s.wantToken {
		s.wrongShape = true
	} else if payload, valid := call["data"].(hexutil.Bytes); !valid || !bytes.Equal(payload, []byte{0x31, 0x3c, 0xe5, 0x67}) {
		s.wrongShape = true
	}
	response := s.responses[s.calls-1]
	if response.err != nil {
		return response.err
	}
	*result.(*hexutil.Bytes) = response.data
	return nil
}

func TestV3TokenDecimalsReadShapeAndFailureModes(t *testing.T) {
	word := func(n byte) hexutil.Bytes { b := make(hexutil.Bytes, 32); b[31] = n; return b }
	token := common.HexToAddress(v3TokenUSDT)
	block := big.NewInt(129092646)
	for _, tc := range []struct {
		name      string
		responses []struct {
			data hexutil.Bytes
			err  error
		}
		calls int
		want  string
	}{
		{"valid", []struct {
			data hexutil.Bytes
			err  error
		}{{word(18), nil}}, 1, ""},
		{"read-recovered", []struct {
			data hexutil.Bytes
			err  error
		}{{nil, errors.New("secret provider detail")}, {word(18), nil}}, 2, ""},
		{"read-still-unavailable", []struct {
			data hexutil.Bytes
			err  error
		}{{nil, errors.New("secret provider detail")}, {nil, errors.New("secret provider detail")}}, 2, "V3 token decimals read unavailable"},
		{"malformed-not-retried", []struct {
			data hexutil.Bytes
			err  error
		}{{word(18)[:31], nil}}, 1, "V3 token decimals result malformed"},
		{"wrong-value-not-retried", []struct {
			data hexutil.Bytes
			err  error
		}{{word(6), nil}}, 1, "V3 token decimals mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &v3DecimalsRPCStub{wantToken: token, wantBlock: hexutil.EncodeBig(block), responses: tc.responses}
			value, err := readV3TokenDecimals(context.Background(), stub, token, block)
			if stub.wrongShape || stub.calls != tc.calls {
				t.Fatalf("request shape or count: shape=%t calls=%d", stub.wrongShape, stub.calls)
			}
			if tc.want == "" {
				if err != nil || value != 18 {
					t.Fatalf("value=%d err=%v", value, err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("value=%d err=%v, want=%s", value, err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stub := &v3DecimalsRPCStub{wantToken: token, wantBlock: hexutil.EncodeBig(block), responses: []struct {
		data hexutil.Bytes
		err  error
	}{{nil, context.Canceled}}}
	if _, err := readV3TokenDecimals(ctx, stub, token, block); err == nil || stub.calls != 1 {
		t.Fatalf("cancelled read retried or accepted: calls=%d err=%v", stub.calls, err)
	}
}

func TestV3TokenDecimalsEmitsCanonicalRPCJSON(t *testing.T) {
	token := common.HexToAddress(v3TokenUSDT)
	block := big.NewInt(129092646)
	observed := make(chan []byte, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Method != "eth_call" || len(request.Params) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		observed <- append(append([]byte(nil), request.Params[0]...), '\n')
		observed <- append(append([]byte(nil), request.Params[1]...), '\n')
		w.Header().Set("Content-Type", "application/json")
		response := struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  string          `json:"result"`
		}{JSONRPC: "2.0", ID: request.ID, Result: hexutil.Encode(common.LeftPadBytes([]byte{18}, 32))}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	value, err := readV3TokenDecimals(ctx, client.Client(), token, block)
	if err != nil || value != 18 {
		t.Fatalf("decimals=%d err=%v", value, err)
	}
	var call map[string]any
	if err := json.Unmarshal(<-observed, &call); err != nil {
		t.Fatal(err)
	}
	if len(call) != 2 || call["to"] != strings.ToLower(token.Hex()) || call["data"] != "0x313ce567" {
		t.Fatalf("unexpected eth_call fields: %v", call)
	}
	var tag string
	if err := json.Unmarshal(<-observed, &tag); err != nil || tag != hexutil.EncodeBig(block) {
		t.Fatalf("fixed block tag=%q err=%v", tag, err)
	}
}

func TestV3TokenAdmissionPinsBothTokenAddressesWithoutHash(t *testing.T) {
	for _, tc := range []struct {
		symbol  string
		address string
	}{
		{"usdt", v3TokenUSDT},
		{"testcat", v3TokenTESTCAT},
	} {
		t.Run(tc.symbol, func(t *testing.T) {
			token, _, hash, _, _, err := v3TokenAsset(tc.symbol)
			if err != nil || token != common.HexToAddress(tc.address) || hash != (common.Hash{}) {
				t.Fatalf("token identity: %v", err)
			}
			if err := verifyV3AssetAddress(tc.symbol, token, hash); err != nil {
				t.Fatal(err)
			}
			if err := verifyV3AssetAddress(tc.symbol, common.HexToAddress(v3TokenProxy), hash); err == nil {
				t.Fatal("foreign token address accepted")
			}
			if err := verifyV3AssetAddress(tc.symbol, token, crypto.Keccak256Hash([]byte{1})); err == nil {
				t.Fatal("unexpected token hash policy accepted")
			}
		})
	}
	if err := verifyV3AssetAddress("usdc", common.HexToAddress(v3TokenUSDT), common.Hash{}); err == nil {
		t.Fatal("unsupported token accepted")
	}
}

func TestV3TokenAdmissionRequiresOnlyApprovedCommercialReadProvider(t *testing.T) {
	for _, key := range []string{"enabled", "primaryRPC", "secondaryRPC"} {
		name := "idoV3TokenAdmission." + key
		old := viper.Get(name)
		t.Cleanup(func() { viper.Set(name, old) })
	}
	viper.Set("idoV3TokenAdmission.enabled", true)
	for _, primary := range []string{
		"", "https://bsc-dataseed2.ninicoin.io", "http://silent-bold-vineyard.bsc.quiknode.pro/private",
		"https://user:password@silent-bold-vineyard.bsc.quiknode.pro/private",
		"https://silent-bold-vineyard.bsc.quiknode.pro/",
		"https://silent-bold-vineyard.bsc.quiknode.pro/private?unexpected=1",
	} {
		viper.Set("idoV3TokenAdmission.primaryRPC", primary)
		viper.Set("idoV3TokenAdmission.secondaryRPC", "https://bsc-dataseed2.ninicoin.io")
		if client, err := v3TokenGateRPC(context.Background()); err == nil {
			client.Close()
			t.Fatalf("untrusted provider accepted: %q", primary)
		}
	}
	// A configured secondary endpoint is deliberately ignored, even when it
	// is invalid. This tests configuration selection without dialing a network.
	viper.Set("idoV3TokenAdmission.primaryRPC", "https://silent-bold-vineyard.bsc.quiknode.pro/private-token/")
	viper.Set("idoV3TokenAdmission.secondaryRPC", "http://untrusted.example/")
	endpoint, err := v3TokenGateEndpoint()
	if err != nil || endpoint != "https://silent-bold-vineyard.bsc.quiknode.pro/private-token/" {
		t.Fatal("the approved commercial endpoint was not selected alone")
	}
}

func TestV3TokenCallDataMatchesReviewedSelectors(t *testing.T) {
	for _, tc := range []struct{ asset, method, selector, token string }{
		{"usdt", "configurePaymentToken(address,bool)", "2cb96e4f", v3TokenUSDT},
		{"testcat", "configureProjectToken(address,bool)", "82f30c7c", v3TokenTESTCAT},
	} {
		t.Run(tc.asset, func(t *testing.T) {
			token, _, _, method, _, err := v3TokenAsset(tc.asset)
			if err != nil || token != common.HexToAddress(tc.token) || method != tc.method {
				t.Fatalf("asset identity: %v", err)
			}
			data := v3TokenCallData(method, &token, true)
			if len(data) != 68 || common.Bytes2Hex(data[:4]) != tc.selector ||
				!bytes.Equal(data[4:36], common.LeftPadBytes(token.Bytes(), 32)) ||
				new(big.Int).SetBytes(data[36:]).Cmp(big.NewInt(1)) != 0 {
				t.Fatal("Safe inner call does not exactly configure selected token to true")
			}
		})
	}
}

func v3TokenTestTx(t *testing.T, value *big.Int, inner []byte) *types.Transaction {
	return v3TokenTestTxAtSafe(t, value, inner, common.HexToAddress(v3TokenOwnerSafe))
}

func v3TokenTestTxAtSafe(t *testing.T, value *big.Int, inner []byte, safe common.Address) *types.Transaction {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(v3TokenSafeABI))
	if err != nil {
		t.Fatal(err)
	}
	data, err := parsed.Pack("execTransaction", common.HexToAddress(v3TokenProxy), value, inner,
		uint8(0), new(big.Int), new(big.Int), new(big.Int), common.Address{}, common.Address{}, make([]byte, 130))
	if err != nil {
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	tx := types.NewTx(&types.LegacyTx{Nonce: 1, To: ptrAddress(safe), Gas: 200000,
		GasPrice: big.NewInt(1), Data: data})
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(56)), key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func ptrAddress(a common.Address) *common.Address { return &a }

func TestV3TokenReceiptRequiresExactSafeExecutionAndEvent(t *testing.T) {
	token, _, _, method, event, err := v3TokenAsset("usdt")
	if err != nil {
		t.Fatal(err)
	}
	inner := v3TokenCallData(method, &token, true)
	tx := v3TokenTestTx(t, big.NewInt(0), inner)
	blockHash := crypto.Keccak256Hash([]byte("canonical-block"))
	block := uint64(200)
	success := &types.Log{Address: common.HexToAddress(v3TokenOwnerSafe), Topics: []common.Hash{v3TokenSafeSuccess},
		Data: make([]byte, 64), TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: block}
	copy(success.Data[:32], crypto.Keccak256([]byte("safe-transaction")))
	configured := &types.Log{Address: common.HexToAddress(v3TokenProxy),
		Topics: []common.Hash{crypto.Keccak256Hash([]byte(event)), common.BytesToHash(common.LeftPadBytes(token.Bytes(), 32))},
		Data:   append(make([]byte, 31), 1), TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: block}
	configured.Data = append(configured.Data, append(make([]byte, 31), 18)...)
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, TxHash: tx.Hash(), BlockHash: blockHash,
		BlockNumber: new(big.Int).SetUint64(block), Logs: []*types.Log{success, configured}}
	if err := verifyV3TokenSafeTransaction(tx, receipt, token, method, event); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		tx      *types.Transaction
		receipt *types.Receipt
	}{
		{"wrong-inner-call", v3TokenTestTx(t, big.NewInt(0), v3TokenCallData(method, &token, false)), nil},
		{"wrong-token", v3TokenTestTx(t, big.NewInt(0), v3TokenCallData(method, ptrAddress(common.HexToAddress(v3TokenTESTCAT)), true)), nil},
		{"nonzero-inner-value", v3TokenTestTx(t, big.NewInt(1), inner), nil},
		{"no-event", tx, &types.Receipt{Status: 1, TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: big.NewInt(200), Logs: []*types.Log{success}}},
		{"no-safe-success", tx, &types.Receipt{Status: 1, TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: big.NewInt(200), Logs: []*types.Log{configured}}},
		{"duplicate-token-event", tx, &types.Receipt{Status: 1, TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: big.NewInt(200), Logs: []*types.Log{success, configured, configured}}},
		{"receipt-revert", tx, &types.Receipt{Status: 0, TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: big.NewInt(200), Logs: []*types.Log{success, configured}}},
		{"empty-safe-hash", tx, &types.Receipt{Status: 1, TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: big.NewInt(200), Logs: []*types.Log{{Address: common.HexToAddress(v3TokenOwnerSafe), Topics: []common.Hash{v3TokenSafeSuccess}, Data: make([]byte, 64), TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: block}, configured}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := tc.receipt
			if candidate == nil {
				candidate = &types.Receipt{Status: 1, TxHash: tc.tx.Hash(), BlockHash: blockHash, BlockNumber: big.NewInt(200), Logs: []*types.Log{success, configured}}
			}
			if err := verifyV3TokenSafeTransaction(tc.tx, candidate, token, method, event); err == nil {
				t.Fatal("invalid Safe receipt accepted")
			}
		})
	}
}

func TestV3NewProjectTokenReceiptBindsAddressAndActualDecimals(t *testing.T) {
	token := common.HexToAddress("0x1234567890123456789012345678901234567890")
	method, event := "configureProjectToken(address,bool)", "ProjectTokenConfigured(address,bool,uint8)"
	tx := v3TokenTestTx(t, big.NewInt(0), v3TokenCallData(method, &token, true))
	blockHash := crypto.Keccak256Hash([]byte("new-project-token-block"))
	block := uint64(205)
	success := &types.Log{Address: common.HexToAddress(v3TokenOwnerSafe), Topics: []common.Hash{v3TokenSafeSuccess},
		Data: make([]byte, 64), TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: block}
	copy(success.Data[:32], crypto.Keccak256([]byte("safe-tx")))
	configured := &types.Log{Address: common.HexToAddress(v3TokenProxy),
		Topics: []common.Hash{crypto.Keccak256Hash([]byte(event)), common.BytesToHash(common.LeftPadBytes(token.Bytes(), 32))},
		Data:   append(common.LeftPadBytes([]byte{1}, 32), common.LeftPadBytes([]byte{6}, 32)...),
		TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: block}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, TxHash: tx.Hash(), BlockHash: blockHash,
		BlockNumber: new(big.Int).SetUint64(block), Logs: []*types.Log{success, configured}}
	if err := verifyV3TokenSafeTransactionDecimals(tx, receipt, token, method, event, 6); err != nil {
		t.Fatalf("valid 6-decimal project token rejected: %v", err)
	}
	// The deployed Owner Safe indexes the Safe tx hash; the 32-byte data is
	// payment. Preserve exact original tx/event/decimals checks for this shape.
	indexed := *success
	indexed.Topics = []common.Hash{v3TokenSafeSuccess, common.BytesToHash(success.Data[:32])}
	indexed.Data = make([]byte, 32)
	indexedReceipt := *receipt
	indexedReceipt.Logs = []*types.Log{&indexed, configured}
	if err := verifyV3TokenSafeTransactionDecimals(tx, &indexedReceipt, token, method, event, 6); err != nil {
		t.Fatalf("deployed Safe indexed success rejected: %v", err)
	}
	for _, bad := range []struct {
		name   string
		topics []common.Hash
		data   []byte
	}{
		{"missing indexed hash", []common.Hash{v3TokenSafeSuccess, {}}, make([]byte, 32)},
		{"nonzero payment", indexed.Topics, append(make([]byte, 31), 1)},
		{"extra indexed topic", append(append([]common.Hash{}, indexed.Topics...), common.Hash{}), make([]byte, 32)},
	} {
		t.Run(bad.name, func(t *testing.T) {
			event := indexed
			event.Topics, event.Data = bad.topics, bad.data
			candidate := indexedReceipt
			candidate.Logs = []*types.Log{&event, configured}
			if err := verifyV3TokenSafeTransactionDecimals(tx, &candidate, token, method, "ProjectTokenConfigured(address,bool,uint8)", 6); err == nil {
				t.Fatal("malformed indexed Safe event accepted")
			}
		})
	}
	if err := verifyV3TokenSafeTransactionDecimals(tx, receipt, token, method, event, 18); err == nil {
		t.Fatal("different decimals accepted")
	}
	if err := verifyV3TokenSafeTransactionDecimals(tx, receipt, common.HexToAddress(v3TokenTESTCAT), method, event, 6); err == nil {
		t.Fatal("different project token accepted")
	}
}

func TestV3ProjectTokenDecimalsAtConfirmedBlock(t *testing.T) {
	token := common.HexToAddress("0x1234567890123456789012345678901234567890")
	block := big.NewInt(200)
	word := func(n byte) hexutil.Bytes { data := make(hexutil.Bytes, 32); data[31] = n; return data }
	for _, tc := range []struct {
		name string
		data hexutil.Bytes
		err  error
		want uint8
	}{
		{"six decimals", word(6), nil, 6},
		{"eighteen decimals", word(18), nil, 18},
		{"zero decimals", word(0), nil, 0},
		{"too many decimals", word(19), nil, 0},
		{"malformed", word(6)[:31], nil, 0},
		{"provider unavailable", nil, errors.New("sensitive provider URL"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &v3DecimalsRPCStub{wantToken: token, wantBlock: hexutil.EncodeBig(block), responses: []struct {
				data hexutil.Bytes
				err  error
			}{{tc.data, tc.err}}}
			got, err := readV3ProjectTokenDecimals(context.Background(), stub, token, block)
			if stub.calls != 1 || stub.wrongShape || (err == nil) != (tc.want != 0) || (err == nil && got != tc.want) {
				t.Fatalf("shape=%t calls=%d value=%d error=%v", stub.wrongShape, stub.calls, got, err)
			}
			if err != nil && strings.Contains(err.Error(), "sensitive") {
				t.Fatal("RPC credential leaked")
			}
		})
	}
}
