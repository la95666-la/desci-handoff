package chainevent

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"math/big"
	"testing"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
)

// Runs inside the existing network-none, MySQL 8.4 fixture only. Schema
// migration is installed separately by the disposable runner, never by Go.
func testV3ProxyStreamSeededMySQL(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	defer func() { rpc.filters = 0; rpc.headerReads = map[uint64]int{} }()
	ctx := context.Background()
	rollback := errors.New("stream rehearsal rollback")
	s := v3StreamScope{Chain: b.ChainID, Proxy: b.ProxyAddress, Runtime: rt, Confirmations: 3}
	e := db.Transaction(func(tx *gorm.DB) error {
		rpc.filters = 0
		if e := bootstrapV3Stream(ctx, tx, rpc, s, "shadow", 90, rpc.blocks[90].head.Hash()); e != nil {
			t.Fatalf("bootstrap: %v", e)
		}
		if e := produceV3Stream(ctx, tx, rpc, s, 3); e != nil {
			t.Fatalf("producer: %v", e)
		}
		c, e := v3StreamCursorRead(tx, s, false)
		if e != nil || c.CursorBlock != 93 || c.Revision != 2 {
			t.Fatalf("producer cursor %+v %v", c, e)
		}
		if _, e := readV3Coverage(tx, s, 90, 93); e != nil {
			t.Fatalf("coverage persist/decode/digest: %v", e)
		}
		meta := v3StreamBlock{Header: (*v3StreamHeader)(types.CopyHeader(rpc.blocks[91].head)), Logs: []types.Log{}, Receipts: map[string]*v3StreamReceipt{}}
		meta.Header.BaseFee = new(big.Int).Lsh(big.NewInt(1), 180)
		raw, _, e := v3StreamPayload(meta)
		if e != nil {
			t.Fatal(e)
		}
		var stored string
		if tx.Raw(`SELECT CAST(? AS CHAR)`, raw).Scan(&stored).Error != nil || stored != raw {
			t.Fatal("exact payload storage precision")
		}
		if _, e := readV3Coverage(tx, s, 90, 94); e == nil {
			t.Fatal("coverage hole silently bridged")
		}
		if e := shadowV3Consumer(ctx, tx, s); e != nil {
			t.Fatalf("shadow parity %v", e)
		}
		if e := switchV3StreamMode(ctx, tx, rpc, s, 1, "stream"); e == nil {
			t.Fatal("stale revision switched writer")
		}
		if e := consumeV3Stream(ctx, tx, rpc, s, b, rt, 3); e == nil {
			t.Fatal("shadow wrote consumer")
		}
		var writer int
		tx.Raw(`SELECT COALESCE(@jc_v3_stream_writer,0)`).Scan(&writer)
		if writer != 0 {
			t.Fatalf("writer flag leaked after rejected shadow consumer: %d", writer)
		}
		testOnlyStreamConsumerMode(t, tx, rpc, s)
		testV3PriorityConsumerMySQL(t, tx, rpc, s, b, rt)
		testV3SharedBatchMySQL(t, tx, rpc, s, b, rt)
		testPeriodicAuditGrantMySQL(t, tx, rpc, s, b, rt)
		testV3SevenConsumerBudgetMySQL(t, tx, rpc, s, b, rt)
		testV3PreparedConsumerParity(t, tx, rpc, s, b, rt, 3)
		// Preparation must not turn a corrupt pre-existing projection into a
		// silently repaired balance. Rejection leaves the original cursor.
		badProjection := errors.New("corrupt prepared projection rehearsal")
		if e := tx.Transaction(func(w *gorm.DB) error {
			if e := w.Exec(`UPDATE ido_v3_project_projection SET claimed_raw='1' WHERE project_id=?`, b.ProjectID).Error; e != nil {
				return e
			}
			if e := consumeV3Stream(ctx, w, rpc, s, b, rt, 3); e == nil {
				t.Fatal("prepared consumer silently repaired corrupt projection")
			}
			shared := context.WithValue(ctx, v3SharedBatchKey{}, true)
			if e := consumeV3Stream(shared, w, rpc, s, b, rt, 3); e == nil {
				t.Fatal("shared quiet path silently repaired corrupt projection")
			}
			c, e := readV3ConsumerCursor(w, b, false)
			if e != nil || c.CursorBlock != 90 {
				t.Fatal("corrupt projection leaked cursor", c, e)
			}
			return badProjection
		}); !errors.Is(e, badProjection) {
			t.Fatal(e)
		}
		// Restore one synthetic scope with 128 complete empty heights, then
		// prove the new ceiling does not multiply SQL by the number of RPCs.
		originalLatest := rpc.latest
		rpc.latest = 230
		for n := uint64(94); n <= 218; n++ {
			copy := *rpc.blocks[93]
			copy.head = &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: rpc.blocks[n-1].head.Hash()}
			rpc.blocks[n] = &copy
		}
		wideRollback := errors.New("128 block prepared rehearsal rollback")
		wideErr := tx.Transaction(func(w *gorm.DB) error {
			for n := uint64(94); n <= 218; n += 32 {
				limit := uint64(32)
				if 218-n+1 < limit {
					limit = 218 - n + 1
				}
				if e := produceV3Stream(ctx, w, rpc, s, limit); e != nil {
					return e
				}
			}
			counter := &v3SQLCounter{Interface: w.Logger}
			if e := consumeV3Stream(ctx, w.Session(&gorm.Session{Logger: counter}), rpc, s, b, rt, 128); e != nil {
				return e
			}
			// Idle consumers verify canonical checkpoint/anchors without
			// reserving a write window; a lagging consumer below gets no writes.
			idle, cancelIdle := context.WithTimeout(ctx, 10*time.Second)
			if e := consumeV3StreamWindows(idle, w, rpc, s, b, rt); e != nil {
				cancelIdle()
				t.Fatal("caught-up consumer unnecessarily reserved 15s", e)
			}
			cancelIdle()
			cursor, e := readV3ConsumerCursor(w, b, false)
			if e != nil || cursor.CursorBlock != 218 || cursor.Revision != 129 || counter.checkpoints != 1 {
				t.Fatalf("wide prepared window %+v SQL=%d checkpoint inserts=%d err=%v", cursor, counter.total, counter.checkpoints, e)
			}
			t.Logf("128-block empty prepared window: SQL=%d, checkpoint INSERT=%d", counter.total, counter.checkpoints)
			return wideRollback
		})
		rpc.latest = originalLatest
		if !errors.Is(wideErr, wideRollback) {
			t.Fatal("wide prepared rehearsal", wideErr)
		}
		short, cancelShort := context.WithTimeout(ctx, 10*time.Second)
		shortErr := consumeV3StreamWindows(short, tx, rpc, s, b, rt)
		cancelShort()
		if !v3Deferred(shortErr) {
			t.Fatal("lagging consumer started short work window", shortErr)
		}
		unchanged, readErr := readV3ConsumerCursor(tx, b, false)
		if readErr != nil || unchanged.CursorBlock != 90 {
			t.Fatal("budget yield advanced durable cursor", unchanged, readErr)
		}
		// Legacy cursor writes are fenced even when its process still runs.
		if e := tx.Exec(`UPDATE ido_v3_scan_cursor SET revision=revision+1 WHERE chain_id=? AND project_id=?`, b.ChainID, b.ProjectID).Error; e == nil {
			var diagnostic struct {
				Mode   string
				Writer int
			}
			tx.Raw(`SELECT mode,COALESCE(@jc_v3_stream_writer,0) AS writer FROM ido_v3_proxy_stream`).Scan(&diagnostic)
			t.Fatalf("legacy writer admitted mode=%s writer=%d", diagnostic.Mode, diagnostic.Writer)
		}
		rpc.filters = 0
		if e := consumeV3Stream(ctx, tx, rpc, s, b, rt, 3); e != nil {
			t.Fatalf("consumer: %v", e)
		}
		// Producer cost is shared, while project failure remains independent.
		second := b
		second.ProjectID++
		if e := v3StreamWriterTx(ctx, tx, func(w *gorm.DB) error {
			if w.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners) SELECT ?,status,funding_mode,contract_version,chain_id,contract_address,onchain_locked,token_address,payment_token_address,owner_address,fee_receiver,description,tags,team_info,roadmap,partners FROM projects WHERE project_id=?`, second.ProjectID, b.ProjectID).Error != nil {
				return errV3Stream
			}
			if w.Exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES (?,?,?)`, second.ProjectID, s.Chain, v3Lower(s.Proxy)).Error != nil {
				return errV3Stream
			}
			if w.Exec(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) VALUES (?,?,?,90,?,1,'running')`, s.Chain, v3Lower(s.Proxy), second.ProjectID, v3LowerHashHeader(rpc.blocks[90].head)).Error != nil {
				return errV3Stream
			}
			return w.Exec(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash) VALUES (?,?,?,90,?)`, s.Chain, v3Lower(s.Proxy), second.ProjectID, v3LowerHashHeader(rpc.blocks[90].head)).Error
		}); e != nil {
			t.Fatal(e)
		}
		if e := consumeV3Stream(ctx, tx, rpc, s, second, rt, 3); e == nil {
			t.Fatal("project without creation evidence consumed")
		}
		if _, e := readV3PublishedProjection(ctx, tx, b); e != nil {
			t.Fatal("bad second consumer revoked first", e)
		}
		// Remove only this synthetic failed cursor so the following all-project
		// halt can prove its full identity/effect revocation independently.
		if tx.Exec(`DELETE FROM ido_v3_scan_cursor WHERE project_id=?`, second.ProjectID).Error != nil {
			t.Fatal("fixture cleanup")
		}
		var p struct{ CursorBlock, Revision uint64 }
		tx.Raw(`SELECT cursor_block,revision FROM ido_v3_scan_cursor WHERE chain_id=? AND project_id=?`, b.ChainID, b.ProjectID).Scan(&p)
		if p.CursorBlock != 93 || p.Revision != 4 {
			t.Fatalf("consumer progress %+v", p)
		}
		if _, e := readV3PublishedProjection(ctx, tx, b); e != nil {
			t.Fatalf("consumer publication: %v", e)
		}
		// Retry/"restart": already consumed coverage is idempotent.
		if e := consumeV3Stream(ctx, tx, rpc, s, b, rt, 3); e != nil {
			t.Fatalf("idle retry %v", e)
		}
		var count int64
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_proxy_consumer WHERE project_id=?`, b.ProjectID).Scan(&count)
		if count != 1 {
			t.Fatal("duplicate consumer")
		}
		if tx.Exec(`UPDATE ido_v3_proxy_coverage SET digest=REPEAT('0',64) WHERE block_number=91`).Error == nil {
			t.Fatal("evidence mutated")
		}
		if e := haltV3Stream(ctx, tx, s); e != nil {
			t.Fatalf("atomic halt: %v", e)
		}
		if _, e := readV3PublishedProjection(ctx, tx, b); e == nil {
			t.Fatal("halt retained authority")
		}
		if e := consumeV3Stream(ctx, tx, rpc, s, b, rt, 3); e == nil {
			t.Fatal("halted stream consumed")
		}
		if e := produceV3Stream(ctx, tx, rpc, s, 3); e == nil {
			t.Fatal("halted stream produced")
		}
		stopped, e := v3StreamCursorRead(tx, s, false)
		if e != nil {
			t.Fatal(e)
		}
		if e = recoverV3StreamEpoch(ctx, tx, rpc, s, stopped.Revision, 90, rpc.blocks[90].head.Hash()); e != nil {
			t.Fatalf("explicit epoch recovery %v", e)
		}
		if e = recoverV3StreamEpoch(ctx, tx, rpc, s, stopped.Revision, 90, rpc.blocks[90].head.Hash()); e == nil {
			t.Fatal("replayed old recovery revision")
		}
		recovered, e := v3StreamCursorRead(tx, s, false)
		if e != nil || recovered.Epoch != 2 || recovered.CursorBlock != 90 {
			t.Fatalf("epoch recovery %+v %v", recovered, e)
		}
		if e = consumeV3Stream(ctx, tx, rpc, s, b, rt, 3); e == nil {
			t.Fatal("epoch reset resumed project authority")
		}
		if e = produceV3Stream(ctx, tx, rpc, s, 3); e != nil {
			t.Fatalf("replacement coverage %v", e)
		}
		var epochs int64
		tx.Raw(`SELECT COUNT(DISTINCT epoch) FROM ido_v3_proxy_coverage WHERE chain_id=? AND proxy_address=?`, s.Chain, v3Lower(s.Proxy)).Scan(&epochs)
		if epochs != 2 {
			t.Fatal("old epoch history lost")
		}
		// Old evidence cannot be consumed through a new epoch's gap.
		if _, e := readV3Coverage(tx, s, 90, 94); e == nil {
			t.Fatal("old epoch filled replacement hole")
		}
		return rollback
	})
	if !errors.Is(e, rollback) {
		t.Fatalf("stream isolation rollback %v", e)
	}
	// All application facts remain at the pre-rehearsal cursor after rollback.
	var block uint64
	db.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND project_id=?`, b.ChainID, b.ProjectID).Scan(&block)
	if block != 90 {
		t.Fatal("rollback leaked cursor")
	}
	if _, e := v3StreamCursorRead(db, s, false); e == nil {
		t.Fatal("rollback leaked stream")
	}
	// Physical connection pinning must reset the marker even when sql.Tx has
	// already auto-rolled back after caller cancellation.
	sqlDB, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	sqlDB.SetMaxOpenConns(1)
	canceled, cancel := context.WithCancel(ctx)
	e = v3StreamWriterTx(canceled, db, func(tx *gorm.DB) error { cancel(); <-canceled.Done(); return canceled.Err() })
	if e == nil {
		t.Fatal("canceled writer committed")
	}
	var marker int
	if db.Raw(`SELECT COALESCE(@jc_v3_stream_writer,0)`).Scan(&marker).Error != nil || marker != 0 {
		t.Fatal("writer session marker leaked after transaction cancellation", marker)
	}
	testV3StreamCancellationEvent(t, db, rpc, b, rt)
	testV3CompleteParityMySQL(t, db)
	testV3TestRetirementBaselineMySQL(t, db, rpc, b, rt)
	// Independent committed scope/connection test runs last: all evidence is
	// retained until the disposable database is destroyed.
	testV3ConsumerDoesNotLockProducerDuringRPC(t, db, rpc, b, rt)
	t.Log("proxy-stream MySQL: coverage/digests, mode fence, stale revision, independent failure, idempotent replay, financial event, crash rollback, atomic halt and retained recovery epochs passed")
}

