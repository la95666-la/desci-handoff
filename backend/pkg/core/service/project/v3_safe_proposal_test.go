package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestV3SafeProposalHashAndOwnerSignature(t *testing.T) {
	safe := common.HexToAddress(v3TokenOwnerSafe)
	target := common.HexToAddress(v3TokenProxy)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	owner := crypto.PubkeyToAddress(key.PublicKey)
	hash := common.HexToHash("0x" + strings.Repeat("ab", 32))
	server := rpc.NewServer()
	if err := server.RegisterName("eth", &v3SafeRPCStub{t: t, safe: safe, target: target, owner: owner, hash: hash}); err != nil {
		t.Fatal(err)
	}
	client := ethclient.NewClient(rpc.DialInProc(server))
	defer client.Close()
	data := v3TokenCallData("configurePaymentToken(address,bool)", &target, true)
	p, err := prepareV3SafeProposal(context.Background(), client, safe, target, data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Nonce != "7" || p.SafeTxHash != hash.Hex() {
		t.Fatalf("unexpected proposal: %+v", p)
	}
	digest := crypto.Keccak256([]byte("\x19Ethereum Signed Message:\n32"), hash.Bytes())
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		t.Fatal(err)
	}
	sig[64] += 27
	encoded, err := v3SafeOwnerSignature(context.Background(), client, p, owner.Hex(), "0x"+common.Bytes2Hex(sig))
	if err != nil || !strings.HasSuffix(encoded, "1f") && !strings.HasSuffix(encoded, "20") {
		t.Fatalf("signature: %s %v", encoded, err)
	}
	if _, err := v3SafeOwnerSignature(context.Background(), client, p, safe.Hex(), "0x"+common.Bytes2Hex(sig)); err == nil {
		t.Fatal("non-owner accepted")
	}
	if _, err := prepareV3SafeProposal(context.Background(), client, safe, owner, data); err == nil {
		t.Fatal("foreign target accepted")
	}
}

type v3SafeRPCStub struct {
	t                   *testing.T
	safe, target, owner common.Address
	hash                common.Hash
}

func (s *v3SafeRPCStub) ChainId(context.Context) (string, error) { return "0x38", nil }
func (s *v3SafeRPCStub) Call(_ context.Context, call map[string]any, block string) (string, error) {
	if !strings.EqualFold(call["to"].(string), s.safe.Hex()) {
		s.t.Fatal("wrong Safe")
	}
	encoded, ok := call["input"].(string)
	if !ok {
		encoded, ok = call["data"].(string)
	}
	if !ok {
		s.t.Fatalf("missing input: %v", call)
	}
	input := common.FromHex(encoded)
	if len(input) < 4 {
		s.t.Fatal("short call")
	}
	switch common.Bytes2Hex(input[:4]) {
	case common.Bytes2Hex(crypto.Keccak256([]byte("nonce()"))[:4]):
		return "0x" + common.Bytes2Hex(common.LeftPadBytes(big.NewInt(7).Bytes(), 32)), nil
	case common.Bytes2Hex(crypto.Keccak256([]byte("isOwner(address)"))[:4]):
		if !bytes.Equal(input[16:36], s.owner.Bytes()) {
			return "0x" + strings.Repeat("0", 64), nil
		}
		return "0x" + common.Bytes2Hex(common.LeftPadBytes([]byte{1}, 32)), nil
	case common.Bytes2Hex(crypto.Keccak256([]byte("getTransactionHash(address,uint256,bytes,uint8,uint256,uint256,uint256,address,address,uint256)"))[:4]):
		args := input[4:]
		if len(args) < 384 || !bytes.Equal(args[12:32], s.target.Bytes()) || new(big.Int).SetBytes(args[288:320]).Uint64() != 7 || new(big.Int).SetBytes(args[64:96]).Uint64() != 320 {
			s.t.Fatal("Safe hash arguments changed")
		}
		return s.hash.Hex(), nil
	default:
		s.t.Fatalf("unexpected selector %x", input[:4])
		return "", nil
	}
}

type v3SafeHTTPStub func(*http.Request) (*http.Response, error)

