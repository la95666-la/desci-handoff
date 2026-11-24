package chainevent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// Read-only, block-pinned multi-height provider for a single disposable
// Core/outbox -> Job -> financial -> reorg recovery trajectory.
type v3BridgeHistory struct {
	blocks map[uint64]*v3PinnedScanStub
	latest uint64
}

func (h v3BridgeHistory) at(n *big.Int) (*v3PinnedScanStub, error) {
	if n != nil && n.IsUint64() && h.blocks[n.Uint64()] != nil {
		return h.blocks[n.Uint64()], nil
	}
	return nil, context.DeadlineExceeded
}
func (h v3BridgeHistory) ChainID(ctx context.Context) (*big.Int, error) {
	return h.blocks[90].ChainID(ctx)
}
func (h v3BridgeHistory) BlockNumber(context.Context) (uint64, error) { return h.latest, nil }
func (h v3BridgeHistory) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	s, err := h.at(n)
	if err != nil {
		return nil, err
	}
	return s.HeaderByNumber(ctx, n)
}
func (h v3BridgeHistory) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	s, err := h.at(n)
	if err != nil {
		return nil, err
	}
	return s.CodeAt(ctx, a, n)
}
func (h v3BridgeHistory) StorageAt(ctx context.Context, a common.Address, slot common.Hash, n *big.Int) ([]byte, error) {
	s, err := h.at(n)
	if err != nil {
		return nil, err
	}
	return s.StorageAt(ctx, a, slot, n)
}
func (h v3BridgeHistory) CallContract(ctx context.Context, c ethereum.CallMsg, n *big.Int) ([]byte, error) {
	s, err := h.at(n)
	if err != nil {
		return nil, err
	}
	return s.CallContract(ctx, c, n)
}
func (h v3BridgeHistory) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	s, err := h.at(q.FromBlock)
	if err != nil {
		return nil, err
	}
	return s.FilterLogs(ctx, q)
}
func (h v3BridgeHistory) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	for _, s := range h.blocks {
		if receipt := s.receipts[hash]; receipt != nil {
			return receipt, nil
		}
	}
	return nil, context.DeadlineExceeded
}

