package chainevent

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
)

func testV3SharedBatchMySQL(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	rollback := errors.New("shared 512 batch rollback")
	originalLatest := rpc.latest
	defer func() { rpc.latest = originalLatest }()
	rpc.latest = 620
	for n := uint64(94); n <= 602; n++ {
		copy := *rpc.blocks[93]
		copy.head = &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: rpc.blocks[n-1].head.Hash()}
		rpc.blocks[n] = &copy
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		for n := uint64(94); n <= 602; n += 32 {
			limit := uint64(32)
			if 602-n+1 < limit {
				limit = 602 - n + 1
			}
			if err := produceV3Stream(ctx, tx, rpc, s, limit); err != nil {
				return err
			}
		}
		legacyCounter := &v3SQLCounter{Interface: tx.Logger}
		var reference v3ConsumerFacts
		if err := tx.Transaction(func(w *gorm.DB) error {
			for i := 0; i < 4; i++ {
				if err := consumeV3Stream(ctx, w.Session(&gorm.Session{Logger: legacyCounter}), rpc, s, b, rt, 128); err != nil {
					return err
				}
			}
			reference = readV3ConsumerFacts(t, w, b)
			return rollback
		}); !errors.Is(err, rollback) {
			return err
		}
		counter := &v3SQLCounter{Interface: tx.Logger}
		countRPC := &deploymentCountingRPC{v3ScannerRPC: rpc}
		sharedCtx := context.WithValue(ctx, v3SharedBatchKey{}, true)
		if err := consumeV3StreamWindows(sharedCtx, tx.Session(&gorm.Session{Logger: counter}), countRPC, s, b, rt); err != nil {
			return err
		}
		actual := readV3ConsumerFacts(t, tx, b)
		if !reflect.DeepEqual(reference, actual) {
			t.Fatal("512 shared batch changed replay financial/cursor facts")
		}
		if actual.Cursor.CursorBlock != 602 || actual.Cursor.Revision != 513 || counter.checkpoints != 1 {
			t.Fatal(actual.Cursor, counter.checkpoints)
		}
		if countRPC.calls != 3 {
			t.Fatal("quiet batch repeated state calls", countRPC.calls)
		}
		// A caught-up check must not rewrite consumer timestamps/business rows.
		idleCounter := &v3SQLCounter{Interface: tx.Logger}
		var before, after string
		q := `SELECT CAST(last_success_at AS CHAR) FROM ido_v3_proxy_consumer WHERE chain_id=? AND proxy_address=? AND project_id=?`
		if err := tx.Raw(q, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID).Scan(&before).Error; err != nil {
			return err
		}
		if err := consumeV3StreamWindows(sharedCtx, tx.Session(&gorm.Session{Logger: idleCounter}), rpc, s, b, rt); err != nil {
			return err
		}
		if err := tx.Raw(q, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID).Scan(&after).Error; err != nil {
			return err
		}
		if before != after || idleCounter.checkpoints != 0 {
			t.Fatal("idle poll manufactured consumer progress", before, after)
		}
		t.Logf("shared quiet 512 blocks vs 4x128: identical facts; checkpoint INSERT %d->%d; state calls=%d SQL=%d->%d", legacyCounter.checkpoints, counter.checkpoints, countRPC.calls, legacyCounter.total, counter.total)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	testV3CatchupBatchMySQL(t, db, rpc, s, b, rt)
}

func testV3CatchupBatchMySQL(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	rollback := errors.New("catchup parity rollback")
	rpc.latest = 1700
	for n := uint64(94); n <= 1626; n++ {
		copy := *rpc.blocks[93]
		copy.head = &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: rpc.blocks[n-1].head.Hash()}
		rpc.blocks[n] = &copy
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		for n := uint64(94); n <= 1626; n += 32 {
			limit := uint64(32)
			if 1626-n+1 < limit {
				limit = 1626 - n + 1
			}
			if err := produceV3Stream(ctx, tx, rpc, s, limit); err != nil {
				return err
			}
		}
		var reference v3ConsumerFacts
		if err := tx.Transaction(func(w *gorm.DB) error {
			for i := 0; i < 12; i++ {
				if err := consumeV3Stream(ctx, w, rpc, s, b, rt, 128); err != nil {
					return err
				}
			}
			reference = readV3ConsumerFacts(t, w, b)
			return rollback
		}); !errors.Is(err, rollback) {
			return err
		}
		catchup := context.WithValue(ctx, v3SharedBatchKey{}, true)
		catchup = context.WithValue(catchup, v3CatchupBatchKey{}, true)
		if err := consumeV3Stream(catchup, tx, rpc, s, b, rt, v3CatchupConsumerBlocks); err != nil {
			return err
		}
		actual := readV3ConsumerFacts(t, tx, b)
		if !reflect.DeepEqual(reference, actual) || actual.Cursor.CursorBlock != 1626 {
			t.Fatal("1536 catchup skipped evidence or changed facts", actual.Cursor)
		}
		t.Log("catchup1536 vs12x128 identical canonical cursor/journal/effects/projection/outbox")
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