func (f v3SafeHTTPStub) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestV3SafeQueueFailsClosed(t *testing.T) {
	t.Setenv("SAFE_API_KEY", "test-only")
	old := v3SafeProposalHTTP
	t.Cleanup(func() { v3SafeProposalHTTP = old })
	p := V3SafeProposal{Safe: v3TokenOwnerSafe, Nonce: "7", SafeTxHash: common.HexToHash("0x1234").Hex()}
	list := `{"count":1,"results":[{"nonce":7}]}`
	v3SafeProposalHTTP = &http.Client{Transport: v3SafeHTTPStub(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-only" {
			t.Fatal("missing bearer")
		}
		code := http.StatusNotFound
		body := "{}"
		if strings.Contains(r.URL.Path, "/safes/") {
			code = http.StatusOK
			body = list
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if err := v3SafeQueueCheck(context.Background(), p); err == nil {
		t.Fatal("occupied nonce accepted")
	}
	list = `{"count":101,"results":[]}`
	if err := v3SafeQueueCheck(context.Background(), p); err == nil {
		t.Fatal("truncated queue accepted")
	}
	list = `{"count":0,"results":[]}`
	if err := v3SafeQueueCheck(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	list = `{}`
	if err := v3SafeQueueCheck(context.Background(), p); err == nil {
		t.Fatal("missing queue pagination accepted")
	}
	list = `{"count":1,"results":[{"nonce":"7"}]}`
	if err := v3SafeQueueCheck(context.Background(), p); err == nil {
		t.Fatal("string nonce accepted")
	}
	_ = json.Valid([]byte(list))
}

func TestV3SafeProposalReadbackMustMatchExactPayload(t *testing.T) {
	t.Setenv("SAFE_API_KEY", "test-only")
	old := v3SafeProposalHTTP
	t.Cleanup(func() { v3SafeProposalHTTP = old })
	p := V3SafeProposal{Safe: v3TokenOwnerSafe, To: v3TokenProxy, Data: "0x1234", Nonce: "7", SafeTxHash: common.HexToHash("0xabcd").Hex()}
	row := map[string]any{"safe": p.Safe, "to": p.To, "data": p.Data, "nonce": 7, "safeTxHash": p.SafeTxHash, "value": "0", "operation": 0}
	v3SafeProposalHTTP = &http.Client{Transport: v3SafeHTTPStub(func(r *http.Request) (*http.Response, error) {
		b, _ := json.Marshal(row)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b)), Header: make(http.Header)}, nil
	})}
	if err := v3SafeProposalReadback(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	row["data"] = "0xdead"
	if err := v3SafeProposalReadback(context.Background(), p); err == nil {
		t.Fatal("changed calldata accepted")
	}
}

func TestV3SafeProposalSubmitRequiresReadback(t *testing.T) {
	t.Setenv("SAFE_API_KEY", "test-only")
	old := v3SafeProposalHTTP
	t.Cleanup(func() { v3SafeProposalHTTP = old })
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	safe := common.HexToAddress(v3TokenOwnerSafe)
	target := common.HexToAddress(v3TokenProxy)
	owner := crypto.PubkeyToAddress(key.PublicKey)
	hash := common.HexToHash("0x" + strings.Repeat("ab", 32))
	server := rpc.NewServer()
	if err := server.RegisterName("eth", &v3SafeRPCStub{t: t, safe: safe, target: target, owner: owner, hash: hash}); err != nil {
		t.Fatal(err)
	}
	client := ethclient.NewClient(rpc.DialInProc(server))
	defer client.Close()
	data := v3TokenCallData("configurePaymentToken(address,bool)", &target, true)
	digest := crypto.Keccak256([]byte("\x19Ethereum Signed Message:\n32"), hash.Bytes())
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		t.Fatal(err)
	}
	sig[64] += 27
	posts := 0
	readback := false
	v3SafeProposalHTTP = &http.Client{Transport: v3SafeHTTPStub(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-only" {
			t.Fatal("missing server-side bearer")
		}
		code := http.StatusNotFound
		body := `{}`
		if strings.Contains(r.URL.Path, "/safes/") && r.Method == http.MethodGet {
			if r.URL.Query().Get("nonce") != "7" || r.URL.Query().Get("executed") != "false" || (r.URL.Query().Get("trusted") != "true" && r.URL.Query().Get("trusted") != "false") {
				t.Fatal("queue filter missing")
			}
			code = http.StatusOK
			body = `{"count":0,"results":[]}`
		}
		if r.Method == http.MethodPost {
			posts++
			var request map[string]any
			if json.NewDecoder(r.Body).Decode(&request) != nil || request["contractTransactionHash"] != hash.Hex() || request["to"] != target.Hex() || request["nonce"] != "7" || request["sender"] != owner.Hex() {
				t.Fatalf("wrong Safe POST: %v", request)
			}
			code = http.StatusCreated
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/multisig-transactions/"+hash.Hex()+"/") && posts > 0 && readback {
			row := map[string]any{"safe": safe.Hex(), "to": target.Hex(), "data": "0x" + common.Bytes2Hex(data), "nonce": 7, "safeTxHash": hash.Hex(), "value": "0", "operation": 0}
			b, _ := json.Marshal(row)
			body = string(b)
			code = http.StatusOK
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if _, err := submitV3SafeProposal(context.Background(), client, safe, target, data, hash.Hex(), owner.Hex(), "0x"+common.Bytes2Hex(sig)); err == nil || posts != 1 {
		t.Fatalf("missing readback incorrectly accepted: %v posts=%d", err, posts)
	}
	// Isolate a fresh service scenario. In operation an unknown POST must be
	// reconciled under its original hash rather than retried blindly.
	posts = 0
	if _, err := submitV3SafeProposalChecked(context.Background(), client, safe, target, data, hash.Hex(), owner.Hex(), "0x"+common.Bytes2Hex(sig), func() error { return errors.New("approval expired or simulation rejected") }); err == nil || posts != 0 {
		t.Fatal("failed final check reached Safe POST", posts, err)
	}
	readback = true
	if _, err := submitV3SafeProposal(context.Background(), client, safe, target, data, hash.Hex(), owner.Hex(), "0x"+common.Bytes2Hex(sig)); err != nil || posts != 1 {
		t.Fatalf("matching readback rejected: %v posts=%d", err, posts)
	}
}
