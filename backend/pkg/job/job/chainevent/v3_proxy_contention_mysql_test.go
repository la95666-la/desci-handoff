package chainevent

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
	"math/big"
	"testing"
	"time"
)

// Called only by the network-none MySQL 8.4 fixture, after rollback-only
// rehearsals. This one deliberately commits so a second connection sees locks.
func testV3ConsumerDoesNotLockProducerDuringRPC(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	sqlDB, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	sqlDB.SetMaxOpenConns(4)
	defer sqlDB.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, v3SharedBatchKey{}, true)
	old := b
	b.ProjectID += 100
	b.ProxyAddress = common.HexToAddress("0x3939393939393939393939393939393939393939")
	copyRPC := &v3EmptyWindowRPC{blocks: map[uint64]*v3PinnedScanStub{}, latest: 110, headerReads: map[uint64]int{}}
	for n, block := range rpc.blocks {
		copy := *block
		copy.proxy = b.ProxyAddress
		copy.project = b.ProjectID
		copy.tuple = v3PendingTuple(b.ProjectID, b.ProjectToken, b.PaymentToken)
		copyRPC.blocks[n] = &copy
	}
	rpc = copyRPC
	s := v3StreamScope{Chain: b.ChainID, Proxy: b.ProxyAddress, Runtime: rt, Confirmations: 3}
	exec := func(q string, args ...any) {
		t.Helper()
		if e := db.Exec(q, args...).Error; e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block) SELECT chain_id,?,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block FROM ido_v3_runtime WHERE chain_id=? AND proxy_address=?`, v3Lower(s.Proxy), old.ChainID, v3Lower(old.ProxyAddress))
	exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners) SELECT ?,status,funding_mode,contract_version,chain_id,?,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners FROM projects WHERE project_id=?`, b.ProjectID, v3Lower(s.Proxy), old.ProjectID)
	exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(?,?,?)`, b.ProjectID, b.ChainID, v3Lower(s.Proxy))
	var payload string
	if e := db.Raw(`SELECT payload FROM ido_v3_event_journal WHERE project_id=? AND event_kind='created'`, old.ProjectID).Scan(&payload).Error; e != nil {
		t.Fatal(e)
	}
	var created commonchain.V3ReplayEvent
	if json.Unmarshal([]byte(payload), &created) != nil || created.Created == nil {
		t.Fatal("missing creation")
	}
	created.Created.ProjectID = b.ProjectID
	created.Created.Proxy = b.ProxyAddress
	raw, e := json.Marshal(created)
	if e != nil {
		t.Fatal(e)
	}
	exec(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) VALUES(?,?,?,90,?,1,'running')`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3LowerHashHeader(rpc.blocks[90].head))
	exec(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash) VALUES(?,?,?,90,?)`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3LowerHashHeader(rpc.blocks[90].head))
	exec(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,tx_hash,log_index,event_kind,payload) VALUES(?,?,?,90,?,?,1,'created',?)`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3LowerHashHeader(rpc.blocks[90].head), created.Created.TxHash.Hex(), string(raw))
	if e := rebuildV3ProjectTx(db, b, 90, rpc.blocks[90].head.Hash(), 1); e != nil {
		t.Fatal(e)
	}
	for n := uint64(94); n <= 96; n++ {
		copy := *rpc.blocks[93]
		copy.head = &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: rpc.blocks[n-1].head.Hash()}
		rpc.blocks[n] = &copy
	}
	if e := bootstrapV3Stream(ctx, db, rpc, s, "shadow", 90, rpc.blocks[90].head.Hash()); e != nil {
		t.Fatal(e)
	}
	if e := produceV3Stream(ctx, db, rpc, s, 3); e != nil {
		t.Fatal(e)
	}
	// Synthetic fixture creation predates covered logs; mode setup is local
	// fixture-only, never an application switch or production bypass.
	if e := db.Exec(`UPDATE ido_v3_proxy_stream SET mode='stream',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Error; e != nil {
		t.Fatal(e)
	}
	producerFailure := &v3TransientConsumerRPC{v3ScannerRPC: rpc, db: db, project: b.ProjectID}
	if n, e := produceV3StreamBudget(ctx, db, producerFailure, s, &v3ProducerBudget{}); !v3Deferred(e) || n != 0 {
		t.Fatalf("producer timeout n=%d err=%v", n, e)
	}
	unchanged, e := v3StreamCursorRead(db, s, false)
	if e != nil || unchanged.CursorBlock != 93 || unchanged.State != "running" {
		t.Fatal("producer timeout mutated prefix", unchanged, e)
	}
	probe := &v3UnlockedConsumerProbe{v3ScannerRPC: rpc, db: db, s: s, t: t}
	if e := consumeV3Stream(ctx, db, probe, s, b, rt, 3); e != nil {
		t.Fatal("consumer while producer advances", e)
	}
	c, e := v3StreamCursorRead(db, s, false)
	if e != nil || c.CursorBlock != 94 || probe.calls < 4 {
		t.Fatalf("producer=%+v RPC probes=%d err=%v", c, probe.calls, e)
	}
	var block uint64
	db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=?`, b.ProjectID).Scan(&block)
	if block != 93 {
		t.Fatalf("consumer did not publish prepared prefix: %d", block)
	}
	t.Logf("producer committed 93->94 during consumer preparation; %d live RPC probes acquired stream FOR UPDATE NOWAIT", probe.calls)
	// A source timeout before publication preserves every durable cursor and
	// business fact. A timeout after publication preserves its fully verified
	// prefix but reports incomplete (never publishes whole-tick success).
	transient := &v3TransientConsumerRPC{v3ScannerRPC: rpc, db: db, project: b.ProjectID}
	if e := consumeV3Stream(ctx, db, transient, s, b, rt, 1); !v3Deferred(e) {
		t.Fatalf("precommit timeout not deferred: %v", e)
	}
	current, e := readV3ConsumerCursor(db, b, false)
	if e != nil || current.CursorBlock != 93 || current.State != "running" {
		t.Fatal("timeout changed durable cursor", current, e)
	}
	transient.postcommit = true
	if e := consumeV3Stream(ctx, db, transient, s, b, rt, 1); !v3Deferred(e) {
		t.Fatalf("postcommit timeout not deferred: %v", e)
	}
	current, e = readV3ConsumerCursor(db, b, false)
	if e != nil || current.CursorBlock != 94 || current.State != "running" {
		t.Fatal("verified publication revoked on timeout", current, e)
	}
	if _, e := readV3PublishedProjection(ctx, db, b); e != nil {
		t.Fatal("historical projection destroyed", e)
	}
	// Reset only this disposable fixture's just-consumed block to exercise
	// the following independent CAS/depth regressions at their original tip.
	if e := v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		if e := tx.Exec(`DELETE FROM ido_v3_block_checkpoint WHERE project_id=? AND block_number=94`, b.ProjectID).Error; e != nil {
			return e
		}
		if e := tx.Exec(`UPDATE ido_v3_scan_cursor SET cursor_block=93,cursor_hash=?,revision=revision-1 WHERE project_id=?`, v3LowerHashHeader(rpc.blocks[93].head), b.ProjectID).Error; e != nil {
			return e
		}
		return rebuildV3ProjectTx(tx, b, 93, rpc.blocks[93].head.Hash(), current.Revision-1)
	}); e != nil {
		t.Fatal(e)
	}
	// Committed root DB cache: second read of the exact epoch/range is local,
	// independent consumers receive deep copies, epochs never share evidence.
	cacheCtx := newV3CoverageReads(ctx, db)
	counter := &v3SQLCounter{Interface: db.Logger}
	observedDB := db.Session(&gorm.Session{Logger: counter})
	cacheCtx = newV3CoverageReads(ctx, observedDB)
	one, e := readV3ConsumerCoverage(cacheCtx, observedDB, s, 1, 90, 93)
	if e != nil {
		t.Fatal(e)
	}
	one[90].header().Extra = []byte("caller mutation")
	two, e := readV3ConsumerCoverage(cacheCtx, observedDB, s, 1, 90, 93)
	if e != nil || len(two[90].header().Extra) != 0 || counter.coverage != 1 {
		t.Fatal("coverage not shared safely", e, counter.coverage)
	}
	// GORM Session wrappers used by production tracing share the same pool;
	// pointer inequality must not silently disable public evidence sharing.
	three, e := readV3ConsumerCoverage(cacheCtx, observedDB.Session(&gorm.Session{}), s, 1, 90, 93)
	if e != nil || len(three) != 4 || counter.coverage != 1 {
		t.Fatal("tracing session disabled shared coverage", e, counter.coverage)
	}
	if _, e := readV3ConsumerCoverage(cacheCtx, observedDB, s, 2, 90, 93); e == nil {
		t.Fatal("wrong epoch accepted")
	}
	// Different project cursor offsets share one aligned public page.
	pageCounter := &v3SQLCounter{Interface: db.Logger}
	pageDB := db.Session(&gorm.Session{Logger: pageCounter})
	pageCtx := newV3CoverageReads(ctx, pageDB)
	public, e := v3StreamCursorRead(db, s, false)
	if e != nil {
		t.Fatal(e)
	}
	onePage, e := readV3SharedCoverage(pageCtx, pageDB, s, public, 90, 93)
	if e != nil {
		t.Fatal(e)
	}
	onePage[91].header().Extra = []byte("consumer mutation")
	otherPage, e := readV3SharedCoverage(pageCtx, pageDB.Session(&gorm.Session{}), s, public, 91, 93)
	if e != nil || len(otherPage[91].header().Extra) != 0 || pageCounter.coverage != 1 {
		t.Fatal("offset pages not shared or alias leaked", e, pageCounter.coverage)
	}
	// Prepare once, then mutate the project revision on another connection.
	// The stale plan must not commit even though every RPC response is valid.
	cas := &v3ConsumerCASProbe{v3ScannerRPC: rpc, db: db, b: b}
	if e := consumeV3Stream(ctx, db, cas, s, b, rt, 1); e == nil {
		t.Fatal("changed project CAS accepted")
	}
	db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=?`, b.ProjectID).Scan(&block)
	if block != 93 {
		t.Fatal("CAS rejection advanced project", block)
	}
	// Restore only this injected test revision; no production repair path.
	if e := v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		return tx.Exec(`UPDATE ido_v3_scan_cursor SET revision=revision-1 WHERE project_id=?`, b.ProjectID).Error
	}); e != nil {
		t.Fatal(e)
	}
	// Replacing the epoch between evidence reads and publication must not
	// pass as ordinary producer advancement, even at the same height/hash.
	changed := &v3ConsumerEpochProbe{v3ScannerRPC: rpc, db: db, s: s}
	if e := consumeV3Stream(ctx, db, changed, s, b, rt, 1); e == nil {
		t.Fatal("epoch changed during preparation was published")
	}
	db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=?`, b.ProjectID).Scan(&block)
	if block != 93 {
		t.Fatal("rejected preparation leaked cursor", block)
	}
	// Restore only the synthetic epoch transition, then inject lost depth
	// AFTER publication; the exact just-published revision must be revoked.
	exec(`UPDATE ido_v3_proxy_stream SET epoch=1,revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...)
	depth := &v3ConsumerDepthProbe{v3ScannerRPC: rpc, db: db, project: b.ProjectID}
	if e := consumeV3Stream(ctx, db, depth, s, b, rt, 1); e == nil {
		t.Fatal("post-publication lost depth accepted")
	}
	var state string
	db.Raw(`SELECT state FROM ido_v3_scan_cursor WHERE project_id=?`, b.ProjectID).Scan(&state)
	var projections int64
	db.Raw(`SELECT COUNT(*) FROM ido_v3_project_projection WHERE project_id=?`, b.ProjectID).Scan(&projections)
	if state != "reorg_halted" || projections != 0 {
		t.Fatalf("post-publication revocation state=%s projections=%d", state, projections)
	}
	// Two real connections race global stop vs project revoke. Both use the
	// same stream->project order, including the trigger's stream read.
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- haltV3Stream(ctx, db, s) }()
	go func() {
		<-start
		results <- v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
			if _, e := v3StreamCursorRead(tx, s, true); e != nil {
				return e
			}
			var row struct{ CursorBlock, Revision uint64 }
			if e := tx.Raw(`SELECT cursor_block,revision FROM ido_v3_scan_cursor WHERE project_id=? FOR UPDATE`, b.ProjectID).Scan(&row).Error; e != nil {
				return e
			}
			return (v3Journal{db: tx}).haltRangeUnknown(ctx, b, row.CursorBlock, row.Revision-1)
		})
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if e := <-results; e != nil {
			t.Fatalf("ordered concurrent revocation failed: %v", e)
		}
	}
	// Evidence in this independent synthetic scope is retained until the
	// network-none disposable database is destroyed.
}

type v3TransientConsumerRPC struct {
	v3ScannerRPC
	db         *gorm.DB
	project    uint64
	postcommit bool
}

func (r *v3TransientConsumerRPC) ChainID(ctx context.Context) (*big.Int, error) {
	if !r.postcommit {
		return nil, context.DeadlineExceeded
	}
	var n uint64
	if e := r.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=?`, r.project).Scan(&n).Error; e != nil {
		return nil, e
	}
	if n >= 94 {
		return nil, context.DeadlineExceeded
	}
	return r.v3ScannerRPC.ChainID(ctx)
}