// A network-none disposable trajectory checks two independent signed review
// domains, typed Core storage, synthetic signed attempts (never broadcast),
// the original-hash creation bridge, checked Job finance and bounded reorg
// recovery. The read-only RPC stub cannot authorize a production execution.
func TestV3CreationBridgeDisposableMySQLOriginalAttempt(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable MySQL unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
		db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable V3 MySQL")
	}
	proxy := common.HexToAddress("0x9393939393939393939393939393939393939393")
	sale := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	impl := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	key, err := crypto.HexToECDSA(strings.Repeat("7", 64))
	if err != nil {
		t.Fatal(err)
	}
	operator := crypto.PubkeyToAddress(key.PublicKey)
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 93, ProxyAddress: proxy,
		ProjectToken: sale, PaymentToken: pay}
	parentHead := &types.Header{Number: big.NewInt(89)}
	head := &types.Header{Number: big.NewInt(90), ParentHash: parentHead.Hash()}
	params := commonchain.V3CreateParams{ProjectID: big.NewInt(93), ProjectToken: sale, PaymentToken: pay,
		ProjectOwner: sale, FeeReceiver: pay, FeeRate: 200, Price: big.NewInt(20), HardCap: big.NewInt(1000),
		SoftCap: big.NewInt(500), MaxPerUser: big.NewInt(900), Start: big.NewInt(100), End: big.NewInt(200),
		VestingTimes: []*big.Int{big.NewInt(300)}, VestingPercents: []*big.Int{big.NewInt(10000)}}
	calldata, err := commonchain.PackV3Create(params)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 7, To: &proxy, Gas: 150000,
		GasPrice: big.NewInt(1), Data: calldata}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := abi.NewType("address", "", nil)
	u8, _ := abi.NewType("uint8", "", nil)
	u16, _ := abi.NewType("uint16", "", nil)
	u256, _ := abi.NewType("uint256", "", nil)
	fields := abi.Arguments{{Type: addr}, {Type: addr}, {Type: u8}, {Type: u8}, {Type: u16}}
	for i := 0; i < 6; i++ {
		fields = append(fields, abi.Argument{Type: u256})
	}
	payload, err := fields.Pack(sale, pay, uint8(18), uint8(18), uint16(200), big.NewInt(20),
		big.NewInt(1000), big.NewInt(500), big.NewInt(900), big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Address: proxy, BlockNumber: 90, BlockHash: head.Hash(), TxHash: signed.Hash(), Index: 3,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")),
			common.BigToHash(big.NewInt(93)), common.BytesToHash(sale.Bytes()), common.BytesToHash(pay.Bytes())}, Data: payload}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl,
		ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	setExpiresAt := time.Now().Add(6 * time.Second).Unix()
	set := idov3whitelist.SetManifest{ChainID: 31337, ProjectID: 93,
		FundingMode: constant.FundingModeCappedPresale, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		ProxyAddress: v3Lower(proxy), ProjectToken: v3Lower(sale),
		ProjectOwner: v3Lower(sale), Implementation: v3Lower(impl), ProxyCodeHash: strings.ToLower(runtime.ProxyCodeHash.Hex()),
		ImplementationCodeHash: strings.ToLower(runtime.CodeHash.Hex()), OwnerSafe: "0x3333333333333333333333333333333333333333",
		OwnerCodeHash: strings.ToLower(crypto.Keccak256Hash([]byte{2}).Hex()), Operator: v3Lower(operator),
		Governor: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", GovernorCodeHash: strings.ToLower(crypto.Keccak256Hash([]byte{3}).Hex()),
		StartBlock: 1, Confirmations: 3, PaymentSymbol: "USDC", PaymentToken: v3Lower(pay),
		PaymentDecimals: 18,
		FeeReceiver:     v3Lower(pay), FeeRate: 200, RequestDigest: strings.ToLower(crypto.Keccak256Hash([]byte{5}).Hex()), ExpiresAt: setExpiresAt}
	scan := idov3whitelist.AdmissionManifest{ChainID: 31337, ProjectID: 93, ProxyAddress: set.ProxyAddress,
		ProjectToken: set.ProjectToken, PaymentToken: set.PaymentToken, Implementation: set.Implementation,
		ProxyCodeHash: set.ProxyCodeHash, ImplementationCodeHash: set.ImplementationCodeHash,
		Governor: set.Governor, StartBlock: 1, CreationBlock: 90, Confirmations: 3,
		ExpiresAt: time.Now().Add(time.Hour).Unix()}
	setPublic, setPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	scanPublic, scanPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	setJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	setSignature := ed25519.Sign(setPrivate, append([]byte("joincare:ido-v3:set:v1\x00"), setJSON...))
	scanJSON, err := json.Marshal(scan)
	if err != nil {
		t.Fatal(err)
	}
	scanSignature := ed25519.Sign(scanPrivate, append([]byte("joincare:ido-v3:admission:v1\x00"), scanJSON...))
	manifestDigest := sha256.Sum256(append([]byte("joincare:ido-v3:set:v1\x00"), setJSON...))
	stub := &v3PinnedScanStub{chain: 31337, head: head, parentHead: parentHead, proxy: proxy, impl: impl,
		payment: pay, sale: sale, project: 93, tuple: v3PendingTuple(93, sale, pay), logs: []types.Log{log},
		receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90),
			BlockHash: head.Hash(), TxHash: signed.Hash(), Logs: []*types.Log{&log}}}
	seed := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("disposable V3 bridge fixture unavailable: %v", err)
		}
	}
	seed(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,
		implementation_code_hash,governor_address,start_block) VALUES(31337,?, ?,?,?,'0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`,
		v3Lower(proxy), v3Lower(impl), strings.ToLower(runtime.ProxyCodeHash.Hex()), strings.ToLower(runtime.CodeHash.Hex()))
	seed(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,
		token_address,payment_token_address,owner_address,fee_receiver,token_decimals,payment_token_decimals,
		success_fee_rate,token_price_raw,hard_cap_raw,soft_cap_raw,max_per_user_raw,start_time,end_time,
		description,tags,team_info,roadmap,partners)
		VALUES(93,'Set','capped_presale','ido_v3_stablecoin',31337,?,?,?,?,?,18,18,200,'20','1000','500','900',
		'1970-01-01 00:01:40','1970-01-01 00:03:20','','','','','')`,
		v3Lower(proxy), v3Lower(sale), v3Lower(pay), v3Lower(sale), v3Lower(pay))
	seed(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(93,31337,?)`, v3Lower(proxy))
	seed(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,manifest_digest,
		create_calldata_hash,operator_address) VALUES(93,31337,?,?,?,?,?)`, v3Lower(proxy), set.RequestDigest,
		"0x"+hex.EncodeToString(manifestDigest[:]), strings.ToLower(crypto.Keccak256Hash(calldata).Hex()), v3Lower(operator))
	ctx := context.Background()
	if idov3whitelist.VerifySignedSetManifest(setPublic, set, setSignature, common.HexToHash(set.RequestDigest), time.Now()) != nil ||
		idov3whitelist.CheckCreationSetCommitment(ctx, db, set, scan) != nil {
		t.Fatal("independent signed Set not bound to create commitment")
	}
	if _, err := idov3whitelist.AdmitSignedCreation(ctx, db, scanPublic, scan, scanSignature, time.Now()); err != nil {
		t.Fatal("independently signed scanner create admission unavailable")
	}
	wrongScan := scan
	wrongScan.ProxyAddress = v3Lower(sale)
	if idov3whitelist.CheckCreationSetCommitment(ctx, db, set, wrongScan) == nil {
		t.Fatal("foreign scan proxy reused Set commitment")
	}
	// Core's typed storage route, not a handwritten ready row: a Safe or a
	// foreign proxy must fail before the durable shared outbox is touched.
	plan := commonchain.V3TransportPreview{Transport: commonchain.V3RelayerTransport,
		To: proxy, From: operator, CallData: calldata}
	if _, err := chaintx.ReserveV3CreateIntentTx(db, binding, commonchain.V3TransportPreview{
		Transport: commonchain.V3SafeTransport, To: proxy, From: operator, CallData: calldata}); err == nil {
		t.Fatal("Safe create reached relayer outbox")
	}
	if _, err := chaintx.ReserveV3CreateIntentTx(db, binding, commonchain.V3TransportPreview{
		Transport: commonchain.V3RelayerTransport, To: sale, From: operator, CallData: calldata}); err == nil {
		t.Fatal("foreign proxy reached V3 create outbox")
	}
	var intentID uint64
	if err := db.Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		intentID, reserveErr = chaintx.ReserveV3CreateIntentTx(tx, binding, plan)
		if reserveErr != nil {
			return reserveErr
		}
		return chaintx.ReadyV3IntentTx(tx, binding, operator, intentID)
	}); err != nil || intentID == 0 {
		t.Fatal("typed Core create intent/ready unavailable")
	}
	seed(`UPDATE chain_tx_submission SET state='signed',nonce=7,gas_limit=150000,gas_price_raw='1',raw_tx=?,tx_hash=?,
		v3_signed_at=UTC_TIMESTAMP(6) WHERE project_id=93 AND action='ido_v3_create' AND state='ready'`, raw, signed.Hash().Hex())
	var attempt entity.ChainTxSubmission
	if db.Where("id=? AND project_id=93 AND action='ido_v3_create'", intentID).Take(&attempt).Error != nil {
		t.Fatal("disposable signed create unavailable")
	}
	seed(`INSERT INTO chain_relayer_nonce(chain_id,sender_address,next_nonce,active_submission_id) VALUES(31337,?,8,?)`,
		operator.Hex(), attempt.ID)
	// The attempt was signed under a live Set. The DB-generated timestamp is
	// immutable, so cross the real wall-clock deadline rather than fabricating
	// an expired timestamp or bypassing the 1021 trigger.
	if idov3whitelist.VerifySignedScanManifest(scanPublic, scan, scanSignature, time.Now()) != nil {
		t.Fatal("scanner admission expired before bridge")
	}
	if !chaintx.V3SignedBeforeSet(attempt, setExpiresAt) {
		t.Fatal("fixture signature not inside original Set review window")
	}
	// An independently confirmed original receipt precedes historical
	// materialization. The scanner bridge must not infer it from a topic.
	seed(`UPDATE chain_tx_submission SET state='final_success',receipt_status=1,receipt_block_number=90,
		receipt_block_hash=? WHERE id=? AND state='signed' AND active_slot=1`, head.Hash().Hex(), attempt.ID)
	admitted, err := idov3whitelist.AdmitSignedCreation(ctx, db, scanPublic, scan, scanSignature, time.Now())
	if err != nil || admitted.Binding != binding || admitted.Runtime != runtime {
		t.Fatal("signed scanner decision no longer matches exact project")
	}
	// Only the original confirmed hash may materialize after expiry. Neither
	// Job nor this test signs/broadcasts another transaction at that point.
	// Manifest expiry and verifier both use whole Unix seconds. Wait past
	// the inclusive expiry second, not merely 100ms past its beginning.
	until := time.Until(time.Unix(setExpiresAt+1, 0))
	if until > 0 {
		time.Sleep(until + 100*time.Millisecond)
	}
	if idov3whitelist.VerifySignedSetManifest(setPublic, set, setSignature, common.HexToHash(set.RequestDigest), time.Now()) == nil {
		t.Fatal("expired Set admitted a new create")
	}
	if err := bridgeV3Creation(ctx, db, stub, admitted, operator, time.Now().Add(time.Hour).Unix(),
		setExpiresAt, true); err != nil {
		t.Fatal("original signed creation bridge failed")
	}
	var project struct {
		Status string `gorm:"column:status"`
		Locked uint8  `gorm:"column:onchain_locked"`
	}
	if db.Raw(`SELECT status,onchain_locked FROM projects WHERE project_id=93`).Scan(&project).Error != nil ||
		project.Status != constant.ProjectStatusPending || project.Locked != 1 {
		t.Fatal("canonical creation did not lock exact V3 project")
	}
	var applied entity.ChainTxSubmission
	if db.Where("id=?", attempt.ID).Take(&applied).Error != nil || applied.State != constant.ChainTxStateApplied ||
		applied.FinalEventRecordID == nil || applied.ActiveSlot != nil || applied.TxHash == nil ||
		*applied.TxHash != signed.Hash().Hex() {
		t.Fatal("creation journal did not apply original signed attempt")
	}
	var fence entity.ChainRelayerNonce
	if db.Where("chain_id=31337 AND sender_address=?", operator.Hex()).Take(&fence).Error != nil ||
		fence.ActiveSubmissionID != nil || fence.NextNonce != 8 {
		t.Fatal("creation bridge did not release exact shared sender slot")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err != nil {
		t.Fatal("verified creation projection not published")
	}
	stub.receipts = map[common.Hash]*types.Receipt{signed.Hash(): stub.receipt}
	// Keep this exact project/attempt in the same disposable database for the
	// subsequent Job blocks. Different test processes' journal fixtures are
	// deliberately not used as a source for finance or recovery.
	word := func(v *big.Int) []byte { return v.FillBytes(make([]byte, 32)) }
	deposit := new(big.Int).Mul(big.NewInt(30), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	block := func(height uint64, parent *types.Header, state byte, raised, sold, reserved *big.Int,
		entries []types.Log) *v3PinnedScanStub {
		t.Helper()
		hdr := &types.Header{Number: new(big.Int).SetUint64(height), ParentHash: parent.Hash()}
		tuple := v3PendingTuple(93, sale, pay)
		tuple[32+8*32+31] = state
		copy(tuple[32+15*32:32+16*32], word(raised))
		copy(tuple[32+16*32:32+17*32], word(sold))
		copy(tuple[32+17*32:32+18*32], word(reserved))
		s := &v3PinnedScanStub{chain: 31337, head: hdr, parentHead: parent,
			proxy: proxy, impl: impl, payment: pay, sale: sale, project: 93, tuple: tuple,
			receipts: make(map[common.Hash]*types.Receipt)}
		for i := range entries {
			entries[i].Address, entries[i].BlockNumber, entries[i].BlockHash = proxy, height, hdr.Hash()
			entries[i].Index = uint(i + 1)
			s.receipts[entries[i].TxHash] = &types.Receipt{Status: types.ReceiptStatusSuccessful,
				BlockNumber: new(big.Int).SetUint64(height), BlockHash: hdr.Hash(), TxHash: entries[i].TxHash,
				Logs: []*types.Log{&entries[i]}}
		}
		s.logs = entries
		return s
	}
	zero := big.NewInt(0)
	projectTopic := common.BigToHash(big.NewInt(93))
	b91 := block(91, head, 0, zero, zero, deposit, []types.Log{{
		TxHash: common.HexToHash("0x91"), Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")),
			projectTopic, common.BytesToHash(sale.Bytes())}, Data: word(deposit),
	}})
	b92 := block(92, b91.head, 0, zero, zero, deposit, nil)
	finishPolicy := v3PolicySameCasePending(t, db, binding, runtime, set, scan, setPublic, setSignature, scanPublic, scanSignature, stub, b91, b92)
	activateCall, err := commonchain.PackV3Lifecycle("activateProject", big.NewInt(93))
	if err != nil {
		t.Fatal(err)
	}
	activation, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 8, To: &proxy, Gas: 150000,
		GasPrice: big.NewInt(1), Data: activateCall}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if err != nil {
		t.Fatal(err)
	}
	activationRaw, err := activation.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var activateID uint64
	if err := db.Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		activateID, reserveErr = chaintx.ReserveV3OperatorIntentTx(tx, binding,
			commonchain.V3TransportPreview{Transport: commonchain.V3RelayerTransport,
				To: proxy, From: operator, CallData: activateCall}, "ido_v3_activate")
		if reserveErr != nil {
			return reserveErr
		}
		return chaintx.ReadyV3IntentTx(tx, binding, operator, activateID)
	}); err != nil || activateID == 0 {
		t.Fatal("typed lifecycle intent unavailable")
	}
	seed(`UPDATE chain_tx_submission SET state='signed',nonce=8,gas_limit=150000,gas_price_raw='1',
		raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6) WHERE id=? AND state='ready'`,
		activationRaw, activation.Hash().Hex(), activateID)
	seed(`UPDATE chain_relayer_nonce SET next_nonce=9,active_submission_id=? WHERE chain_id=31337 AND sender_address=? AND active_submission_id IS NULL AND next_nonce=8`, activateID, operator.Hex())
	b93 := block(93, b92.head, 1, zero, zero, deposit, []types.Log{{
		TxHash: activation.Hash(), Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), projectTopic},
	}})
	b93.policy = append([]byte(nil), b92.policy...)
	b93.owner, b93.governor = b92.owner, b92.governor
	b93.ownerCode, b93.governorCode, b93.threshold = b92.ownerCode, b92.governorCode, b92.threshold
	buyer := common.HexToAddress("0x6666666666666666666666666666666666666666")
	purchase := types.Log{TxHash: common.HexToHash("0x93"), Topics: []common.Hash{
		crypto.Keccak256Hash([]byte("Purchased(uint256,address,address,uint256,address,uint256)")),
		projectTopic, common.BytesToHash(buyer.Bytes()), common.BytesToHash(pay.Bytes())},
		Data: append(append(word(big.NewInt(600)), common.LeftPadBytes(sale.Bytes(), 32)...), word(deposit)...)}
	b94 := block(94, b93.head, 1, big.NewInt(600), deposit, deposit, []types.Log{purchase})
	b94.policy = append([]byte(nil), b93.policy...)
	b94.owner, b94.governor = b93.owner, b93.governor
	b94.ownerCode, b94.governorCode, b94.threshold = b93.ownerCode, b93.governorCode, b93.threshold
	history := v3BridgeHistory{blocks: map[uint64]*v3PinnedScanStub{89: stub, 90: stub,
		91: b91, 92: b92, 93: b93, 94: b94}, latest: 97}
	for _, height := range []uint64{93} {
		if count, err := v3ScanConfirmedRange(ctx, db, history, binding, runtime, 3, 1); err != nil || count != 1 {
			t.Fatalf("registered Job block %d unavailable: count=%d err=%v", height, count, err)
		}
	}
	var activationState string
	if db.Raw(`SELECT state FROM chain_tx_submission WHERE id=?`, activateID).Scan(&activationState).Error != nil ||
		activationState != constant.ChainTxStateApplied {
		t.Fatal("original lifecycle hash not applied by verified Job")
	}
	// A matching topic but contradictory receipt cannot materialize finance.
	goodReceipt := b94.receipts[purchase.TxHash]
	badReceipt := *goodReceipt
	badReceipt.Status = types.ReceiptStatusFailed
	b94.receipts[purchase.TxHash] = &badReceipt
	if count, err := v3ScanConfirmedRange(ctx, db, history, binding, runtime, 3, 1); err == nil || count != 0 {
		t.Fatal("failed purchase receipt reached financial journal")
	}
	var effects int64
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE project_id=93 AND entry_kind='purchase'`).Scan(&effects).Error != nil || effects != 0 {
		t.Fatal("invalid receipt left V3 financial facts")
	}
	b94.receipts[purchase.TxHash] = goodReceipt
	if count, err := v3ScanConfirmedRange(ctx, db, history, binding, runtime, 3, 1); err != nil || count != 1 {
		t.Fatalf("canonical purchase not applied: count=%d err=%v", count, err)
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE project_id=93 AND entry_kind='purchase' AND status='canonical'`).Scan(&effects).Error != nil || effects != 2 {
		t.Fatal("purchase payment and project entitlement not distinct canonical facts")
	}
	if wallets, err := readV3PublishedProjection(ctx, db, binding); err != nil || !strings.Contains(wallets, v3Lower(buyer)) {
		t.Fatal("canonical purchase wallet projection unavailable")
	}
	// Replace only the purchase block; the signed creation and activation
	// survive. The monitor must halt publication before independently
	// revalidating the entire surviving prefix and resuming at block 92.
	replaced := *b94.head
	replaced.Extra = []byte("purchase reorg")
	b94.head = &replaced
	if err := checkV3CursorCanonical(ctx, db, history, binding); err == nil {
		t.Fatal("replaced purchase block left cursor running")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("reorg retained published wallet")
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE project_id=93 AND status='canonical'`).Scan(&effects).Error != nil || effects != 0 {
		t.Fatal("reorg retained canonical financial facts")
	}
	if err := recoverV3HaltedPrefix(ctx, db, history, admitted); err != nil {
		t.Fatalf("authenticated surviving-prefix recovery failed: %v", err)
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE project_id=93 AND entry_kind='purchase' AND status='orphaned'`).Scan(&effects).Error != nil || effects != 2 {
		t.Fatal("orphan purchase facts were not retained separately")
	}
	if db.Raw(`SELECT state FROM chain_tx_submission WHERE id=?`, activateID).Scan(&activationState).Error != nil ||
		activationState != constant.ChainTxStateApplied {
		t.Fatal("surviving original activation was not restored")
	}
	if wallets, err := readV3PublishedProjection(ctx, db, binding); err != nil || strings.Contains(wallets, v3Lower(buyer)) {
		t.Fatal("recovered prefix still published orphan buyer")
	}
	finishPolicy(history, b94)
}
