package chainevent

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

// A synthetic history with independently addressable heights, unlike the
// single-current-block scanner stub. It is never an approved RPC source.
type v3TwoBlockHistory struct{ creation, next *v3PinnedScanStub }

// A reorg arriving during the head read must not turn an idle range into a
// successful tick. This stub supplies no authority outside the local test.
type v3RangeSwitchOnHead struct{ v3TwoBlockHistory }

// Replace creation only after bootstrap has committed its initial cursor.
// The read-only RPC has no approved identity; this exercises revocation.
type v3BootstrapSwitchAfterCommit struct {
	*v3PinnedScanStub
	db          *gorm.DB
	binding     idov3whitelist.ExpectedProjectBinding
	switched    bool
	unavailable bool
	loseDepth   bool
	loseParent  bool
}

func (s *v3BootstrapSwitchAfterCommit) BlockNumber(ctx context.Context) (uint64, error) {
	if s.loseDepth {
		var cursor struct {
			Block uint64 `gorm:"column:cursor_block"`
		}
		read := s.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
			s.binding.ChainID, v3Lower(s.binding.ProxyAddress), s.binding.ProjectID).Scan(&cursor)
		if read.Error != nil {
			return 0, fmt.Errorf("synthetic cursor unavailable")
		}
		if read.RowsAffected == 1 && cursor.Block == s.head.Number.Uint64() {
			s.switched = true
			return cursor.Block + 1, nil // one confirmation short, headers unchanged
		}
	}
	return s.v3PinnedScanStub.BlockNumber(ctx)
}

func (s *v3BootstrapSwitchAfterCommit) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if s.loseParent && n != nil && n.IsUint64() && n.Uint64()+1 == s.head.Number.Uint64() {
		var cursor struct {
			Block uint64 `gorm:"column:cursor_block"`
		}
		read := s.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
			s.binding.ChainID, v3Lower(s.binding.ProxyAddress), s.binding.ProjectID).Scan(&cursor)
		if read.Error != nil {
			return nil, fmt.Errorf("synthetic parent cursor unavailable")
		}
		if read.RowsAffected == 1 && cursor.Block == s.head.Number.Uint64() {
			s.switched = true
			return nil, fmt.Errorf("synthetic parent unavailable after commit")
		}
	}
	if !s.switched && n != nil && n.IsUint64() && n.Uint64() == s.head.Number.Uint64() {
		var cursor struct {
			Block uint64 `gorm:"column:cursor_block"`
		}
		read := s.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
			s.binding.ChainID, v3Lower(s.binding.ProxyAddress), s.binding.ProjectID).Scan(&cursor)
		if read.Error != nil {
			return nil, fmt.Errorf("synthetic bootstrap cursor read failed")
		}
		if read.RowsAffected == 1 && cursor.Block == n.Uint64() {
			s.switched = true
			if s.unavailable {
				return nil, fmt.Errorf("synthetic post-commit RPC unavailable")
			}
			changed := *s.head
			changed.Extra = []byte("creation replaced after bootstrap commit")
			s.head = &changed
		}
	}
	return s.v3PinnedScanStub.HeaderByNumber(ctx, n)
}

type v3RangeLaggingHead struct{ v3TwoBlockHistory }

type v3RangeUnavailableHead struct{ v3TwoBlockHistory }

type v3CursorHeaderUnavailable struct{ *v3PinnedScanStub }

type v3CursorChainUnavailable struct{ *v3PinnedScanStub }

func (s v3CursorChainUnavailable) ChainID(context.Context) (*big.Int, error) {
	return nil, fmt.Errorf("synthetic chain unavailable")
}

func (s v3CursorHeaderUnavailable) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	if number != nil && number.IsUint64() && number.Uint64() == s.head.Number.Uint64() {
		return nil, fmt.Errorf("synthetic cursor header unavailable")
	}
	return s.v3PinnedScanStub.HeaderByNumber(ctx, number)
}

func (h v3RangeUnavailableHead) BlockNumber(context.Context) (uint64, error) {
	return 0, fmt.Errorf("synthetic head unavailable")
}

type v3RangeShallowHead struct{ v3TwoBlockHistory }

func (h v3RangeShallowHead) BlockNumber(context.Context) (uint64, error) {
	return h.next.head.Number.Uint64() + 1, nil
}

type v3RangeDropsConfirmations struct {
	v3TwoBlockHistory
	db      *gorm.DB
	binding idov3whitelist.ExpectedProjectBinding
	replace bool
}

func (h v3RangeDropsConfirmations) BlockNumber(ctx context.Context) (uint64, error) {
	var cursor struct {
		Block uint64 `gorm:"column:cursor_block"`
	}
	read := h.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
		h.binding.ChainID, v3Lower(h.binding.ProxyAddress), h.binding.ProjectID).Scan(&cursor)
	if read.Error != nil || read.RowsAffected != 1 {
		return 0, fmt.Errorf("synthetic cursor read unavailable")
	}
	if cursor.Block == h.next.head.Number.Uint64() {
		if h.replace {
			changed := *h.next.head
			changed.Extra = []byte("replaced with lowered final head")
			h.next.head = &changed
		}
		return cursor.Block + 1, nil // one fewer confirmation after commit
	}
	return h.v3TwoBlockHistory.BlockNumber(ctx)
}

type v3RangeSwitchAfterChecked struct {
	v3TwoBlockHistory
	endHeight uint64
	db        *gorm.DB
	binding   idov3whitelist.ExpectedProjectBinding
}

func (h *v3RangeSwitchAfterChecked) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	if number != nil && number.IsUint64() && number.Uint64() == h.endHeight {
		var cursor struct {
			Block uint64 `gorm:"column:cursor_block"`
		}
		read := h.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
			h.binding.ChainID, v3Lower(h.binding.ProxyAddress), h.binding.ProjectID).Scan(&cursor)
		if read.Error != nil || read.RowsAffected != 1 {
			return nil, fmt.Errorf("synthetic cursor read unavailable")
		}
		// The checked scanner's journal has committed the last block, but the
		// range driver's final canonical read now sees a replaced header.
		if cursor.Block == h.endHeight {
			original := h.next.head
			changed := *original
			changed.Extra = []byte("replaced after checked apply")
			return &changed, nil
		}
	}
	return h.v3TwoBlockHistory.HeaderByNumber(ctx, number)
}

type v3RangeRevisionOnHead struct {
	v3TwoBlockHistory
	db       *gorm.DB
	binding  idov3whitelist.ExpectedProjectBinding
	mutateAt uint64
}

type v3RangeRevisionAfterApply struct{ v3RangeRevisionOnHead }

// Mutate the canonical header only when the final confirmation head is
// requested after checked apply and the first final canonical read.
type v3RangeSwitchOnFinalHead struct {
	v3TwoBlockHistory
	endHeight uint64
	db        *gorm.DB
	binding   idov3whitelist.ExpectedProjectBinding
	switched  bool
}

// The first height commits, then the second height's bad log is reported.
// During that failure, the already-published first height is replaced.
type v3RangeSwitchOnPartialFailure struct {
	v3TwoBlockHistory
	committed *types.Header
	switched  bool
}

// Unlike a known replacement, a failed canonical read is unknown. The
// bounded range must still revoke the projection it just committed.
type v3RangeUnknownOnPartialFailure struct {
	v3TwoBlockHistory
	committed uint64
	db        *gorm.DB
	binding   idov3whitelist.ExpectedProjectBinding
}

type v3RangeUnknownOnFirstFailure struct {
	v3TwoBlockHistory
	cursor uint64
	failed bool
}

type v3RangeUnknownOnIdleRecheck struct {
	v3TwoBlockHistory
	cursor uint64
	reads  int
}

func (h *v3RangeUnknownOnIdleRecheck) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if n != nil && n.IsUint64() && n.Uint64() == h.cursor {
		h.reads++
		if h.reads > 1 {
			return nil, fmt.Errorf("synthetic idle canonical header unavailable")
		}
	}
	return h.v3TwoBlockHistory.HeaderByNumber(ctx, n)
}

func (h *v3RangeUnknownOnFirstFailure) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if n != nil && n.IsUint64() && n.Uint64() == h.next.head.Number.Uint64() {
		h.failed = true
		return nil, fmt.Errorf("synthetic first new header unavailable")
	}
	if h.failed && n != nil && n.IsUint64() && n.Uint64() == h.cursor {
		return nil, fmt.Errorf("synthetic prior cursor header unavailable")
	}
	return h.v3TwoBlockHistory.HeaderByNumber(ctx, n)
}