type v3ConsumerCASProbe struct {
	v3ScannerRPC
	db      *gorm.DB
	b       idov3whitelist.ExpectedProjectBinding
	changed bool
}

func (r *v3ConsumerCASProbe) ChainID(ctx context.Context) (*big.Int, error) {
	if !r.changed {
		r.changed = true
		if e := v3StreamWriterTx(ctx, r.db, func(tx *gorm.DB) error {
			return tx.Exec(`UPDATE ido_v3_scan_cursor SET revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=?`, r.b.ChainID, v3Lower(r.b.ProxyAddress), r.b.ProjectID).Error
		}); e != nil {
			return nil, e
		}
	}
	return r.v3ScannerRPC.ChainID(ctx)
}

type v3ConsumerEpochProbe struct {
	v3ScannerRPC
	db      *gorm.DB
	s       v3StreamScope
	changed bool
}

func (r *v3ConsumerEpochProbe) ChainID(ctx context.Context) (*big.Int, error) {
	if !r.changed {
		r.changed = true
		if e := r.db.WithContext(ctx).Exec(`UPDATE ido_v3_proxy_stream SET epoch=epoch+1,revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=?`, r.s.args()...).Error; e != nil {
			return nil, e
		}
	}
	return r.v3ScannerRPC.ChainID(ctx)
}

