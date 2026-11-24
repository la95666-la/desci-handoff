package chainevent

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// One state per registered Job; protected by the Job writer mutex, never used
// as authorization. Restart conservatively returns to four-block units.
type v3ProducerBudget struct{ batch uint64 }

var errV3UnitDeadline = errors.New("V3 producer unit deadline; durable anchor revalidated")

func (b *v3ProducerBudget) size() uint64 {
	if b.batch == 0 {
		b.batch = 4
	}
	return b.batch
}
func (b *v3ProducerBudget) success(elapsed time.Duration) {
	// Escape tiny batches despite the fixed identity/anchor cost. At 16+
	// blocks keep conservative growth, avoiding 16->32->timeout oscillation
	// on providers where a healthy 16-block unit takes roughly five seconds.
	growthWindow := 3 * time.Second
	if b.batch < 16 {
		growthWindow = 6 * time.Second
	}
	if elapsed < growthWindow && b.batch < 32 {
		b.batch *= 2
	}
	// A completed 5s/16-block batch is productive, not a failure. Wall time
	// includes DB contention and provider latency; halve only on an actual
	// unit deadline after fresh anchor validation, never on normal success.
}
func (b *v3ProducerBudget) timeout() {
	if b.batch > 1 {
		b.batch /= 2
	}
}

// Each callback commits ONE fully verified contiguous unit atomically. A
// failed unit cannot erase earlier commits. Only local deadline exhaustion
// may shrink/retry, and only after independently checking the persisted tip.
// Structural/provider disagreement is never retried as a smaller "success".
func runV3ProducerBudget(ctx context.Context, b *v3ProducerBudget,
	step func(context.Context, uint64) (uint64, error), anchor func(context.Context) error) (uint64, error) {
	return runV3ProducerBudgetUnit(ctx, b, 8*time.Second, step, anchor)
}

func runV3ProducerBudgetUnit(ctx context.Context, b *v3ProducerBudget, unitBudget time.Duration,
	step func(context.Context, uint64) (uint64, error), anchor func(context.Context) error) (uint64, error) {
	var progressed uint64
	for units := 0; units < 64 && progressed < 512; units++ {
		if e := ctx.Err(); e != nil {
			return progressed, e
		}
		// Do not start an 8s unit in the final 3-7s of a turn: parent-window
		// exhaustion is not evidence that the batch/provider is too slow.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < unitBudget+time.Second {
			return progressed, nil
		}
		limit := b.size()
		if limit > 512-progressed {
			limit = 512 - progressed
		}
		unit, cancel := context.WithTimeout(ctx, unitBudget)
		started := time.Now()
		n, e := step(unit, limit)
		expired := errors.Is(unit.Err(), context.DeadlineExceeded)
		cancel()
		if e != nil {
			if expired && ctx.Err() == nil && !errors.Is(e, errV3StreamConflict) {
				// Revalidation has its own parent budget, not the expired unit context.
				if check := anchor(ctx); check != nil {
					return progressed, check
				}
				old := b.size()
				b.timeout()
				if old == 1 {
					return progressed, errV3UnitDeadline
				}
				continue
			}
			return progressed, e
		}
		if n > limit {
			return progressed, errV3Stream
		}
		progressed += n
		b.success(time.Since(started))
		if n == 0 {
			return progressed, nil
		}
	}
	return progressed, nil
}

func produceV3StreamBudget(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b *v3ProducerBudget) (uint64, error) {
	dbFailures := &v3DBFailures{Interface: db.Logger}
	db = db.Session(&gorm.Session{Logger: dbFailures})
	n, e := runV3ProducerBudget(ctx, b, func(unit context.Context, limit uint64) (uint64, error) {
		before, e := v3StreamCursorRead(db.WithContext(unit), s, false)
		if e != nil {
			return 0, e
		}
		tracked := &v3TrackedRPC{v3ScannerRPC: rpc}
		if e = produceV3Stream(unit, db, tracked, s, limit); e != nil {
			return 0, tracked.finish(unit, "producer_unit", e)
		}
		after, e := v3StreamCursorRead(db.WithContext(unit), s, false)
		if e != nil || after.CursorBlock < before.CursorBlock {
			return 0, errV3Stream
		}
		return after.CursorBlock - before.CursorBlock, nil
	}, func(c context.Context) (resultErr error) {
		tracked := &v3TrackedRPC{v3ScannerRPC: rpc}
		defer func() { resultErr = tracked.finish(c, "producer_revalidation", resultErr) }()
		current, e := v3StreamCursorRead(db.WithContext(c), s, false)
		if e != nil || current.State != "running" {
			return errV3Stream
		}
		if e = v3StreamAnchor(c, tracked, s, current.CursorBlock, common.HexToHash(current.CursorHash)); e != nil {
			return e
		}
		h, e := tracked.HeaderByNumber(c, new(big.Int).SetUint64(current.CursorBlock))
		if e != nil || h == nil {
			return errV3Stream
		}
		return v3StreamRuntime(c, tracked, s, h)
	})
	return n, dbFailures.finish(e)
}