func (h v3RangeUnknownOnPartialFailure) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if n != nil && n.IsUint64() && n.Uint64() == h.committed {
		var cursor struct {
			Block uint64 `gorm:"column:cursor_block"`
		}
		read := h.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
			h.binding.ChainID, v3Lower(h.binding.ProxyAddress), h.binding.ProjectID).Scan(&cursor)
		if read.Error != nil {
			return nil, fmt.Errorf("synthetic cursor read unavailable")
		}
		if cursor.Block == h.committed {
			return nil, fmt.Errorf("synthetic canonical RPC unavailable")
		}
	}
	return h.v3TwoBlockHistory.HeaderByNumber(ctx, n)
}

func (h *v3RangeSwitchOnPartialFailure) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	if !h.switched && q.FromBlock != nil && q.FromBlock.IsUint64() && q.FromBlock.Uint64() == h.next.head.Number.Uint64() {
		h.switched = true
		changed := *h.committed
		changed.Extra = []byte("committed height replaced during next failure")
		h.creation.head = &changed
	}
	return h.v3TwoBlockHistory.FilterLogs(ctx, q)
}

func (h *v3RangeSwitchOnFinalHead) BlockNumber(ctx context.Context) (uint64, error) {
	var cursor struct {
		Block uint64 `gorm:"column:cursor_block"`
	}
	read := h.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
		h.binding.ChainID, v3Lower(h.binding.ProxyAddress), h.binding.ProjectID).Scan(&cursor)
	if read.Error != nil || read.RowsAffected != 1 {
		return 0, fmt.Errorf("synthetic cursor read unavailable")
	}
	if cursor.Block == h.endHeight && !h.switched {
		h.switched = true
		changed := *h.next.head
		changed.Extra = []byte("replaced during final head read")
		h.next.head = &changed
	}
	return h.v3TwoBlockHistory.BlockNumber(ctx)
}