type v3ConsumerDepthProbe struct {
	v3ScannerRPC
	db      *gorm.DB
	project uint64
}

func (r *v3ConsumerDepthProbe) BlockNumber(ctx context.Context) (uint64, error) {
	var block uint64
	if e := r.db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=?`, r.project).Scan(&block).Error; e != nil {
		return 0, e
	}
	if block >= 94 {
		return block, nil
	}
	return r.v3ScannerRPC.BlockNumber(ctx)
}

type v3UnlockedConsumerProbe struct {
	v3ScannerRPC
	db    *gorm.DB
	s     v3StreamScope
	t     *testing.T
	calls int
}

func (r *v3UnlockedConsumerProbe) ChainID(ctx context.Context) (*big.Int, error) {
	r.calls++
	if e := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mode string
		return tx.Raw(`SELECT mode FROM ido_v3_proxy_stream WHERE chain_id=? AND proxy_address=? AND contract_version=? FOR UPDATE NOWAIT`, r.s.args()...).Scan(&mode).Error
	}); e != nil {
		r.t.Errorf("public row locked during live RPC #%d: %v", r.calls, e)
		return nil, e
	}
	if r.calls == 1 {
		if e := produceV3Stream(ctx, r.db, r.v3ScannerRPC, r.s, 1); e != nil {
			return nil, e
		}
	}
	return r.v3ScannerRPC.ChainID(ctx)
}