type streamCancellationRPC struct {
	*v3EmptyWindowRPC
	event   types.Log
	deposit types.Log
	policy  types.Log
	receipt *types.Receipt
}

func (r streamCancellationRPC) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	if q.FromBlock.Uint64() <= 92 && q.ToBlock.Uint64() >= 92 {
		return []types.Log{r.deposit, r.event, r.policy}, nil
	}
	return []types.Log{}, nil
}
func (r streamCancellationRPC) TransactionReceipt(_ context.Context, h common.Hash) (*types.Receipt, error) {
	if h != r.event.TxHash {
		return nil, errV3Stream
	}
	return v3CloneReceipt(r.receipt), nil
}

func testV3StreamCancellationEvent(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	ctx := context.Background()
	rollback := errors.New("event rehearsal rollback")
	s := v3StreamScope{Chain: b.ChainID, Proxy: b.ProxyAddress, Runtime: rt, Confirmations: 3}
	saved := rpc.blocks[92].tuple
	oldPolicy := viper.Get("idoV3PolicyJournal.enabled")
	viper.Set("idoV3PolicyJournal.enabled", true)
	defer func() {
		viper.Set("idoV3PolicyJournal.enabled", oldPolicy)
		rpc.blocks[92].tuple = saved
		rpc.blocks[93].tuple = v3PendingTuple(b.ProjectID, b.ProjectToken, b.PaymentToken)
		rpc.blocks[92].policy = nil
		rpc.blocks[93].policy = nil
	}()
	for _, n := range []uint64{92, 93} {
		tuple := v3PendingTuple(b.ProjectID, b.ProjectToken, b.PaymentToken)
		tuple[32+8*32+31] = 4
		rpc.blocks[n].tuple = tuple
	}
	l := types.Log{Address: b.ProxyAddress, BlockNumber: 92, BlockHash: rpc.blocks[92].head.Hash(), TxHash: common.HexToHash("0xcc"), Index: 1, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectCancelled(uint256,address,uint256)")), common.BigToHash(new(big.Int).SetUint64(b.ProjectID)), common.BytesToHash(b.PaymentToken.Bytes())}, Data: make([]byte, 32)}
	deposit := l
	deposit.Index = 0
	deposit.Topics = []common.Hash{crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")), common.BigToHash(new(big.Int).SetUint64(b.ProjectID)), common.BytesToHash(b.ProjectToken.Bytes())}
	deposit.Data = big.NewInt(1000).FillBytes(make([]byte, 32))
	root := common.HexToHash("0xabcdef")
	policy := l
	policy.Index = 2
	policy.Topics = []common.Hash{v3PolicyUpdateTopic, common.BigToHash(new(big.Int).SetUint64(b.ProjectID)), common.BigToHash(big.NewInt(1))}
	policy.Data = append(big.NewInt(1).FillBytes(make([]byte, 32)), root.Bytes()...)
	for _, n := range []uint64{92, 93} {
		rpc.blocks[n].policy = append(append(big.NewInt(1).FillBytes(make([]byte, 32)), root.Bytes()...), big.NewInt(1).FillBytes(make([]byte, 32))...)
	}
	r := streamCancellationRPC{v3EmptyWindowRPC: rpc, event: l, deposit: deposit, policy: policy, receipt: &types.Receipt{Status: 1, TxHash: l.TxHash, BlockHash: l.BlockHash, BlockNumber: big.NewInt(92), Logs: []*types.Log{&deposit, &l, &policy}}}
	e := db.Transaction(func(tx *gorm.DB) error {
		if e := bootstrapV3Stream(ctx, tx, r, s, "shadow", 90, rpc.blocks[90].head.Hash()); e != nil {
			t.Fatal(e)
		}
		if e := produceV3Stream(ctx, tx, r, s, 3); e != nil {
			t.Fatalf("event produce %v", e)
		}
		testOnlyStreamConsumerMode(t, tx, r, s)
		testV3PreparedConsumerParity(t, tx, r, s, b, rt, 3)
		// Inject a failure after event application but before outer commit; neither
		// journal nor business state may escape the rolled-back consumer unit.
		failure := errors.New("injected crash")
		e := tx.Transaction(func(inner *gorm.DB) error {
			if e := consumeV3Stream(ctx, inner, r, s, b, rt, 3); e != nil {
				t.Fatalf("event consume %v", e)
			}
			return failure
		})
		if !errors.Is(e, failure) {
			t.Fatal(e)
		}
		var status string
		tx.Raw(`SELECT status FROM projects WHERE project_id=?`, b.ProjectID).Scan(&status)
		if status != "Pending" {
			t.Fatal("failed transaction leaked status", status)
		}
		if e := consumeV3Stream(ctx, tx, r, s, b, rt, 3); e != nil {
			t.Fatal(e)
		}
		tx.Raw(`SELECT status FROM projects WHERE project_id=?`, b.ProjectID).Scan(&status)
		if status != "Canceled" {
			t.Fatal("event not applied", status)
		}
		if e := consumeV3Stream(ctx, tx, r, s, b, rt, 3); e != nil {
			t.Fatal(e)
		}
		var count int64
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE project_id=? AND event_kind='ProjectCancelled'`, b.ProjectID).Scan(&count)
		if count != 1 {
			t.Fatal("event applied twice")
		}
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE project_id=? AND status='canonical'`, b.ProjectID).Scan(&count)
		if count == 0 {
			t.Fatal("stream lost deposit financial effect")
		}
		if e := haltV3Stream(ctx, tx, s); e != nil {
			t.Fatal("post-event revocation", e)
		}
		if _, e := readV3PublishedProjection(ctx, tx, b); e == nil {
			t.Fatal("post-event stale authority")
		}
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_financial_effect WHERE project_id=? AND status='canonical'`, b.ProjectID).Scan(&count)
		if count != 0 {
			t.Fatal("halt retained financial authority")
		}
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE project_id=? AND event_kind='ProjectCancelled'`, b.ProjectID).Scan(&count)
		if count != 1 {
			t.Fatal("halt erased event history")
		}
		return rollback
	})
	if !errors.Is(e, rollback) {
		t.Fatal(e)
	}
}

// The historical fixture starts AT creation with an empty anchor marker. It
// must NOT pass the new full-history production switch. Seed mode only inside
// its rollback transaction to test lower-level consumer/fence failure paths.
// Full switch success is exercised by a separate pre-creation fixture.
func testOnlyStreamConsumerMode(t *testing.T, tx *gorm.DB, rpc v3ScannerRPC, s v3StreamScope) {
	t.Helper()
	if e := switchV3StreamMode(context.Background(), tx, rpc, s, 2, "stream"); e == nil {
		t.Fatal("incomplete creation coverage switched production writer")
	}
	if tx.Exec(`UPDATE ido_v3_proxy_stream SET mode='stream',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=? AND mode='shadow' AND revision=2`, s.args()...).Error != nil {
		t.Fatal("rollback-only consumer fixture mode setup")
	}
}
