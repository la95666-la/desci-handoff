package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

// The disposable MySQL fixture exercises the actual cursor/projection and
// checkpoint constraints. No read can reach a public or production RPC.
type v3EmptyWindowRPC struct {
	headerMu          sync.Mutex
	blocks            map[uint64]*v3PinnedScanStub
	latest            uint64
	logs              []types.Log
	filters           int
	changeSecond      bool
	changeHeader      bool
	headerReads       map[uint64]int
	postCommitDB      *gorm.DB
	postCommitProject uint64
}

func (h *v3EmptyWindowRPC) ChainID(context.Context) (*big.Int, error) { return big.NewInt(31337), nil }
func (h *v3EmptyWindowRPC) BlockNumber(ctx context.Context) (uint64, error) {
	if h.postCommitDB != nil {
		var cursor struct {
			Block uint64 `gorm:"column:cursor_block"`
		}
		read := h.postCommitDB.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=31337 AND project_id=?`, h.postCommitProject).Scan(&cursor)
		if read.Error != nil {
			return 0, read.Error
		}
		if read.RowsAffected == 1 && cursor.Block == 93 {
			return 94, nil
		} // three confirmations require head >= 95
	}
	return h.latest, nil
}
func (h *v3EmptyWindowRPC) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	h.headerMu.Lock()
	defer h.headerMu.Unlock()
	if n == nil || !n.IsUint64() || h.blocks[n.Uint64()] == nil {
		return nil, context.DeadlineExceeded
	}
	height := n.Uint64()
	h.headerReads[height]++
	if h.changeHeader && height == 92 && h.headerReads[height] >= 2 {
		replacement := *h.blocks[height].head
		replacement.Extra = []byte("reorg")
		return &replacement, nil
	}
	return h.blocks[height].head, nil
}
func (h *v3EmptyWindowRPC) at(n *big.Int) (*v3PinnedScanStub, error) {
	if n == nil || !n.IsUint64() || h.blocks[n.Uint64()] == nil {
		return nil, context.DeadlineExceeded
	}
	return h.blocks[n.Uint64()], nil
}
func (h *v3EmptyWindowRPC) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	s, e := h.at(n)
	if e != nil {
		return nil, e
	}
	return s.CodeAt(ctx, a, n)
}
func (h *v3EmptyWindowRPC) StorageAt(ctx context.Context, a common.Address, k common.Hash, n *big.Int) ([]byte, error) {
	s, e := h.at(n)
	if e != nil {
		return nil, e
	}
	return s.StorageAt(ctx, a, k, n)
}
func (h *v3EmptyWindowRPC) CallContract(ctx context.Context, c ethereum.CallMsg, n *big.Int) ([]byte, error) {
	s, e := h.at(n)
	if e != nil {
		return nil, e
	}
	return s.CallContract(ctx, c, n)
}
func (h *v3EmptyWindowRPC) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	h.filters++
	if q.FromBlock == nil || q.ToBlock == nil || len(q.Addresses) != 1 || q.Addresses[0] != h.blocks[90].proxy {
		return nil, context.DeadlineExceeded
	}
	if h.changeSecond && h.filters == 2 {
		return []types.Log{{Address: h.blocks[90].proxy, BlockNumber: 92}}, nil
	}
	var found []types.Log
	for _, log := range h.logs {
		if log.BlockNumber >= q.FromBlock.Uint64() && log.BlockNumber <= q.ToBlock.Uint64() {
			found = append(found, log)
		}
	}
	return found, nil
}
func (h *v3EmptyWindowRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return nil, context.DeadlineExceeded
}

func TestV3EmptyRangeDisposableMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL only")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable MySQL")
	}
	// Other disposable fixtures own 0x4444... and 0x8888... in this same
	// rehearsal; keep the fast-path fixture in a separate runtime scope.
	proxy := common.HexToAddress("0x6767676767676767676767676767676767676767")
	sale := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	impl := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 41376991395446999, ProjectToken: sale, PaymentToken: pay}
	seed := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	seed(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block)
		VALUES(31337,?,?,?,?,?,1)`, v3Lower(proxy), v3Lower(impl),
		strings.ToLower(crypto.Keccak256Hash([]byte{0x60, 0x80}).Hex()),
		strings.ToLower(crypto.Keccak256Hash([]byte{0x60, 0x01}).Hex()), v3Lower(sale))
	seed(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners) VALUES (?,'Pending','capped_presale','ido_v3_stablecoin',31337,?,1,?,?,?,?, '', '', '', '', '')`, b.ProjectID, v3Lower(proxy), v3Lower(sale), v3Lower(pay), v3Lower(sale), v3Lower(pay))
	seed(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES (?,31337,?)`, b.ProjectID, v3Lower(proxy))
	header := &types.Header{Number: big.NewInt(90)}
	h := &v3EmptyWindowRPC{blocks: map[uint64]*v3PinnedScanStub{}, latest: 110, headerReads: map[uint64]int{}}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	for height := uint64(90); height <= 93; height++ {
		if height > 90 {
			header = &types.Header{Number: new(big.Int).SetUint64(height), ParentHash: header.Hash()}
		}
		h.blocks[height] = &v3PinnedScanStub{chain: 31337, head: header, proxy: proxy, impl: impl, payment: pay, sale: sale, project: b.ProjectID, tuple: v3PendingTuple(b.ProjectID, sale, pay)}
	}
	created := &commonchain.V3CreatedEvent{ChainID: 31337, ProjectID: b.ProjectID, BlockNumber: 90, Proxy: proxy, ProjectToken: sale, PaymentToken: pay, ProjectOwner: sale, FeeReceiver: pay, ProjectDecimals: 18, PaymentDecimals: 18, FeeRate: 200, TokenPriceRaw: "20", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", StartTimeRaw: "100", EndTimeRaw: "200", TxHash: common.HexToHash("0xb1"), BlockHash: h.blocks[90].head.Hash(), LogIndex: 1}
	payload, e := json.Marshal(commonchain.V3ReplayEvent{Created: created})
	if e != nil {
		t.Fatal(e)
	}
	seed(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) VALUES (31337,?,?,90,?,1,'running')`, v3Lower(proxy), b.ProjectID, v3LowerHashHeader(h.blocks[90].head))
	seed(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash) VALUES (31337,?,?,90,?)`, v3Lower(proxy), b.ProjectID, v3LowerHashHeader(h.blocks[90].head))
	seed(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,tx_hash,log_index,event_kind,payload) VALUES (31337,?,?,90,?,?,1,'created',?)`, v3Lower(proxy), b.ProjectID, v3LowerHashHeader(h.blocks[90].head), strings.ToLower(created.TxHash.Hex()), string(payload))
	seed(`INSERT INTO ido_v3_project_projection(chain_id,proxy_address,project_id,revision,cursor_block,cursor_hash,state,raised_raw,sold_raw,deposited_raw,refunded_raw,claimed_raw,wallets,proceeds_paid_raw,fee_paid_raw,project_tokens_withdrawn_raw) VALUES (31337,?,?,1,90,?,'pending','0','0','0','0','0','{}','0','0','0')`, v3Lower(proxy), b.ProjectID, v3LowerHashHeader(h.blocks[90].head))
	check := func(block, rev uint64, count int64) {
		t.Helper()
		var current struct {
			Block uint64 `gorm:"column:cursor_block"`
			Rev   uint64 `gorm:"column:revision"`
		}
		if e := db.Raw(`SELECT cursor_block,revision FROM ido_v3_scan_cursor WHERE chain_id=31337 AND project_id=?`, b.ProjectID).Scan(&current).Error; e != nil || current.Block != block || current.Rev != rev {
			t.Fatalf("cursor %+v error %v", current, e)
		}
		var n int64
		if e := db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND status='canonical'`, b.ProjectID).Scan(&n).Error; e != nil || n != count {
			t.Fatalf("checkpoint count=%d err=%v", n, e)
		}
	}
	ctx := context.Background()
	testV3WalletJSONLifecycleMySQL(t, db, b, created, h.blocks[90].head, runtime)
	if os.Getenv("JOINCARE_V3_PROXY_STREAM_TEST") == "1" {
		testV3ProxyStreamSeededMySQL(t, db, h, b, runtime)
	}
	h.changeSecond = true
	firstShared := newV3TickReads(ctx, h)
	defer firstShared.close()
	if handled, err := scanV3EmptyRange(ctx, db, firstShared, b, runtime, 91, 92, 3); err == nil || handled != 0 {
		t.Fatalf("changed second filter accepted: handled=%v err=%v", handled, err)
	}
	check(90, 1, 1)
	h.changeSecond = false
	h.filters = 0
	h.changeHeader = true
	h.headerReads = map[uint64]int{}
	reorgShared := newV3TickReads(ctx, h)
	defer reorgShared.close()
	if handled, err := scanV3EmptyRange(ctx, db, reorgShared, b, runtime, 91, 92, 3); err == nil || handled != 0 {
		t.Fatalf("reorg accepted: handled=%v err=%v", handled, err)
	}
	check(90, 1, 1)
	h.changeHeader = false
	h.headerReads = map[uint64]int{}
	h.filters = 0
	// No proxy logs alone cannot prove the project/policy stayed constant.
	// A changed end-of-window project tuple or policy must leave no prefix.
	originalTuple := h.blocks[92].tuple
	h.blocks[92].tuple = append([]byte(nil), originalTuple...)
	h.blocks[92].tuple[32+9*32+31]++ // immutable token price
	if count, err := scanV3EmptyRange(ctx, db, h, b, runtime, 91, 92, 3); err == nil || count != 0 {
		t.Fatalf("changed project snapshot accepted: count=%d err=%v", count, err)
	}
	check(90, 1, 1)
	h.blocks[92].tuple = originalTuple
	h.blocks[92].policy = make([]byte, 96)
	h.blocks[92].policy[95] = 1 // policy version changed without a log
	if count, err := scanV3EmptyRange(ctx, db, h, b, runtime, 91, 92, 3); err == nil || count != 0 {
		t.Fatalf("changed policy snapshot accepted: count=%d err=%v", count, err)
	}
	check(90, 1, 1)
	h.blocks[92].policy = nil
	// A checkpoint collision on the second block must roll back the first
	// inserted checkpoint and the projection/cursor updates together.
	seed(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash)
		VALUES(31337,?,?,92,?)`, v3Lower(proxy), b.ProjectID, v3LowerHashHeader(h.blocks[92].head))
	if count, err := scanV3EmptyRange(ctx, db, h, b, runtime, 91, 92, 3); err == nil || count != 0 {
		t.Fatalf("checkpoint collision published partial prefix: count=%d err=%v", count, err)
	}
	check(90, 1, 2) // only the deliberately injected, untrusted row remains
	var firstBlock int64
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND block_number=91`, b.ProjectID).Scan(&firstBlock).Error; err != nil || firstBlock != 0 {
		t.Fatalf("rolled-back prefix retained first checkpoint: count=%d err=%v", firstBlock, err)
	}
	seed(`DELETE FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND block_number=92`, b.ProjectID)
	check(90, 1, 1)
	h.logs = []types.Log{{Address: proxy, BlockNumber: 93, BlockHash: h.blocks[93].head.Hash(), TxHash: common.HexToHash("0xc1"), Topics: []common.Hash{v3PolicyUpdateTopic, common.BigToHash(new(big.Int).SetUint64(b.ProjectID))}}}
	previousPolicyFlag := viper.Get("idoV3PolicyJournal.enabled")
	viper.Set("idoV3PolicyJournal.enabled", true)
	t.Cleanup(func() { viper.Set("idoV3PolicyJournal.enabled", previousPolicyFlag) })
	// Exercise the actual approved range driver: it publishes the two empty
	// heights, then hands the event-bearing height to the old receipt-aware
	// scanner. This synthetic policy log has no valid receipt, so it must stop
	// with only the verified empty prefix committed.
	if count, err := v3ScanApprovedConfirmedRange(ctx, db, h, b, runtime, 3, 3); err == nil || count != 2 {
		t.Fatalf("approved range skipped event or lost empty prefix: count=%d err=%v", count, err)
	}
	check(92, 3, 3)
	// A complete 16-block event-free window is one atomic publication. Roll
	// back this synthetic rehearsal so the independent failure tests below
	// continue from the unchanged cursor at 92.
	h.logs = nil
	previousHeader := h.blocks[93].head
	for height := uint64(94); height <= 108; height++ {
		next := &types.Header{Number: new(big.Int).SetUint64(height), ParentHash: previousHeader.Hash()}
		h.blocks[height] = &v3PinnedScanStub{chain: 31337, head: next, proxy: proxy, impl: impl,
			payment: pay, sale: sale, project: b.ProjectID, tuple: v3PendingTuple(b.ProjectID, sale, pay)}
		previousHeader = next
	}
	h.latest = 120
	rollback := errors.New("disposable 16-block rehearsal complete")
	err = db.Transaction(func(tx *gorm.DB) error {
		shared := newV3TickReads(ctx, h)
		defer shared.close()
		count, scanErr := scanV3EmptyRange(ctx, tx, shared, b, runtime, 93, 108, 3)
		if scanErr != nil || count != 16 {
			t.Fatalf("16-block range rejected: count=%d err=%v", count, scanErr)
		}
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Revision uint64 `gorm:"column:revision"`
		}
		if read := tx.Raw(`SELECT cursor_block,revision FROM ido_v3_scan_cursor WHERE chain_id=31337 AND project_id=?`, b.ProjectID).Scan(&cursor); read.Error != nil || read.RowsAffected != 1 || cursor.Block != 108 || cursor.Revision != 19 {
			t.Fatalf("batch cursor mismatched: %+v err=%v", cursor, read.Error)
		}
		var checkpoints int64
		if e := tx.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND status='canonical'`, b.ProjectID).Scan(&checkpoints).Error; e != nil || checkpoints != 19 {
			t.Fatalf("batch checkpoint closure mismatched: count=%d err=%v", checkpoints, e)
		}
		if _, e := readV3PublishedProjection(ctx, tx, b); e != nil {
			t.Fatalf("published batch projection mismatched: %v", e)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("disposable range rollback unavailable: %v", err)
	}
	// The range driver must certify the committed prefix, not compare it to
	// the originally requested end, when it voluntarily yields for budget.
	err = db.Transaction(func(tx *gorm.DB) error {
		budgetCtx, cancel := context.WithTimeout(ctx, 14*time.Second)
		defer cancel()
		shared := newV3TickReads(budgetCtx, h)
		defer shared.close()
		count, scanErr := v3ScanApprovedConfirmedRange(budgetCtx, tx, shared, b, runtime, 3, 128)
		if scanErr != nil || count != v3EmptyRangeLimit {
			t.Fatalf("budget yield rejected committed prefix: count=%d err=%v", count, scanErr)
		}
		if _, e := readV3PublishedProjection(budgetCtx, tx, b); e != nil {
			t.Fatalf("yield lost canonical projection: %v", e)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("disposable yield rollback unavailable: %v", err)
	}
	check(92, 3, 3)
	h.latest = 110
	h.logs = nil
	h.filters = 0
	// The projection is a published fact, not a cache the fast path may
	// silently repair. Corruption must roll back all new checkpoints.
	seed(`UPDATE ido_v3_project_projection SET raised_raw='1' WHERE chain_id=31337 AND project_id=? AND revision=3`, b.ProjectID)
	if handled, err := scanV3EmptyRange(ctx, db, h, b, runtime, 93, 93, 3); err == nil || handled != 0 {
		t.Fatalf("corrupt projection accepted: handled=%v err=%v", handled, err)
	}
	check(92, 3, 3)
	seed(`UPDATE ido_v3_project_projection SET raised_raw='0' WHERE chain_id=31337 AND project_id=? AND revision=3`, b.ProjectID)
	h.logs = []types.Log{{Address: proxy, BlockNumber: 93, BlockHash: h.blocks[93].head.Hash(), TxHash: common.HexToHash("0xc1"), Topics: []common.Hash{v3PolicyUpdateTopic, common.BigToHash(new(big.Int).SetUint64(b.ProjectID))}}}
	h.filters = 0
	var published string
	if e := db.Raw(`SELECT state FROM ido_v3_project_projection WHERE chain_id=31337 AND project_id=? AND revision=3 AND cursor_block=92`, b.ProjectID).Scan(&published).Error; e != nil || published != "pending" {
		t.Fatalf("projection not maintained: %s %v", published, e)
	}
	h.filters = 0
	if handled, err := scanV3EmptyRange(ctx, db, h, b, runtime, 93, 93, 3); err != nil || handled != 0 {
		t.Fatalf("event block skipped: handled=%v err=%v", handled, err)
	}
	check(92, 3, 3)
	h.logs = nil
	h.filters = 0
	h.postCommitDB, h.postCommitProject = db, b.ProjectID
	if count, err := v3ScanApprovedConfirmedRange(ctx, db, h, b, runtime, 3, 1); !errors.Is(err, errV3EmptyRangePostCommit) || count != 1 {
		t.Fatalf("post-commit depth loss accepted: count=%d err=%v", count, err)
	}
	var state string
	read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND project_id=? AND cursor_block=93 AND revision=5`, b.ProjectID).Scan(&state)
	if read.Error != nil || read.RowsAffected != 1 || state != "reorg_halted" {
		t.Fatalf("post-commit unknown left cursor running: %s %v", state, read.Error)
	}
	var visible int64
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_project_projection WHERE chain_id=31337 AND project_id=?`, b.ProjectID).Scan(&visible).Error; err != nil || visible != 0 {
		t.Fatalf("post-commit unknown retained published projection: %d %v", visible, err)
	}
}
