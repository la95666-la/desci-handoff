package chainevent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/auth/sign"
	"joincare_server/pkg/common/constant"
	localredis "joincare_server/pkg/common/database/redis"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	admin "joincare_server/pkg/core/admin"
	adminjwt "joincare_server/pkg/core/admin/jwt"
	"joincare_server/pkg/core/db"
	clientroute "joincare_server/pkg/core/route"
	"joincare_server/pkg/core/service/project"
)

// All eth_* calls are served from the same in-memory synthetic history used
// by the registered Job. No RPC method can submit a transaction.
type v3M3History struct {
	v3BridgeHistory
	txs map[common.Hash]*types.Transaction
}

func (h *v3M3History) receipt(hash common.Hash) (*types.Receipt, error) {
	for _, s := range h.blocks {
		if item := s.receipts[hash]; item != nil {
			return item, nil
		}
	}
	return nil, fmt.Errorf("synthetic receipt unavailable")
}

func (h *v3M3History) TransactionByHash(_ context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	tx := h.txs[hash]
	if tx == nil {
		return nil, false, fmt.Errorf("synthetic transaction unavailable")
	}
	return tx, false, nil
}

func (h *v3M3History) jsonRPC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	ctx := r.Context()
	parseBlock := func(raw json.RawMessage) (*big.Int, error) {
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return nil, fmt.Errorf("bad height")
		}
		if value == "latest" {
			return new(big.Int).SetUint64(h.latest), nil
		}
		v, ok := new(big.Int).SetString(strings.TrimPrefix(value, "0x"), 16)
		if !ok {
			return nil, fmt.Errorf("bad height")
		}
		return v, nil
	}
	var result any
	var err error
	switch req.Method {
	case "eth_chainId":
		result = "0x7a69"
	case "eth_blockNumber":
		result = fmt.Sprintf("0x%x", h.latest)
	case "eth_getBlockByNumber":
		var n *big.Int
		n, err = parseBlock(req.Params[0])
		if err == nil {
			var header *types.Header
			header, err = h.HeaderByNumber(ctx, n)
			if err == nil {
				encoded, _ := json.Marshal(header)
				var fields map[string]any
				_ = json.Unmarshal(encoded, &fields)
				fields["hash"] = header.Hash().Hex()
				fields["difficulty"] = "0x0"
				fields["gasLimit"] = "0x0"
				fields["gasUsed"] = "0x0"
				fields["timestamp"] = "0x0"
				result = fields
			}
		}
	case "eth_getCode":
		var address string
		var n *big.Int
		_ = json.Unmarshal(req.Params[0], &address)
		n, err = parseBlock(req.Params[1])
		if err == nil {
			var code []byte
			code, err = h.CodeAt(ctx, common.HexToAddress(address), n)
			result = "0x" + hex.EncodeToString(code)
		}
	case "eth_getStorageAt":
		var address, slot string
		var n *big.Int
		_ = json.Unmarshal(req.Params[0], &address)
		_ = json.Unmarshal(req.Params[1], &slot)
		n, err = parseBlock(req.Params[2])
		if err == nil {
			var value []byte
			value, err = h.StorageAt(ctx, common.HexToAddress(address), common.HexToHash(slot), n)
			result = "0x" + hex.EncodeToString(value)
		}
	case "eth_call":
		var call struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Input string `json:"input"`
			Data  string `json:"data"`
		}
		_ = json.Unmarshal(req.Params[0], &call)
		n, e := parseBlock(req.Params[1])
		err = e
		if err == nil {
			data := call.Input
			if data == "" {
				data = call.Data
			}
			to := common.HexToAddress(call.To)
			var output []byte
			output, err = h.CallContract(ctx, ethereum.CallMsg{From: common.HexToAddress(call.From), To: &to, Data: common.FromHex(data)}, n)
			result = "0x" + hex.EncodeToString(output)
		}
	case "eth_getTransactionReceipt":
		var hash string
		_ = json.Unmarshal(req.Params[0], &hash)
		result, err = h.receipt(common.HexToHash(hash))
	case "eth_getTransactionByHash":
		var hash string
		_ = json.Unmarshal(req.Params[0], &hash)
		if tx := h.txs[common.HexToHash(hash)]; tx != nil {
			var raw map[string]any
			encoded, _ := json.Marshal(tx)
			_ = json.Unmarshal(encoded, &raw)
			raw["blockNumber"] = "0x5b"
			raw["blockHash"] = strings.ToLower(h.blocks[91].head.Hash().Hex())
			raw["transactionIndex"] = "0x0"
			result = raw
		} else {
			err = fmt.Errorf("synthetic transaction unavailable")
		}
	default:
		err = fmt.Errorf("unsupported read-only synthetic RPC method")
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "synthetic RPC unavailable"}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func v3M3Pinned(t *testing.T, prefix string, payload any, private ed25519.PrivateKey) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	viper.Set(prefix+"manifest", string(encoded))
	viper.Set(prefix+"signature", base64.StdEncoding.EncodeToString(ed25519.Sign(private, append([]byte(map[string]string{
		"idoV3Set.": "joincare:ido-v3:set:v1\x00", "idoV3Scanner.": "joincare:ido-v3:admission:v1\x00",
		"idoV3PolicyExecution.": "joincare:ido-v3:safe-execution:v1\x00"}[prefix]), encoded...))))
	viper.Set(prefix+"pinnedPublicKey", base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)))
}