func (h v3RangeRevisionAfterApply) BlockNumber(ctx context.Context) (uint64, error) {
	var cursor struct {
		Block uint64 `gorm:"column:cursor_block"`
	}
	read := h.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`,
		h.binding.ChainID, v3Lower(h.binding.ProxyAddress), h.binding.ProjectID).Scan(&cursor)
	if read.Error != nil || read.RowsAffected != 1 {
		return 0, fmt.Errorf("synthetic cursor read unavailable")
	}
	if cursor.Block == h.mutateAt {
		return h.v3RangeRevisionOnHead.BlockNumber(ctx)
	}
	return h.v3TwoBlockHistory.BlockNumber(ctx)
}

func (h v3RangeRevisionOnHead) BlockNumber(ctx context.Context) (uint64, error) {
	latest, err := h.v3TwoBlockHistory.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	if result := h.db.WithContext(ctx).Exec(`UPDATE ido_v3_scan_cursor SET revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=? AND cursor_block=? AND state='running'`, h.binding.ChainID, v3Lower(h.binding.ProxyAddress), h.binding.ProjectID, h.mutateAt); result.Error != nil || result.RowsAffected != 1 {
		return 0, fmt.Errorf("synthetic revision mutation failed")
	}
	return latest, nil
}

func (h v3RangeLaggingHead) BlockNumber(context.Context) (uint64, error) {
	return h.creation.head.Number.Uint64(), nil
}

func (h v3RangeSwitchOnHead) BlockNumber(ctx context.Context) (uint64, error) {
	latest, err := h.v3TwoBlockHistory.BlockNumber(ctx)
	h.next.head = &types.Header{Number: new(big.Int).Set(h.next.head.Number), ParentHash: h.next.head.ParentHash, Extra: []byte("late replacement")}
	return latest, err
}

func (h v3TwoBlockHistory) ChainID(ctx context.Context) (*big.Int, error) {
	return h.creation.ChainID(ctx)
}
func (h v3TwoBlockHistory) BlockNumber(ctx context.Context) (uint64, error) {
	return h.next.BlockNumber(ctx)
}
func (h v3TwoBlockHistory) at(number *big.Int) (*v3PinnedScanStub, error) {
	if number == nil {
		return nil, fmt.Errorf("historical block required")
	}
	if number.Cmp(h.creation.head.Number) == 0 || number.Cmp(h.creation.parentHead.Number) == 0 {
		return h.creation, nil
	}
	if number.Cmp(h.next.head.Number) == 0 {
		return h.next, nil
	}
	return nil, fmt.Errorf("historical block unavailable")
}
func (h v3TwoBlockHistory) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	s, err := h.at(number)
	if err != nil {
		return nil, err
	}
	return s.HeaderByNumber(ctx, number)
}
func (h v3TwoBlockHistory) CodeAt(ctx context.Context, address common.Address, number *big.Int) ([]byte, error) {
	s, err := h.at(number)
	if err != nil {
		return nil, err
	}
	return s.CodeAt(ctx, address, number)
}
func (h v3TwoBlockHistory) StorageAt(ctx context.Context, address common.Address, slot common.Hash, number *big.Int) ([]byte, error) {
	s, err := h.at(number)
	if err != nil {
		return nil, err
	}
	return s.StorageAt(ctx, address, slot, number)
}
func (h v3TwoBlockHistory) CallContract(ctx context.Context, call ethereum.CallMsg, number *big.Int) ([]byte, error) {
	s, err := h.at(number)
	if err != nil {
		return nil, err
	}
	return s.CallContract(ctx, call, number)
}
func (h v3TwoBlockHistory) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	s, err := h.at(query.FromBlock)
	if err != nil {
		return nil, err
	}
	return s.FilterLogs(ctx, query)
}
func (h v3TwoBlockHistory) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	return h.creation.TransactionReceipt(ctx, hash)
}

// Only the disposable network-none MySQL container exports this DSN. The
// existing candidate audit fixture seeds the single exact V3 binding first.
func TestV3JournalDisposableMySQLAtomicRollback(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable MySQL unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable V3 MySQL")
	}
	proxy := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 41376991395446785,
		ProjectToken: common.HexToAddress("0x1111111111111111111111111111111111111111"), PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	parentHeader := &types.Header{Number: big.NewInt(10)}
	parent := parentHeader.Hash()
	blockHeader := &types.Header{Number: big.NewInt(11), ParentHash: parent}
	block := blockHeader.Hash()
	seed := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal("disposable journal fixture unavailable")
		}
	}
	// The preceding audit test seeded an exact, locked candidate. This
	// synthetic journal test owns only its isolated business status.
	seed(`UPDATE projects SET status='Pending' WHERE project_id=? AND chain_id=31337 AND contract_address=? AND onchain_locked=1`, binding.ProjectID, v3Lower(proxy))
	seed(`INSERT INTO ido_v3_scan_cursor (chain_id,proxy_address,project_id,cursor_block,cursor_hash,state) VALUES (31337,?,?,10,?,'running')`, v3Lower(proxy), binding.ProjectID, v3LowerHash(parent))
	seed(`INSERT INTO ido_v3_block_checkpoint (chain_id,proxy_address,project_id,block_number,block_hash) VALUES (31337,?,?,10,?)`, v3Lower(proxy), binding.ProjectID, v3LowerHash(parent))
	journal := v3Journal{db: db}
	ctx := context.Background()
	var cursor uint64
	readCursor := func() {
		t.Helper()
		if err := db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=?`, v3Lower(proxy), binding.ProjectID).Scan(&cursor).Error; err != nil {
			t.Fatal(err)
		}
	}
	log := types.Log{Address: proxy, BlockNumber: 11, BlockHash: block, TxHash: common.HexToHash("0xb1"), Index: 1,
		Topics: []common.Hash{common.HexToHash("0xdead"), common.BigToHash(new(big.Int).SetUint64(binding.ProjectID))}}
	if err := journal.apply(ctx, binding, 11, parent, block, []types.Log{log}); err == nil {
		t.Fatal("invalid event accepted")
	}
	readCursor()
	if cursor != 10 {
		t.Fatal("failed batch advanced durable cursor")
	}
	var events, checkpoints int64
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=?`, binding.ProjectID).Scan(&events).Error != nil || events != 0 {
		t.Fatal("failed batch left journal row")
	}
	impl := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	scanner := &v3PinnedScanStub{chain: 31337, head: blockHeader, parentHead: parentHeader, proxy: proxy, impl: impl, payment: binding.PaymentToken, sale: binding.ProjectToken, project: binding.ProjectID, tuple: v3PendingTuple(binding.ProjectID, binding.ProjectToken, binding.PaymentToken)}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	if err := v3ScanCheckedBlock(ctx, db, scanner, binding, runtime, 11, 3); err == nil {
		t.Fatal("pre-creation empty block incorrectly published as checked project projection")
	}
	readCursor()
	if cursor != 10 {
		t.Fatal("pre-creation checked failure advanced cursor")
	}
	// The unregistered journal prototype can checkpoint a synthetic empty
	// block without an existing project; the checked production path cannot
	// do so until a separately authenticated creation/start-block rule exists.
	if err := journal.apply(ctx, binding, 11, parent, block, nil); err != nil {
		t.Fatalf("synthetic pre-creation checkpoint rejected: %v", err)
	}
	readCursor()
	if cursor != 11 {
		t.Fatal("empty block did not advance cursor")
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND status='canonical'`, binding.ProjectID).Scan(&checkpoints).Error != nil || checkpoints != 2 {
		t.Fatal("empty block checkpoint missing")
	}
	var revision uint64
	if read := db.Raw(`SELECT revision FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&revision); read.Error != nil || read.RowsAffected != 1 {
		t.Fatal("disposable pre-rollback revision unavailable")
	}
	if err := journal.rollback(ctx, binding, 10, parent, revision); err == nil {
		t.Fatal("running cursor rollback bypassed explicit halt")
	}
	if err := journal.haltMismatch(ctx, binding, 11, block); err != nil {
		t.Fatalf("disposable explicit halt failed: %v", err)
	}
	var businessStatus string
	if err := db.Raw(`SELECT status FROM projects WHERE project_id=?`, binding.ProjectID).Scan(&businessStatus).Error; err != nil || businessStatus != v3BusinessHalted {
		t.Fatalf("reorg halt left executable project status: %s err=%v", businessStatus, err)
	}
	if err := journal.rollback(ctx, binding, 10, parent, revision+1); err != nil {
		t.Fatalf("durable rollback failed: %v", err)
	}
	var state string
	if db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND project_id=?`, binding.ProjectID).Scan(&state).Error != nil || state != "reorg_halted" {
		t.Fatal("reorg failed to halt cursor")
	}
	if err := journal.apply(ctx, binding, 11, parent, block, nil); err == nil {
		t.Fatal("halted cursor resumed without recovery")
	}
	readRevision := func() uint64 {
		t.Helper()
		var revision uint64
		if read := db.Raw(`SELECT revision FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&revision); read.Error != nil || read.RowsAffected != 1 || revision == 0 {
			t.Fatal("disposable cursor revision unavailable")
		}
		return revision
	}
	if err := journal.resumeAtAncestor(ctx, binding, 10, parent, readRevision()); err == nil {
		t.Fatal("pre-creation prefix resumed without canonical creation")
	}
	seed(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=? AND proxy_address=? AND project_id=? AND cursor_block=10 AND state='reorg_halted'`, binding.ChainID, v3Lower(proxy), binding.ProjectID)
	seed(`UPDATE projects SET status='Pending' WHERE project_id=? AND status='V3ReorgHalted'`, binding.ProjectID)
	if err := journal.apply(ctx, binding, 11, parent, common.HexToHash("0xa3"), nil); err != nil {
		t.Fatalf("replacement empty block failed: %v", err)
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND status='orphaned'`, binding.ProjectID).Scan(&checkpoints).Error != nil || checkpoints != 1 {
		t.Fatal("old checkpoint not preserved as orphan")
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_project_projection WHERE chain_id=31337 AND project_id=?`, binding.ProjectID).Scan(&events).Error != nil || events != 0 {
		t.Fatal("empty stream accidentally produced wallet projection")
	}
	// Checked apply must roll back both checkpoint and cursor if the pinned
	// end-of-block project counters disagree with the rebuilt journal.
	checkedHash := common.HexToHash("0xc1")
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: binding.ChainID, ProxyAddress: proxy, ProjectID: binding.ProjectID, ProjectToken: binding.ProjectToken, PaymentToken: binding.PaymentToken, BlockNumber: 12, BlockHash: checkedHash, State: idov3whitelist.ProjectPending, RaisedAtBlockRaw: "1", TokensSoldAtBlockRaw: "0"}
	if err := journal.applyChecked(ctx, binding, 12, common.HexToHash("0xa3"), checkedHash, nil, observed); err == nil {
		t.Fatal("mismatched project counters published")
	}
	readCursor()
	if cursor != 11 {
		t.Fatal("checked apply failure advanced cursor")
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND block_number=12`, binding.ProjectID).Scan(&checkpoints).Error != nil || checkpoints != 0 {
		t.Fatal("checked apply failure retained checkpoint")
	}
	// A canonical mismatch must revoke an already-published projection even
	// when no ancestor/replay decision has yet been made.
	if err := journal.haltMismatch(ctx, binding, 11, common.HexToHash("0xa3")); err != nil {
		t.Fatalf("mismatch halt failed: %v", err)
	}
	if err := journal.apply(ctx, binding, 12, common.HexToHash("0xa3"), common.HexToHash("0xa4"), nil); err == nil {
		t.Fatal("halted cursor advanced after mismatch")
	}
	if err := journal.rollback(ctx, binding, 10, parent, readRevision()); err != nil {
		t.Fatalf("halted cursor rollback rejected: %v", err)
	}
	// A pre-creation ancestor has no authenticated project state. Recovery
	// must halt instead of silently unlocking an empty scanner prefix.
	if err := journal.resumeAtAncestor(ctx, binding, 10, parent, readRevision()); err == nil {
		t.Fatal("pre-creation ancestor resumed without a creation fact")
	}
	// A disposable fixture may reset its own halted cursor to continue the
	// independent event-replay scenario below. Production recovery cannot.
	seed(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=? AND proxy_address=? AND project_id=? AND cursor_block=10 AND state='reorg_halted'`, binding.ChainID, v3Lower(proxy), binding.ProjectID)
	seed(`UPDATE projects SET status='Pending' WHERE project_id=? AND status='V3ReorgHalted'`, binding.ProjectID)
	// The following synthetic event scenario starts from a resumed disposable
	// cursor, not a production recovery decision.
	// Replay actual ABI-shaped event payloads through the durable journal. No
	// RPC/source admission is asserted by this synthetic integration fixture.
	word := func(v uint64) []byte { return new(big.Int).SetUint64(v).FillBytes(make([]byte, 32)) }
	addrWord := func(v common.Address) []byte { return common.LeftPadBytes(v.Bytes(), 32) }
	owner := common.HexToAddress("0x5555555555555555555555555555555555555555")
	buyer := common.HexToAddress("0x6666666666666666666666666666666666666666")
	projectTopic := common.BigToHash(new(big.Int).SetUint64(binding.ProjectID))
	sig := func(v string) common.Hash { return crypto.Keccak256Hash([]byte(v)) }
	operatorKey, keyErr := crypto.GenerateKey()
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	operator := crypto.PubkeyToAddress(operatorKey.PublicKey)
	activateCall, callErr := commonchain.PackV3Lifecycle("activateProject", new(big.Int).SetUint64(binding.ProjectID))
	if callErr != nil {
		t.Fatal(callErr)
	}
	signedActivation, signErr := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 7, To: &proxy, Gas: 150000, GasPrice: big.NewInt(1), Data: activateCall}), types.LatestSignerForChainID(big.NewInt(31337)), operatorKey)
	if signErr != nil {
		t.Fatal(signErr)
	}
	activationRaw, marshalErr := signedActivation.MarshalBinary()
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	latest := parent
	if err := journal.apply(ctx, binding, 11, latest, common.HexToHash("0xa4"), nil); err != nil {
		t.Fatalf("replacement empty block rejected: %v", err)
	}
	// Later synthetic blocks use independently supplied project observations;
	// this fixture does not establish a successful RPC-to-journal path.
	latest = common.HexToHash("0xa4")
	apply := func(height uint64, logs ...types.Log) {
		t.Helper()
		next := common.BigToHash(new(big.Int).SetUint64(height + 1000))
		for i := range logs {
			logs[i].Address = proxy
			logs[i].BlockNumber = height
			logs[i].BlockHash = next
			logs[i].TxHash = common.BigToHash(new(big.Int).SetUint64(height + 2000))
			if height == 14 {
				logs[i].TxHash = signedActivation.Hash()
			}
			logs[i].Index = 1 + uint(i)
		}
		states := map[uint64]idov3whitelist.ProjectState{12: idov3whitelist.ProjectPending, 13: idov3whitelist.ProjectPending, 14: idov3whitelist.ProjectActive, 15: idov3whitelist.ProjectActive, 16: idov3whitelist.ProjectSuccessful, 17: idov3whitelist.ProjectSuccessful}
		state, known := states[height]
		if !known {
			t.Fatal("synthetic checked project fixture missing state")
		}
		raised, sold := "0", "0"
		if height >= 15 {
			raised, sold = "600", "300"
		}
		observation := idov3whitelist.ProjectIdentityObservation{ChainID: binding.ChainID, ProxyAddress: proxy, ProjectID: binding.ProjectID, ProjectToken: binding.ProjectToken, PaymentToken: binding.PaymentToken,
			ProjectOwner: owner, FeeReceiver: owner, ProjectDecimals: 1, PaymentDecimals: 1, FeeRate: 200, TokenPriceRaw: "20", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", StartTimeRaw: "1", EndTimeRaw: "100",
			BlockNumber: height, BlockHash: next, State: state, RaisedAtBlockRaw: raised, TokensSoldAtBlockRaw: sold}
		if err := journal.applyChecked(ctx, binding, height, latest, next, logs, observation); err != nil {
			t.Fatalf("synthetic event block %d rejected: %v", height, err)
		}
		latest = next
	}
	create := types.Log{Topics: []common.Hash{sig("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)"), projectTopic, common.BytesToHash(binding.ProjectToken.Bytes()), common.BytesToHash(binding.PaymentToken.Bytes())}}
	create.Data = append(create.Data, addrWord(owner)...)
	create.Data = append(create.Data, addrWord(owner)...)
	for _, v := range []uint64{1, 1, 200, 20, 1000, 500, 900, 1, 100} {
		create.Data = append(create.Data, word(v)...)
	}
	apply(12, create)
	// The initial successful checked block is now tied to exactly one
	// canonical ProjectCreated row and its checkpoint. A changed immutable
	// field must roll back the entire next empty block.
	badHash := common.HexToHash("0xc2")
	bad := idov3whitelist.ProjectIdentityObservation{ChainID: binding.ChainID, ProxyAddress: proxy, ProjectID: binding.ProjectID, PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
		ProjectOwner: owner, FeeReceiver: owner, ProjectDecimals: 1, PaymentDecimals: 1, FeeRate: 200, TokenPriceRaw: "21", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", StartTimeRaw: "1", EndTimeRaw: "100",
		BlockNumber: 13, BlockHash: badHash, State: idov3whitelist.ProjectPending, RaisedAtBlockRaw: "0", TokensSoldAtBlockRaw: "0"}
	if err := journal.applyChecked(ctx, binding, 13, latest, badHash, nil, bad); err == nil {
		t.Fatal("later project price drift published")
	}
	readCursor()
	if cursor != 12 {
		t.Fatal("immutable drift advanced cursor")
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=? AND block_number=13`, binding.ProjectID).Scan(&checkpoints).Error != nil || checkpoints != 0 {
		t.Fatal("drift retained checkpoint")
	}
	deposit := types.Log{Topics: []common.Hash{sig("ProjectTokensDeposited(uint256,address,uint256)"), projectTopic, common.BytesToHash(binding.ProjectToken.Bytes())}, Data: word(400)}
	apply(13, deposit)
	seed(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,manifest_digest,create_calldata_hash,operator_address)
		VALUES (?,31337,?,CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),CONCAT('0x',REPEAT('c',64)),?)`, binding.ProjectID, v3Lower(proxy), v3Lower(operator))
	seed(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,contract_address,call_data,state,nonce,gas_limit,gas_price_raw,raw_tx,tx_hash,active_slot,last_error)
		VALUES (?,'ido_v3_activate',1,31337,?, ?,?,'signed',7,150000,'1',?,?,1,'')`, binding.ProjectID, operator.Hex(), v3Lower(proxy), activateCall, activationRaw, signedActivation.Hash().Hex())
	var attemptID uint64
	if read := db.Raw(`SELECT id FROM chain_tx_submission WHERE project_id=? AND action='ido_v3_activate'`, binding.ProjectID).Scan(&attemptID); read.Error != nil || read.RowsAffected != 1 || attemptID == 0 {
		t.Fatal("V3 signed activation fixture unavailable")
	}
	seed(`INSERT INTO chain_relayer_nonce(chain_id,sender_address,next_nonce,active_submission_id) VALUES(31337,?,8,?)`, operator.Hex(), attemptID)
	activate := types.Log{Topics: []common.Hash{sig("ProjectActivated(uint256)"), projectTopic}}
	apply(14, activate)
	var applied struct {
		State   string  `gorm:"column:state"`
		EventID *uint64 `gorm:"column:final_event_record_id"`
	}
	if read := db.Raw(`SELECT state,final_event_record_id FROM chain_tx_submission WHERE id=?`, attemptID).Scan(&applied); read.Error != nil || read.RowsAffected != 1 || applied.State != constant.ChainTxStateApplied || applied.EventID == nil || *applied.EventID == 0 {
		t.Fatalf("V3 activation not atomically applied: %+v", applied)
	}
	var activeFence int64
	if err := db.Raw(`SELECT COUNT(*) FROM chain_relayer_nonce WHERE active_submission_id=?`, attemptID).Scan(&activeFence).Error; err != nil || activeFence != 0 {
		t.Fatal("V3 canonical activation did not release the exact sender fence")
	}
	purchase := types.Log{Topics: []common.Hash{sig("Purchased(uint256,address,address,uint256,address,uint256)"), projectTopic, common.BytesToHash(buyer.Bytes()), common.BytesToHash(binding.PaymentToken.Bytes())}}
	purchase.Data = append(purchase.Data, word(600)...)
	purchase.Data = append(purchase.Data, addrWord(binding.ProjectToken)...)
	purchase.Data = append(purchase.Data, word(300)...)
	apply(15, purchase)
	var financialRows int64
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number=15 AND entry_kind='purchase' AND status='canonical'`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&financialRows).Error; err != nil || financialRows != 2 {
		t.Fatalf("purchase payment and entitlement must be distinct canonical effects: count=%d err=%v", financialRows, err)
	}
	end := types.Log{Topics: []common.Hash{sig("ProjectEnded(uint256,uint8,address,uint256)"), projectTopic, common.BytesToHash(binding.PaymentToken.Bytes())}, Data: append(word(2), word(600)...)}
	apply(16, end)
	claim := types.Log{Topics: []common.Hash{sig("Claimed(uint256,address,address,uint256)"), projectTopic, common.BytesToHash(buyer.Bytes()), common.BytesToHash(binding.ProjectToken.Bytes())}, Data: word(100)}
	apply(17, claim)
	var business string
	if err := db.Raw(`SELECT status FROM projects WHERE project_id=? AND chain_id=? AND contract_address=?`, binding.ProjectID, binding.ChainID, v3Lower(proxy)).Scan(&business).Error; err != nil || business != "Successful" {
		t.Fatalf("verified settlement did not advance V3 business: status=%s err=%v", business, err)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number=17 AND entry_kind='Claimed' AND status='canonical'`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&financialRows).Error; err != nil || financialRows != 1 {
		t.Fatalf("claim must have one canonical project-token effect: count=%d err=%v", financialRows, err)
	}
	var projection struct {
		State   string `gorm:"column:state"`
		Raised  string `gorm:"column:raised_raw"`
		Claimed string `gorm:"column:claimed_raw"`
		Wallets string `gorm:"column:wallets"`
	}
	readProjection := func() {
		t.Helper()
		if err := db.Raw(`SELECT state,raised_raw,claimed_raw,wallets FROM ido_v3_project_projection WHERE chain_id=31337 AND proxy_address=? AND project_id=?`, v3Lower(proxy), binding.ProjectID).Scan(&projection).Error; err != nil {
			t.Fatal(err)
		}
	}
	readProjection()
	if projection.State != "successful" || projection.Raised != "600" || projection.Claimed != "100" || !strings.Contains(projection.Wallets, v3Lower(buyer)) {
		t.Fatalf("wallet position not materialized: %+v", projection)
	}
	if wallets, err := readV3PublishedProjection(ctx, db, binding); err != nil || !strings.Contains(wallets, v3Lower(buyer)) {
		t.Fatalf("running checkpoint projection not internally readable: %v", err)
	}
	if err := journal.haltMismatch(ctx, binding, 17, latest); err != nil {
		t.Fatalf("published wallet revocation failed: %v", err)
	}
	if read := db.Raw(`SELECT state FROM chain_tx_submission WHERE id=?`, attemptID).Scan(&applied.State); read.Error != nil || applied.State != "v3_reorg_suspended" {
		t.Fatalf("unknown canonicality did not suspend signed attempt: %s", applied.State)
	}
	if err := db.Raw(`SELECT status FROM projects WHERE project_id=?`, binding.ProjectID).Scan(&business).Error; err != nil || business != v3BusinessHalted {
		t.Fatalf("unknown canonicality left successful V3 executable: status=%s err=%v", business, err)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND status='canonical'`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&financialRows).Error; err != nil || financialRows != 0 {
		t.Fatalf("unknown reorg left canonical financial facts: count=%d err=%v", financialRows, err)
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_project_projection WHERE chain_id=31337 AND project_id=?`, binding.ProjectID).Scan(&events).Error != nil || events != 0 {
		t.Fatal("published claim survived mismatch halt")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("halted projection was internally readable")
	}
	if err := journal.apply(ctx, binding, 18, latest, common.HexToHash("0xa5"), nil); err == nil {
		t.Fatal("halted wallet projection advanced")
	}
	ancestor := common.BigToHash(big.NewInt(1015))
	if err := journal.rollback(ctx, binding, 15, ancestor, readRevision()); err != nil {
		t.Fatalf("claim reorg rollback rejected: %v", err)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number>15 AND status='canonical'`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&financialRows).Error; err != nil || financialRows != 0 {
		t.Fatalf("orphaned financial effects remain canonical: count=%d err=%v", financialRows, err)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number>15 AND status='orphaned'`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&financialRows).Error; err != nil || financialRows != 1 {
		t.Fatalf("claim effect not retained as orphan evidence: count=%d err=%v", financialRows, err)
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_project_projection WHERE chain_id=31337 AND project_id=?`, binding.ProjectID).Scan(&events).Error != nil || events != 0 {
		t.Fatal("orphaned claim still published")
	}
	if err := journal.resumeAtAncestor(ctx, binding, 15, ancestor, readRevision()); err != nil {
		t.Fatalf("surviving wallet prefix rejected: %v", err)
	}
	if read := db.Raw(`SELECT state FROM chain_tx_submission WHERE id=?`, attemptID).Scan(&applied.State); read.Error != nil || applied.State != constant.ChainTxStateApplied {
		t.Fatalf("surviving signed activation was not restored: %s", applied.State)
	}
	readProjection()
	if projection.State != "active" || projection.Raised != "600" || projection.Claimed != "0" {
		t.Fatalf("orphan claim survived rebuild: %+v", projection)
	}
	if err := db.Raw(`SELECT status FROM projects WHERE project_id=?`, binding.ProjectID).Scan(&business).Error; err != nil || business != "Active" {
		t.Fatalf("verified surviving prefix did not restore active status: status=%s err=%v", business, err)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number=15 AND status='canonical'`, binding.ChainID, v3Lower(proxy), binding.ProjectID).Scan(&financialRows).Error; err != nil || financialRows != 2 {
		t.Fatalf("surviving purchase facts not restored: count=%d err=%v", financialRows, err)
	}
}

// No approved source is present: this is a network-isolated atomicity test
// for the unregistered creation bootstrap, not an admission test.
func TestV3BootstrapDisposableMySQLAtomicCreation(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable MySQL unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable V3 MySQL")
	}
	proxy := common.HexToAddress("0x4444444444444444444444444444444444444444")
	sale := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	impl := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 99, ProjectToken: sale, PaymentToken: pay}
	if err := db.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,description,tags,team_info,roadmap,partners)
		VALUES(99,'Pending','capped_presale','ido_v3_stablecoin',31337,?,1,?,?,'','','','','')`, v3Lower(proxy), v3Lower(sale), v3Lower(pay)).Error; err != nil {
		t.Fatal("disposable project seed failed")
	}
	if err := db.Exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(99,31337,?)`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable binding seed failed")
	}
	parentHead := &types.Header{Number: big.NewInt(89)}
	head := &types.Header{Number: big.NewInt(90), ParentHash: parentHead.Hash()}
	addr, _ := abi.NewType("address", "", nil)
	u8, _ := abi.NewType("uint8", "", nil)
	u16, _ := abi.NewType("uint16", "", nil)
	u256, _ := abi.NewType("uint256", "", nil)
	fields := abi.Arguments{{Type: addr}, {Type: addr}, {Type: u8}, {Type: u8}, {Type: u16}}
	for i := 0; i < 6; i++ {
		fields = append(fields, abi.Argument{Type: u256})
	}
	data, err := fields.Pack(sale, pay, uint8(18), uint8(18), uint16(200), big.NewInt(20), big.NewInt(1000), big.NewInt(500), big.NewInt(900), big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}
	create := types.Log{Address: proxy, BlockNumber: 90, BlockHash: head.Hash(), TxHash: common.HexToHash("0xb1"), Index: 3, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")), common.BigToHash(big.NewInt(99)), common.BytesToHash(sale.Bytes()), common.BytesToHash(pay.Bytes())}, Data: data}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	stub := &v3PinnedScanStub{chain: 31337, head: head, parentHead: parentHead, proxy: proxy, impl: impl, payment: pay, sale: sale, project: 99, tuple: v3PendingTuple(99, sale, pay), logs: []types.Log{create}, receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: head.Hash(), TxHash: create.TxHash, Logs: []*types.Log{&create}}}
	ctx := context.Background()
	postCommit := &v3BootstrapSwitchAfterCommit{v3PinnedScanStub: stub, db: db, binding: binding}
	if err := v3BootstrapCreation(ctx, db, postCommit, binding, runtime, 90, 3); err == nil || !postCommit.switched {
		t.Fatalf("post-commit creation replacement not rejected: switched=%v err=%v", postCommit.switched, err)
	}
	var halted string
	if read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&halted); read.Error != nil || read.RowsAffected != 1 || halted != "reorg_halted" {
		t.Fatalf("post-commit replacement did not halt cursor: state=%s err=%v", halted, read.Error)
	}
	var lockedProject struct {
		Status string `gorm:"column:status"`
		Locked uint8  `gorm:"column:onchain_locked"`
	}
	if read := db.Raw(`SELECT status,onchain_locked FROM projects WHERE project_id=99 AND chain_id=31337 AND contract_address=?`, v3Lower(proxy)).Scan(&lockedProject); read.Error != nil || read.RowsAffected != 1 || lockedProject.Status != v3BusinessHalted || lockedProject.Locked != 1 {
		t.Fatalf("creation replacement incorrectly reopened bound V3 project: %+v", lockedProject)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("post-commit replacement retained internal projection")
	}
	// Only this network-isolated fixture may reset its journal and fake RPC.
	resetBootstrap := func() {
		t.Helper()
		if err := db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error; err != nil ||
			db.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=31337 AND project_id=99`).Error != nil ||
			db.Exec(`DELETE FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=99`).Error != nil ||
			db.Exec(`DELETE FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=99`).Error != nil ||
			db.Exec(`DELETE FROM ido_v3_scan_cursor WHERE chain_id=31337 AND project_id=99`).Error != nil {
			t.Fatal("disposable bootstrap reset failed")
		}
	}
	resetBootstrap()
	stub.head = head
	unknown := &v3BootstrapSwitchAfterCommit{v3PinnedScanStub: stub, db: db, binding: binding, unavailable: true}
	if err := v3BootstrapCreation(ctx, db, unknown, binding, runtime, 90, 3); err == nil || !unknown.switched {
		t.Fatalf("post-commit unknown canonicality accepted: switched=%v err=%v", unknown.switched, err)
	}
	if read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&halted); read.Error != nil || read.RowsAffected != 1 || halted != "reorg_halted" {
		t.Fatalf("post-commit unknown left running projection: state=%s err=%v", halted, read.Error)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("post-commit RPC outage retained internal projection")
	}
	var canonical int64
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=99 AND status='canonical'`).Scan(&canonical).Error; err != nil || canonical != 1 {
		t.Fatal("unknown RPC incorrectly orphaned initial journal")
	}
	resetBootstrap()
	lostDepth := &v3BootstrapSwitchAfterCommit{v3PinnedScanStub: stub, db: db, binding: binding, loseDepth: true}
	if err := v3BootstrapCreation(ctx, db, lostDepth, binding, runtime, 90, 3); err == nil || !lostDepth.switched {
		t.Fatalf("post-commit shallow head accepted: switched=%v err=%v", lostDepth.switched, err)
	}
	if read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&halted); read.Error != nil || read.RowsAffected != 1 || halted != "reorg_halted" {
		t.Fatalf("post-commit shallow head retained projection: state=%s err=%v", halted, read.Error)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("post-commit shallow head retained internal projection")
	}
	resetBootstrap()
	stub.head = head
	parentLost := &v3BootstrapSwitchAfterCommit{v3PinnedScanStub: stub, db: db, binding: binding, loseParent: true}
	if err := v3BootstrapCreation(ctx, db, parentLost, binding, runtime, 90, 3); err == nil || !parentLost.switched {
		t.Fatalf("post-commit parent outage accepted: switched=%v err=%v", parentLost.switched, err)
	}
	if read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&halted); read.Error != nil || read.RowsAffected != 1 || halted != "reorg_halted" {
		t.Fatalf("post-commit parent outage retained projection: state=%s err=%v", halted, read.Error)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("post-commit parent outage retained internal projection")
	}
	resetBootstrap()
	stub.head = head
	if err := v3BootstrapCreation(ctx, db, stub, binding, runtime, 90, 3); err != nil {
		t.Fatalf("synthetic bootstrap rejected: %v", err)
	}
	var row struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Revision uint64 `gorm:"column:revision"`
		Hash     string `gorm:"column:cursor_hash"`
	}
	if read := db.Raw(`SELECT cursor_block,revision,cursor_hash FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&row); read.Error != nil || read.RowsAffected != 1 || row.Block != 90 || row.Revision != 1 || row.Hash != v3LowerHash(head.Hash()) {
		t.Fatal("bootstrap did not atomically advance cursor")
	}
	var checkpoints, events int
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=99 AND status='canonical'`).Scan(&checkpoints).Error != nil || checkpoints != 2 || db.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=99 AND status='canonical'`).Scan(&events).Error != nil || events != 1 {
		t.Fatal("bootstrap journal/checkpoint missing")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err != nil {
		t.Fatal("bootstrap checked projection not readable internally")
	}
	if count, err := v3ScanConfirmedRange(ctx, db, v3CursorChainUnavailable{stub}, binding, runtime, 3, 1); err == nil || count != 0 {
		t.Fatalf("unknown initial chain accepted: count=%d err=%v", count, err)
	}
	var monitoredState string
	if read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&monitoredState); read.Error != nil || read.RowsAffected != 1 || monitoredState != "reorg_halted" {
		t.Fatalf("unknown initial chain retained publication: state=%s err=%v", monitoredState, read.Error)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("unknown initial chain left wallet projection readable")
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=99 AND status='canonical'`).Scan(&canonical).Error; err != nil || canonical != 1 {
		t.Fatal("unknown initial chain orphaned a creation fact")
	}
	resetBootstrap()
	if err := v3BootstrapCreation(ctx, db, stub, binding, runtime, 90, 3); err != nil {
		t.Fatalf("disposable chain outage re-bootstrap failed: %v", err)
	}
	// A later monitoring tick may lose only the cursor header RPC. It must
	// revoke the already committed projection without turning this unknown
	// answer into an orphaned creation or a guessed ancestor.
	if count, err := v3ScanConfirmedRange(ctx, db, v3CursorHeaderUnavailable{stub}, binding, runtime, 3, 1); err == nil || count != 0 {
		t.Fatalf("unknown initial cursor header accepted: count=%d err=%v", count, err)
	}
	if read := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&monitoredState); read.Error != nil || read.RowsAffected != 1 || monitoredState != "reorg_halted" {
		t.Fatalf("unknown initial header retained publication: state=%s err=%v", monitoredState, read.Error)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("unknown initial header left wallet projection readable")
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=99 AND status='canonical'`).Scan(&canonical).Error; err != nil || canonical != 1 {
		t.Fatal("unknown initial header orphaned a creation fact")
	}
	// Re-bootstrap fresh fixture state rather than rewriting a halted cursor's
	// revision; the actual recovery path remains deliberately unregistered.
	resetBootstrap()
	if err := v3BootstrapCreation(ctx, db, stub, binding, runtime, 90, 3); err != nil {
		t.Fatalf("disposable precheck re-bootstrap failed: %v", err)
	}
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err != nil {
		t.Fatalf("synthetic surviving creation prefix rejected: %v", err)
	}
	if err := db.Exec(`UPDATE ido_v3_event_journal SET event_kind='purchase' WHERE chain_id=31337 AND project_id=99 AND block_number=90`).Error; err != nil {
		t.Fatal("disposable journal corruption seed failed")
	}
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil {
		t.Fatal("corrupted creation kind accepted as surviving prefix")
	}
	if err := db.Exec(`UPDATE ido_v3_event_journal SET event_kind='created' WHERE chain_id=31337 AND project_id=99 AND block_number=90`).Error; err != nil {
		t.Fatal("disposable journal corruption reset failed")
	}
	if err := db.Exec(`UPDATE ido_v3_block_checkpoint SET block_hash=? WHERE chain_id=31337 AND project_id=99 AND block_number=90 AND status='canonical'`, v3LowerHash(common.HexToHash("0xbeef"))).Error; err != nil {
		t.Fatal("disposable checkpoint corruption seed failed")
	}
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil {
		t.Fatal("corrupted checkpoint accepted as surviving prefix")
	}
	if err := db.Exec(`UPDATE ido_v3_block_checkpoint SET block_hash=? WHERE chain_id=31337 AND project_id=99 AND block_number=90 AND status='canonical'`, v3LowerHash(head.Hash())).Error; err != nil {
		t.Fatal("disposable checkpoint reset failed")
	}
	stub.changeReceipt = true
	stub.receiptReads = 0
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil {
		t.Fatal("changed surviving receipt accepted")
	}
	stub.changeReceipt = false
	stub.changeThirdReceipt = true
	stub.receiptReads = 0
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil {
		t.Fatal("changed third recovery receipt accepted")
	}
	stub.changeThirdReceipt = false
	stub.changeNthReceipt = 4 // only after the whole prefix/anchors have been reread
	stub.receiptReads = 0
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil {
		t.Fatal("changed final recovery receipt accepted")
	}
	stub.changeNthReceipt = 0
	stub.receiptReads = 0
	stub.changeNthFilter = 4 // initial two, post-project third, final full-prefix fourth
	stub.filters = 0
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil {
		t.Fatal("changed final recovery filter accepted")
	}
	stub.changeNthFilter = 0
	stub.filters = 0
	stub.projectReads = 0
	stub.changeProjectAfter = 3 // scan, main prefix read, final full-prefix read
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, head.Hash()); err == nil || !strings.Contains(err.Error(), "final project changed") {
		t.Fatalf("late immutable project drift accepted: %v", err)
	}
	stub.changeProjectAfter = 0
	stub.projectReads = 0
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 90, 3, common.HexToHash("0xff")); err == nil {
		t.Fatal("wrong ancestor accepted")
	}
	if err := v3BootstrapCreation(ctx, db, stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("duplicate bootstrap accepted")
	}
	if db.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=31337 AND project_id=99`).Scan(&events).Error != nil || events != 1 {
		t.Fatal("duplicate bootstrap changed journal")
	}
	// The first post-creation event-free block must use the persisted creation
	// fact and advance only with a matching canonical parent checkpoint.
	stub.parentHead = head
	stub.head = &types.Header{Number: big.NewInt(91), ParentHash: head.Hash()}
	stub.logs = nil
	creationForRange := *stub
	creationForRange.head = head
	creationForRange.parentHead = parentHead
	creationForRange.logs = []types.Log{create}
	nextForRange := *stub
	historyForRange := v3TwoBlockHistory{creation: &creationForRange, next: &nextForRange}
	if count, err := v3ScanConfirmedRange(ctx, db, historyForRange, binding, runtime, 3, 1); err != nil || count != 1 {
		t.Fatalf("bounded continuous synthetic empty scan rejected: count=%d err=%v", count, err)
	}
	if read := db.Raw(`SELECT cursor_block,revision,cursor_hash FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&row); read.Error != nil || read.RowsAffected != 1 || row.Block != 91 || row.Revision != 2 || row.Hash != v3LowerHash(stub.head.Hash()) {
		t.Fatal("continuous scan did not persist empty checkpoint")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err != nil {
		t.Fatal("continuous projection unavailable internally")
	}
	if count, err := v3ScanConfirmedRange(ctx, db, historyForRange, binding, runtime, 4, 1); err != nil || count != 0 {
		t.Fatalf("range crossed confirmation boundary: count=%d err=%v", count, err)
	}
	if read := db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&row); read.Error != nil || row.Block != 91 {
		t.Fatal("no-new-block range advanced cursor")
	}
	idleUnknown := &v3RangeUnknownOnIdleRecheck{v3TwoBlockHistory: historyForRange, cursor: 91}
	if count, err := v3ScanConfirmedRange(ctx, db, idleUnknown, binding, runtime, 4, 1); err == nil || count != 0 || idleUnknown.reads != 2 {
		t.Fatalf("idle cursor header outage accepted: count=%d reads=%d err=%v", count, idleUnknown.reads, err)
	}
	var idleState string
	if err := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&idleState).Error; err != nil || idleState != "reorg_halted" {
		t.Fatalf("idle canonical outage retained projection: state=%s err=%v", idleState, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("idle canonical outage retained internal projection")
	}
	// Restore only the isolated fixture's cursor; no production recovery path
	// may directly force a halted cursor back into running state.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable idle-outage cursor reset failed")
	}
	var runningState string
	if count, err := v3ScanConfirmedRange(ctx, db, v3RangeRevisionOnHead{historyForRange, db, binding, 91}, binding, runtime, 4, 1); err == nil || count != 0 {
		t.Fatalf("idle scan accepted concurrently revised cursor: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&runningState).Error; err != nil || runningState != "running" {
		t.Fatalf("revision change wrongly halted cursor: state=%s err=%v", runningState, err)
	}
	if count, err := v3ScanConfirmedRange(ctx, db, v3RangeRevisionOnHead{historyForRange, db, binding, 91}, binding, runtime, 3, 1); err == nil || count != 0 || !strings.Contains(err.Error(), "cursor changed") {
		t.Fatalf("active scan accepted stale start revision: count=%d err=%v", count, err)
	}
	if count, err := v3ScanConfirmedRange(ctx, db, v3RangeShallowHead{historyForRange}, binding, runtime, 3, 1); err == nil || count != 0 || !strings.Contains(err.Error(), "cursor confirmations") {
		t.Fatalf("already published cursor below confirmation depth accepted: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&runningState).Error; err != nil || runningState != "reorg_halted" {
		t.Fatalf("lost cursor depth retained projection: state=%s err=%v", runningState, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("lost cursor depth left internal projection readable")
	}
	// Isolated fixture reset only; production requires authenticated recovery.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable shallow-head cursor reset failed")
	}
	if count, err := v3ScanConfirmedRange(ctx, db, v3RangeLaggingHead{historyForRange}, binding, runtime, 3, 1); err == nil || count != 0 {
		t.Fatalf("provider head behind persisted cursor accepted: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&runningState).Error; err != nil || runningState != "reorg_halted" {
		t.Fatalf("lagging head retained internal publication: state=%s err=%v", runningState, err)
	}
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable lagging-head cursor reset failed")
	}
	if count, err := v3ScanConfirmedRange(ctx, db, v3RangeUnavailableHead{historyForRange}, binding, runtime, 3, 1); err == nil || count != 0 {
		t.Fatalf("unavailable head allowed old published view: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&runningState).Error; err != nil || runningState != "reorg_halted" {
		t.Fatalf("unavailable head retained publication: state=%s err=%v", runningState, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("head outage retained internal projection")
	}
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable unavailable-head cursor reset failed")
	}
	creationStub := *stub
	creationStub.head = head
	creationStub.parentHead = parentHead
	creationStub.logs = []types.Log{create}
	creationStub.receipt = &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: head.Hash(), TxHash: create.TxHash, Logs: []*types.Log{&create}}
	creationStub.receiptReads = 0
	nextStub := *stub
	nextStub.logs = nil
	nextStub.receipt = nil
	nextStub.receiptReads = 0
	history := v3TwoBlockHistory{creation: &creationStub, next: &nextStub}
	if err := v3CheckSurvivingPrefix(ctx, db, history, binding, runtime, 90, 91, 3, stub.head.Hash()); err != nil {
		t.Fatalf("contiguous surviving empty block prefix rejected: %v", err)
	}
	stub.logs = []types.Log{create}
	stub.receipt = &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: head.Hash(), TxHash: create.TxHash, Logs: []*types.Log{&create}}
	stub.head = head
	stub.parentHead = parentHead
	stub.receiptReads = 0
	if err := v3CheckSurvivingPrefix(ctx, db, stub, binding, runtime, 90, 91, 3, common.HexToHash(row.Hash)); err == nil {
		t.Fatal("missing intermediate header accepted")
	}
	row.Hash = v3LowerHash(nextForRange.head.Hash())
	// Two receipt reads happen in v3ScanBlock. The checked path reads the
	// project again, then must reject a changed third receipt without publishing
	// the next checkpoint or advancing the durable cursor.
	stub.head = &types.Header{Number: big.NewInt(92), ParentHash: common.HexToHash(row.Hash)}
	stub.parentHead = &types.Header{Number: big.NewInt(91), ParentHash: head.Hash()}
	stub.logs = []types.Log{{Address: proxy, BlockNumber: 92, BlockHash: stub.head.Hash(), TxHash: common.HexToHash("0xb2"), Index: 3, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")), common.BigToHash(big.NewInt(99)), common.BytesToHash(sale.Bytes())}, Data: common.LeftPadBytes(big.NewInt(400).Bytes(), 32)}}
	stub.receipt = &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(92), BlockHash: stub.head.Hash(), TxHash: stub.logs[0].TxHash, Logs: []*types.Log{&stub.logs[0]}}
	stub.receiptReads = 0
	stub.changeThirdReceipt = true
	if err := v3ScanCheckedBlock(ctx, db, stub, binding, runtime, 92, 3); err == nil {
		t.Fatal("changed post-project receipt published")
	}
	if read := db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&row); read.Error != nil || row.Block != 91 {
		t.Fatal("changed receipt advanced cursor")
	}
	stub.changeThirdReceipt = false
	stub.changeFinalParent = true
	stub.parentReads = 0
	stub.logs = nil
	if err := v3ScanCheckedBlock(ctx, db, stub, binding, runtime, 92, 3); err == nil || !strings.Contains(err.Error(), "parent changed") {
		t.Fatalf("changed parent at final read accepted: %v", err)
	}
	if read := db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&row); read.Error != nil || row.Block != 91 {
		t.Fatal("changed parent advanced cursor")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("changed canonical parent retained readable projection")
	}
	// This synthetic test continues to exercise the other cursor-halt path:
	// restore the old cursor state without restoring any wallet entitlement.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable cursor test reset failed")
	}
	stub.changeFinalParent = false
	stub.parentHead = head
	stub.head = &types.Header{Number: big.NewInt(91), ParentHash: head.Hash(), Extra: []byte{1}}
	if err := v3ScanCheckedBlock(ctx, db, stub, binding, runtime, 92, 3); err == nil {
		t.Fatal("reorganized empty block kept published projection")
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("reorg halt retained readable projection")
	}
	// Reset only this disposable fixture's halted cursor; then move the
	// canonical head between the range driver's first check and idle return.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil {
		t.Fatal("disposable late reorg cursor reset failed")
	}
	nextForRange.head = &types.Header{Number: big.NewInt(91), ParentHash: head.Hash()}
	late := v3RangeSwitchOnHead{v3TwoBlockHistory{creation: &creationForRange, next: &nextForRange}}
	if count, err := v3ScanConfirmedRange(ctx, db, late, binding, runtime, 4, 1); err == nil || count != 0 {
		t.Fatalf("late reorg during idle head read accepted: count=%d err=%v", count, err)
	}
	var state string
	if err := db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&state).Error; err != nil || state != "reorg_halted" {
		t.Fatalf("late reorg failed to halt cursor: state=%s err=%v", state, err)
	}
	// Isolate the final-read race in this disposable fixture: the prior halt
	// revoked the projection, but never altered the persisted 91 checkpoint.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil ||
		db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error != nil {
		t.Fatal("disposable final-read cursor reset failed")
	}
	previous := nextForRange
	previous.head = &types.Header{Number: big.NewInt(91), ParentHash: head.Hash()}
	previous.parentHead = head
	previous.logs = nil
	after := previous
	after.head = &types.Header{Number: big.NewInt(92), ParentHash: previous.head.Hash()}
	after.parentHead = previous.head
	after.logs = nil
	finalRead := &v3RangeSwitchAfterChecked{v3TwoBlockHistory: v3TwoBlockHistory{creation: &previous, next: &after}, endHeight: 92, db: db, binding: binding}
	if count, err := v3ScanConfirmedRange(ctx, db, finalRead, binding, runtime, 3, 1); err == nil || count != 1 {
		t.Fatalf("replaced final checked block reported range success: count=%d err=%v", count, err)
	}
	var final struct {
		Block uint64 `gorm:"column:cursor_block"`
		State string `gorm:"column:state"`
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 92 || final.State != "reorg_halted" {
		t.Fatalf("final read failed to halt committed block: %+v err=%v", final, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("final read replacement retained published projection")
	}
	// A stable canonical header plus a head that no longer offers the
	// configured depth is an unknown finality failure, not reorg proof.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil ||
		db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error != nil {
		t.Fatal("disposable final-confirmation cursor reset failed")
	}
	afterAgain := after
	afterAgain.head = &types.Header{Number: big.NewInt(93), ParentHash: after.head.Hash()}
	afterAgain.parentHead = after.head
	afterAgain.logs = nil
	confirmations := v3RangeDropsConfirmations{v3TwoBlockHistory: v3TwoBlockHistory{creation: &after, next: &afterAgain}, db: db, binding: binding, replace: true}
	if count, err := v3ScanConfirmedRange(ctx, db, confirmations, binding, runtime, 3, 1); err == nil || count != 1 || !strings.Contains(err.Error(), "final confirmations") {
		t.Fatalf("reduced depth after checked block accepted: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 93 || final.State != "reorg_halted" {
		t.Fatalf("low-head replacement failed to revoke cursor: %+v err=%v", final, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("low-head replacement retained internal projection")
	}
	// Return only the disposable cursor/header to the previously committed
	// 93 branch for subsequent partial-failure testing.
	afterAgain.head = &types.Header{Number: big.NewInt(93), ParentHash: after.head.Hash()}
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil ||
		db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error != nil {
		t.Fatal("disposable low-head cursor reset failed")
	}
	// Two-block range: a valid empty 94 commits before malformed 95 fails.
	// The next invocation must resume at 95, not skip it or re-apply 94.
	first := afterAgain
	first.head = &types.Header{Number: big.NewInt(94), ParentHash: afterAgain.head.Hash()}
	first.parentHead = afterAgain.head
	first.logs = nil
	second := first
	second.head = &types.Header{Number: big.NewInt(95), ParentHash: first.head.Hash()}
	second.parentHead = first.head
	second.logs = []types.Log{{Address: proxy, BlockNumber: 95, BlockHash: second.head.Hash(), TxHash: common.HexToHash("0xb3"), Index: 1,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("UnknownProjectEvent(uint256)")), common.BigToHash(big.NewInt(99))}}}
	rangeRPC := v3TwoBlockHistory{creation: &first, next: &second}
	partialReplacement := &v3RangeSwitchOnPartialFailure{v3TwoBlockHistory: rangeRPC, committed: first.head}
	if count, err := v3ScanConfirmedRange(ctx, db, partialReplacement, binding, runtime, 3, 2); err == nil || count != 1 || !partialReplacement.switched {
		t.Fatalf("malformed second height skipped or first height not committed: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 94 || final.State != "reorg_halted" {
		t.Fatalf("partial replacement did not revoke committed cursor: %+v err=%v", final, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("partial replacement retained internal projection")
	}
	var nextCheckpoint int64
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND proxy_address=? AND project_id=99 AND block_number=95 AND status='canonical'`, v3Lower(proxy)).Scan(&nextCheckpoint).Error; err != nil || nextCheckpoint != 0 {
		t.Fatalf("failed second height left checkpoint: count=%d err=%v", nextCheckpoint, err)
	}
	// Restore only this disposable fixture's header and cursor for the
	// subsequent exact-height retry; a production halt requires recovery.
	first.head = partialReplacement.committed
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil ||
		db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error != nil {
		t.Fatal("disposable partial replacement cursor reset failed")
	}
	second.logs = nil
	// The first pass rejected before the final project read. Clear the
	// one-block fake's read counter so its subsequent fixed-block answers
	// are stable; this is a fresh provider invocation, not an admitted source.
	second.projectReads = 0
	second.filters = 0
	// The fixture only models through 95; cap the second invocation to one
	// block so it cannot request the intentionally absent height 96.
	if count, err := v3ScanConfirmedRange(ctx, db, rangeRPC, binding, runtime, 3, 1); err != nil || count != 1 {
		t.Fatalf("range failed to resume exactly at second height: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 95 || final.State != "running" {
		t.Fatalf("resumed range did not advance exactly once: %+v err=%v", final, err)
	}
	third := second
	third.head = &types.Header{Number: big.NewInt(96), ParentHash: second.head.Hash()}
	third.parentHead = second.head
	third.logs = nil
	lateRevision := v3RangeRevisionAfterApply{v3RangeRevisionOnHead{v3TwoBlockHistory{creation: &second, next: &third}, db, binding, 96}}
	if count, err := v3ScanConfirmedRange(ctx, db, lateRevision, binding, runtime, 3, 1); err == nil || count != 1 || !strings.Contains(err.Error(), "final cursor changed") {
		t.Fatalf("revision changed after last apply was reported successful: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 96 || final.State != "running" {
		t.Fatalf("revision-only race incorrectly halted cursor: %+v err=%v", final, err)
	}
	// Even a successful final head response can race with a canonical
	// replacement after the range driver's first post-commit header check.
	fourth := third
	fourth.head = &types.Header{Number: big.NewInt(97), ParentHash: third.head.Hash()}
	fourth.parentHead = third.head
	fourth.logs = nil
	switchOnFinalHead := &v3RangeSwitchOnFinalHead{v3TwoBlockHistory: v3TwoBlockHistory{creation: &third, next: &fourth}, endHeight: 97, db: db, binding: binding}
	if count, err := v3ScanConfirmedRange(ctx, db, switchOnFinalHead, binding, runtime, 3, 1); err == nil || count != 1 || !switchOnFinalHead.switched {
		t.Fatalf("final-head replacement reported successful range: count=%d switched=%v err=%v", count, switchOnFinalHead.switched, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 97 || final.State != "reorg_halted" {
		t.Fatalf("final-head replacement did not revoke cursor: %+v err=%v", final, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("final-head replacement retained internal projection")
	}
	// Start a fresh synthetic post-recovery branch solely inside this
	// disposable test. Commit 98, reject 99, then make the cursor's canonical
	// read unavailable (not different) during the failure path.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil ||
		db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error != nil {
		t.Fatal("disposable partial unknown cursor reset failed")
	}
	fourth.head = &types.Header{Number: big.NewInt(97), ParentHash: third.head.Hash()}
	fifth := fourth
	fifth.head = &types.Header{Number: big.NewInt(98), ParentHash: fourth.head.Hash()}
	fifth.parentHead = fourth.head
	fifth.logs = nil
	sixth := fifth
	sixth.head = &types.Header{Number: big.NewInt(99), ParentHash: fifth.head.Hash()}
	sixth.parentHead = fifth.head
	sixth.logs = []types.Log{{Address: proxy, BlockNumber: 99, BlockHash: sixth.head.Hash(), TxHash: common.HexToHash("0xb4"), Index: 1,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("UnknownProjectEvent(uint256)")), common.BigToHash(big.NewInt(99))}}}
	unknownPartial := v3RangeUnknownOnPartialFailure{v3TwoBlockHistory: v3TwoBlockHistory{creation: &fifth, next: &sixth}, committed: 98, db: db, binding: binding}
	if count, err := v3ScanConfirmedRange(ctx, db, unknownPartial, binding, runtime, 3, 2); err == nil || count != 1 {
		t.Fatalf("unknown partial failure left committed cursor healthy: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 98 || final.State != "reorg_halted" {
		t.Fatalf("unknown partial failure did not halt cursor: %+v err=%v", final, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("unknown partial failure retained internal projection")
	}
	if err := db.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=31337 AND project_id=99 AND block_number=98 AND status='canonical'`).Scan(&nextCheckpoint).Error; err != nil || nextCheckpoint != 1 {
		t.Fatal("unknown RPC incorrectly orphaned committed checkpoint")
	}
	// No new height commits: initial cursor is canonical, the first new
	// height fails, then its previously-published cursor becomes unreadable.
	// Model the latter with an RPC whose cursor read becomes unavailable only
	// after an attempted read of the new height.
	if err := db.Exec(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Error; err != nil ||
		db.Exec(`UPDATE projects SET status='Pending' WHERE project_id=99 AND status='V3ReorgHalted'`).Error != nil {
		t.Fatal("disposable first-failure cursor reset failed")
	}
	seventh := sixth
	seventh.head = &types.Header{Number: big.NewInt(99), ParentHash: fifth.head.Hash()}
	seventh.parentHead = fifth.head
	seventh.logs = nil
	firstFailure := &v3RangeUnknownOnFirstFailure{v3TwoBlockHistory: v3TwoBlockHistory{creation: &fifth, next: &seventh}, cursor: 98}
	if count, err := v3ScanConfirmedRange(ctx, db, firstFailure, binding, runtime, 3, 1); err == nil || count != 0 || !firstFailure.failed {
		t.Fatalf("first height failure retained unknown cursor: count=%d err=%v", count, err)
	}
	if err := db.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=31337 AND proxy_address=? AND project_id=99`, v3Lower(proxy)).Scan(&final).Error; err != nil || final.Block != 98 || final.State != "reorg_halted" {
		t.Fatalf("first failure left prior projection published: %+v err=%v", final, err)
	}
	if _, err := readV3PublishedProjection(ctx, db, binding); err == nil {
		t.Fatal("first failure kept prior projection readable")
	}
}