func v3PolicySameCasePending(t *testing.T, database *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	runtime idov3whitelist.RuntimeIdentity, set idov3whitelist.SetManifest, scan idov3whitelist.AdmissionManifest,
	setPublic ed25519.PublicKey, setSig []byte, scanPublic ed25519.PublicKey, scanSig []byte,
	creation, pendingDeposit, pendingNext *v3PinnedScanStub) func(v3BridgeHistory, *v3PinnedScanStub) {
	t.Helper()
	// This is the same database and locked Pending project created by M2.
	if db.Mysql() != nil {
		t.Fatal("M3 test must own its local DB injection")
	}
	db.Client.Mysql = database
	t.Cleanup(func() { db.Client.Mysql = nil })
	// The proof middleware's rate limiter uses Redis. A non-routable local
	// client fails open without connecting to any external service.
	db.Client.Redis = &localredis.Connection{Client: redisclient.NewClusterClient(&redisclient.ClusterOptions{Addrs: []string{"127.0.0.1:1"}, MaxRetries: 0, DialTimeout: 10 * time.Millisecond})}
	t.Cleanup(func() { db.Client.Redis.Client.Close(); db.Client.Redis = nil })
	settings := map[string]any{"idoV3Scanner.enabled": true, "idoV3PolicyPreview.enabled": true,
		"idoV3PolicyReview.enabled": true, "idoV3PolicyExecution.enabled": true,
		"idoV3PolicyJournal.enabled": true, "idoV3PolicyProof.enabled": true,
		"idoV3Execution.proxyAddress":    strings.ToLower(binding.ProxyAddress.Hex()),
		"idoV3Execution.operatorAddress": set.Operator, "chain.project.v2.contractAddress": "0x5555555555555555555555555555555555555555",
		"jwt.secret": strings.Repeat("c", 48), "admin.jwtSecret": strings.Repeat("a", 48), "application.environment": "test"}
	for k, value := range settings {
		old := viper.Get(k)
		key := k
		t.Cleanup(func() { viper.Set(key, old) })
		viper.Set(k, value)
	}
	for _, prefix := range []string{"idoV3Set.", "idoV3Scanner.", "idoV3PolicyExecution."} {
		for _, field := range []string{"manifest", "signature", "pinnedPublicKey"} {
			key := prefix + field
			old := viper.Get(key)
			t.Cleanup(func() { viper.Set(key, old) })
		}
	}
	setJSON, _ := json.Marshal(set)
	scanJSON, _ := json.Marshal(scan)
	viper.Set("idoV3Set.manifest", string(setJSON))
	viper.Set("idoV3Set.signature", base64.StdEncoding.EncodeToString(setSig))
	viper.Set("idoV3Set.pinnedPublicKey", base64.StdEncoding.EncodeToString(setPublic))
	viper.Set("idoV3Scanner.manifest", string(scanJSON))
	viper.Set("idoV3Scanner.signature", base64.StdEncoding.EncodeToString(scanSig))
	viper.Set("idoV3Scanner.pinnedPublicKey", base64.StdEncoding.EncodeToString(scanPublic))
	ctx := context.Background()
	proxy := binding.ProxyAddress
	governor, owner := common.HexToAddress(set.Governor), common.HexToAddress(set.OwnerSafe)
	ownerCode, governorCode := []byte{2}, []byte{3}
	if common.HexToHash(set.OwnerCodeHash) != crypto.Keccak256Hash(ownerCode) || common.HexToHash(set.GovernorCodeHash) != crypto.Keccak256Hash(governorCode) {
		t.Fatal("fixture governance code mismatch")
	}
	prior := v3BridgeHistory{blocks: map[uint64]*v3PinnedScanStub{89: creation, 90: creation, 91: pendingDeposit, 92: pendingNext}, latest: 92}
	for _, s := range prior.blocks {
		s.owner, s.governor = owner, governor
		s.ownerCode, s.governorCode = ownerCode, governorCode
		s.threshold = 2
	}
	// Preview/approval run while Pending and before the Safe block exists.
	// A later, independent external Safe observation shares deposit block 91.
	base := pendingDeposit
	confirmed := pendingNext
	history := &v3M3History{v3BridgeHistory: prior, txs: map[common.Hash]*types.Transaction{}}
	history.latest = 92
	rpc := httptest.NewServer(http.HandlerFunc(history.jsonRPC))
	t.Cleanup(rpc.Close)
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if chainID, err := client.ChainID(ctx); err != nil || chainID.Uint64() != 31337 {
		t.Fatalf("isolated JSON-RPC not connected: %v", err)
	}
	oldRPC := viper.Get("chain.project.url")
	t.Cleanup(func() { viper.Set("chain.project.url", oldRPC) })
	viper.Set("chain.project.url", rpc.URL)
	seed := func(query string, args ...any) {
		t.Helper()
		if e := database.Exec(query, args...).Error; e != nil {
			t.Fatalf("M3 disposable fixture: %v", e)
		}
	}
	seed(`INSERT INTO admin_users(id,username,password_hash,role_code,email,display_name) VALUES
		(701,'m3_initiator','local','super_admin','','local'),(702,'m3_approver','local','super_admin','','local')`)
	seed(`INSERT INTO fair_launch_mfa_provider(provider_id,admission_digest,manifest_digest,adapter_identity,generation,enabled,valid_from)
		VALUES('m3_disposable_adapter',REPEAT('a',64),REPEAT('b',64),'isolated-local-test',1,1,UTC_TIMESTAMP(6)-INTERVAL 1 MINUTE)`)
	// Even if the numeric ID belongs to a real Fair Launch action, a V3 MFA
	// assertion cannot bind to that unrelated FK instead of the V3 review.
	bad := database.Exec(`INSERT INTO fair_launch_mfa_assertion(token_digest,provider_id,provider_evidence_digest,provider_admission_digest,
		provider_generation,externally_verified,admin_id,operation,stage,project_id,action_request_id,issued_at,expires_at)
		VALUES(REPEAT('f',64),'m3_disposable_adapter',REPEAT('c',64),REPEAT('a',64),1,1,702,
		'ido_v3.policy.versioned_change','action_approval',93,1,UTC_TIMESTAMP(6)-INTERVAL 1 MINUTE,UTC_TIMESTAMP(6)+INTERVAL 5 MINUTE)`)
	if bad.Error == nil {
		t.Fatal("M3 MFA accepted Fair Launch action ID instead of review ID")
	}
	project.SetFairLaunchMFAProviderAdmissionVerifier(func(row entity.FairLaunchMFAProvider, _ time.Time) error {
		if row.ProviderID != "m3_disposable_adapter" || row.Generation != 1 {
			return fmt.Errorf("wrong isolated provider")
		}
		return nil
	})
	t.Cleanup(func() { project.SetFairLaunchMFAProviderAdmissionVerifier(nil) })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin.Register(router.Group("/admin"))
	clientroute.Register(router.Group("/client"))
	adminToken := func(id int64) string {
		token, _, e := adminjwt.Generate(id, "local", "super_admin", 1, viper.GetString("admin.jwtSecret"))
		if e != nil {
			t.Fatal(e)
		}
		return token
	}
	call := func(method, path string, body any, adminID int64, uid int) (int, map[string]json.RawMessage) {
		t.Helper()
		var payload []byte
		if body != nil {
			payload, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		if adminID != 0 {
			req.Header.Set("Authorization", "Bearer "+adminToken(adminID))
		}
		if uid != 0 {
			token, _, e := sign.GenerateToken(uid, 1, viper.GetString("jwt.secret"))
			if e != nil {
				t.Fatal(e)
			}
			req.Header.Set(constant.HEADER_JOINCARE_SIGN, token)
			req.Header.Set(constant.HEADER_JOINCARE_TEST, "1")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var result map[string]json.RawMessage
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		if w.Code == http.StatusOK && result["status"] == nil {
			t.Fatal("HTTP response lacked status")
		}
		return w.Code, result
	}
	path := "/admin/v1/projects/93/ido-v3/policy"
	wallet := strings.ToLower(common.HexToAddress("0x6666666666666666666666666666666666666666").Hex())
	if code, _ := call(http.MethodGet, "/client/project/v1/ido-v3/93/proof?wallet="+wallet, nil, 0, 0); code != http.StatusPreconditionRequired && code != http.StatusUnauthorized {
		t.Fatalf("anonymous proof: %d", code)
	}
	code, response := call(http.MethodPost, path+"/preview", map[string]any{"wallets": []string{wallet}, "enabled": true, "reason": "M3 isolated same-case"}, 701, 0)
	if code != 200 {
		t.Fatalf("Pending HTTP preview: %d %s", code, response["code"])
	}
	var preview project.V3PolicyPreview
	if json.Unmarshal(response["data"], &preview) != nil || preview.TargetVersion != "1" {
		t.Fatal("preview not pinned to original public policy")
	}
	issueMFA := func(token string, id uint64, stage string, requestID any) {
		t.Helper()
		digest := sha256.Sum256([]byte(token))
		seed(`INSERT INTO fair_launch_mfa_assertion(token_digest,provider_id,provider_evidence_digest,provider_admission_digest,
			provider_generation,externally_verified,admin_id,operation,stage,project_id,v3_policy_review_id,issued_at,expires_at)
			VALUES(?,'m3_disposable_adapter',REPEAT('c',64),REPEAT('a',64),1,1,?,'ido_v3.policy.versioned_change',?,93,?,UTC_TIMESTAMP(6)-INTERVAL 1 MINUTE,UTC_TIMESTAMP(6)+INTERVAL 5 MINUTE)`, hex.EncodeToString(digest[:]), id, stage, requestID)
	}
	firstToken, secondToken := strings.Repeat("1", 40), strings.Repeat("2", 40)
	issueMFA(firstToken, 701, "action_initiation", nil)
	code, response = call(http.MethodPost, path+"/reviews", map[string]any{"wallets": preview.CanonicalMembers, "enabled": true, "reason": preview.Reason, "base_block": preview.Block,
		"base_hash": preview.BlockHash, "preview_digest": preview.PreviewDigest, "mfa_assertion_token": firstToken}, 701, 0)
	if code != 200 {
		t.Fatalf("HTTP review: %d %s", code, response["code"])
	}
	var created struct {
		ReviewID string `json:"review_id"`
	}
	_ = json.Unmarshal(response["data"], &created)
	reviewID, e := strconv.ParseUint(created.ReviewID, 10, 64)
	if e != nil || reviewID == 0 {
		t.Fatal("review id unavailable")
	}
	code, response = call(http.MethodGet, path+"/reviews/"+created.ReviewID, nil, 702, 0)
	if code != 200 {
		t.Fatalf("second Admin cannot inspect: %d %s", code, response["code"])
	}
	var reviewed project.V3PolicyPreview
	_ = json.Unmarshal(response["data"], &reviewed)
	if reviewed.PreviewDigest != preview.PreviewDigest || len(reviewed.CanonicalMembers) != 1 {
		t.Fatal("immutable review differs from first preview")
	}
	issueMFA(secondToken, 702, "action_approval", reviewID)
	code, response = call(http.MethodPost, path+"/reviews/"+created.ReviewID+"/approve", map[string]any{"preview_digest": reviewed.PreviewDigest, "mfa_assertion_token": secondToken}, 702, 0)
	if code != 200 {
		t.Fatalf("second Admin approval: %d %s", code, response["code"])
	}
	var consumed int64
	if database.Raw(`SELECT COUNT(*) FROM fair_launch_mfa_assertion WHERE admin_id IN (701,702) AND consumed_at IS NOT NULL`).Scan(&consumed).Error != nil || consumed != 2 {
		t.Fatal("both independent MFA assertions not consumed")
	}
	if code, _ = call(http.MethodPost, path+"/reviews/"+created.ReviewID+"/approve", map[string]any{"preview_digest": reviewed.PreviewDigest, "mfa_assertion_token": secondToken}, 702, 0); code == 200 {
		t.Fatal("replayed second MFA accepted")
	}
	if code, _ = call(http.MethodPost, path+"/reviews/"+created.ReviewID+"/approve", map[string]any{"preview_digest": reviewed.PreviewDigest, "mfa_assertion_token": secondToken}, 701, 0); code == 200 {
		t.Fatal("initiating Admin approved own review")
	}
	// The signed Safe outer transaction is fixture-only: no HTTP or RPC
	// endpoint can sign, submit or return a new transaction on request.
	scope := idov3whitelist.Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93), ProxyAddress: proxy}
	tree, e := idov3whitelist.Build(scope, []string{wallet})
	if e != nil {
		t.Fatal(e)
	}
	commitment, e := tree.CommitSnapshot(scope, 1)
	if e != nil {
		t.Fatal(e)
	}
	change := idov3whitelist.PolicyChange{ExpectedVersion: 0, TargetVersion: 1, Enabled: true, Root: commitment.Root, Candidate: &commitment}
	inner, e := idov3whitelist.PackPolicyChangeCall(scope, change)
	if e != nil {
		t.Fatal(e)
	}
	safeABI := `[{"type":"function","name":"execTransaction","inputs":[{"name":"to","type":"address"},{"name":"value","type":"uint256"},{"name":"data","type":"bytes"},{"name":"operation","type":"uint8"},{"name":"safeTxGas","type":"uint256"},{"name":"baseGas","type":"uint256"},{"name":"gasPrice","type":"uint256"},{"name":"gasToken","type":"address"},{"name":"refundReceiver","type":"address"},{"name":"signatures","type":"bytes"}],"outputs":[{"type":"bool"}]}]`
	parsed, e := abi.JSON(strings.NewReader(safeABI))
	if e != nil {
		t.Fatal(e)
	}
	outer, e := parsed.Pack("execTransaction", proxy, big.NewInt(0), inner, uint8(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), common.Address{}, common.Address{}, make([]byte, 130))
	if e != nil {
		t.Fatal(e)
	}
	private, e := crypto.GenerateKey()
	if e != nil {
		t.Fatal(e)
	}
	tx, e := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 0, To: &governor, Gas: 400000, GasPrice: big.NewInt(1), Data: outer}), types.LatestSignerForChainID(scope.ChainID), private)
	if e != nil {
		t.Fatal(e)
	}
	safeHash := crypto.Keccak256Hash([]byte("isolated-safe-execution"))
	policyData := make([]byte, 64)
	policyData[31] = 1
	copy(policyData[32:], commitment.Root[:])
	policyLog := types.Log{Address: proxy, BlockNumber: 91, BlockHash: base.head.Hash(), TxHash: tx.Hash(), Index: 2,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)")), common.BigToHash(scope.ProjectID), common.BigToHash(big.NewInt(1))}, Data: policyData}
	successData := make([]byte, 64)
	copy(successData, safeHash[:])
	successLog := types.Log{Address: governor, BlockNumber: 91, BlockHash: base.head.Hash(), TxHash: tx.Hash(), Index: 1,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("ExecutionSuccess(bytes32,uint256)"))}, Data: successData}
	base.logs = append(base.logs, policyLog)
	base.receipts[tx.Hash()] = &types.Receipt{Status: types.ReceiptStatusSuccessful,
		BlockNumber: big.NewInt(91), BlockHash: base.head.Hash(), TxHash: tx.Hash(), Logs: []*types.Log{&successLog, &policyLog}}
	history.txs[tx.Hash()] = tx
	policyBytes := make([]byte, 96)
	policyBytes[31] = 1
	copy(policyBytes[32:64], commitment.Root[:])
	policyBytes[95] = 1
	base.policy = policyBytes
	confirmed.policy = policyBytes
	pending93 := *confirmed
	pending93.head = &types.Header{Number: big.NewInt(93), ParentHash: confirmed.head.Hash()}
	pending93.parentHead = confirmed.head
	pending93.logs = nil
	pending93.receipts = map[common.Hash]*types.Receipt{}
	pending94 := pending93
	pending94.head = &types.Header{Number: big.NewInt(94), ParentHash: pending93.head.Hash()}
	pending94.parentHead = pending93.head
	history.blocks[93], history.blocks[94] = &pending93, &pending94
	_, attPrivate, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	att := project.V3SafeAttestation{ChainID: 31337, ProjectID: 93, Proxy: strings.ToLower(proxy.Hex()),
		Governor: strings.ToLower(governor.Hex()), ReviewDigest: reviewed.PreviewDigest,
		TxHash: strings.ToLower(tx.Hash().Hex()), SafeTxHash: strings.ToLower(safeHash.Hex()), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	v3M3Pinned(t, "idoV3PolicyExecution.", att, attPrivate)
	seed(`INSERT INTO user(id,twitter_id,twitter_handle,twitter_display_name,twitter_avatar,wallet_address,type)
		VALUES(703,'local703','local703','local703','',?,'local')`, wallet)
	seed(`INSERT INTO user_wallet(uid,wallet_address,chain,is_primary,verified_at,verification_method)
		VALUES(703,?,'evm',1,UTC_TIMESTAMP(6),'personal_sign')`, wallet)
	var ownerID int
	if database.Raw(`SELECT uid FROM user_wallet_identity WHERE wallet_address=?`, wallet).Scan(&ownerID).Error != nil || ownerID != 703 {
		t.Fatal("verified registration did not establish matching wallet identity")
	}
	proofPath := "/client/project/v1/ido-v3/93/proof?wallet=" + wallet
	if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code == 200 {
		t.Fatal("approval alone granted a wallet proof")
	}
	if code, _ = call(http.MethodPost, path+"/reviews/"+created.ReviewID+"/observe", nil, 701, 0); code == 200 {
		t.Fatal("unconfirmed Safe execution observed")
	}
	history.latest = 94
	code, response = call(http.MethodPost, path+"/reviews/"+created.ReviewID+"/observe", nil, 701, 0)
	if code != 200 {
		t.Fatalf("canonical Safe observation: %d %s", code, response["code"])
	}
	if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code == 200 {
		t.Fatal("proof granted before scanner publication")
	}
	// The registered bounded scanner consumes the exact same RPC history and
	// persists the canonical policy journal and checkpoint atomically.
	if count, e := v3ScanConfirmedRange(ctx, database, history, binding, runtime, 3, 2); e != nil || count != 2 {
		t.Fatalf("policy scanner: count=%d err=%v", count, e)
	}
	if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code == 200 {
		t.Fatal("Pending project received proof before activation")
	}
	return func(active v3BridgeHistory, last *v3PinnedScanStub) {
		history.v3BridgeHistory = active
		history.latest = active.latest
		// The synthetic RPC must provide the confirmed head and live tip,
		// not just scanned heights. All three are unchanged Active blocks.
		parent := last
		for height := uint64(95); height <= history.latest; height++ {
			child := *parent
			child.head = &types.Header{Number: new(big.Int).SetUint64(height), ParentHash: parent.head.Hash()}
			child.parentHead = parent.head
			child.logs = nil
			child.receipts = map[common.Hash]*types.Receipt{}
			history.blocks[height] = &child
			parent = &child
		}
		code, response = call(http.MethodGet, proofPath, nil, 0, 703)
		if code != 200 {
			t.Fatalf("bound wallet proof: %d %s", code, response["code"])
		}
		var proof project.V3WalletProof
		_ = json.Unmarshal(response["data"], &proof)
		if proof.Version != "1" || !strings.EqualFold(proof.Root, commitment.Root.Hex()) || proof.Wallet != wallet {
			t.Fatal("wrong single-wallet proof")
		}
		if code, _ = call(http.MethodGet, "/client/project/v1/ido-v3/93/proof?wallet=0x7777777777777777777777777777777777777777", nil, 0, 703); code == 200 {
			t.Fatal("unbound wallet received proof")
		}
		// A changed tip is not a new approved version. Even before Job has
		// scanned it, the buyer cannot reuse a proof for the old root.
		latestPolicy := history.blocks[history.latest].policy
		changedPolicy := append([]byte(nil), latestPolicy...)
		changedPolicy[95] = 2
		history.blocks[history.latest].policy = changedPolicy
		if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code != http.StatusConflict {
			t.Fatalf("unapproved latest version did not close proof: %d", code)
		}
		history.blocks[history.latest].policy = latestPolicy
		seed(`UPDATE user SET session_version=2 WHERE id=703 AND session_version=1`)
		if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code == http.StatusOK {
			t.Fatal("revoked user session received proof")
		}
		seed(`UPDATE user SET session_version=1 WHERE id=703 AND session_version=2`)
		// Replacement of a published policy block must revoke both scanner
		// journal state and HTTP proof. Recovery cannot restore its old root.
		replacement := *base.head
		replacement.Extra = []byte("replaced-policy-block")
		base.head = &replacement
		// Replace every descendant through the published cursor. A reorg of
		// block 91 cannot leave old descendant hashes canonical at 92/93.
		for _, height := range []uint64{92, 93} {
			old := *history.blocks[height].head
			old.Extra = []byte("replaced-policy-descendant")
			history.blocks[height].head = &old
		}
		if e := checkV3CursorCanonical(ctx, database, history, binding); e == nil {
			t.Fatal("replaced policy block left scanner running")
		}
		if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code == 200 {
			t.Fatal("reorg retained wallet proof")
		}
		if e := recoverV3HaltedPrefix(ctx, database, history, idov3whitelist.AdmittedProject{Binding: binding, Runtime: runtime, CreationBlock: scan.CreationBlock, Confirmations: 3}); e != nil {
			t.Fatalf("policy prefix recovery: %v", e)
		}
		if code, _ = call(http.MethodGet, proofPath, nil, 0, 703); code == 200 {
			t.Fatal("orphaned policy proof restored")
		}
		var orphan int64
		if database.Raw(`SELECT COUNT(*) FROM ido_v3_policy_event_journal WHERE project_id=93 AND state='orphaned'`).Scan(&orphan).Error != nil || orphan != 1 {
			t.Fatal("replaced policy event not retained as orphan")
		}
	}
}
