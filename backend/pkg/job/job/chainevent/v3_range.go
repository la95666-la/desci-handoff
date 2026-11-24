package chainevent

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// v3ScanConfirmedRange is a bounded driver for an already bootstrapped
// project. The registered Job supplies the separately signed identity and
// authenticated runtime/project binding; this function cannot
// create one from an unverified row, log or caller-provided receipt. Each
// block is independently verified and committed by v3ScanCheckedBlock, which
// fences the cursor under lock. A reorg halts; this does not select an ancestor
// or resume. No V1/V2 event source or signer is reused here.
func v3ScanConfirmedRange(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	minConfirmations, maxBlocks uint64) (uint64, error) {
	return v3ScanConfirmedRangeWithEmptyWindows(ctx, db, rpc, binding, runtime, minConfirmations, maxBlocks, false)
}

// Only the independently admitted multi-project Job enables this optimized
// path. Legacy signed and synthetic scanners retain their exact block-by-block
// journal behavior; the approved caller still checks the original attempt.
func v3ScanApprovedConfirmedRange(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	minConfirmations, maxBlocks uint64) (uint64, error) {
	return v3ScanConfirmedRangeWithEmptyWindows(ctx, db, rpc, binding, runtime, minConfirmations, maxBlocks, true)
}

func v3ScanConfirmedRangeWithEmptyWindows(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	minConfirmations, maxBlocks uint64, emptyWindows bool) (uint64, error) {
	if ctx == nil || db == nil || rpc == nil || minConfirmations == 0 || maxBlocks == 0 || maxBlocks > 512 ||
		binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		binding.ProxyAddress == binding.PaymentToken || binding.ProxyAddress == binding.ProjectToken || binding.PaymentToken == binding.ProjectToken {
		return 0, errors.New("V3 range scanner scope invalid")
	}
	var cursor struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	read := db.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&cursor)
	if read.Error != nil || read.RowsAffected != 1 || cursor.Block == 0 || cursor.Block == math.MaxUint64 || cursor.State != "running" {
		return 0, errors.New("V3 range scanner cursor unavailable or halted")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		// The configured project cannot be monitored on an unknown or foreign
		// chain. Do not leave a previously published projection readable merely
		// because this failure occurred before the canonical header read.
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision); haltErr != nil {
			return 0, errors.New("V3 range scanner chain unavailable; projection revocation unavailable")
		}
		return 0, errors.New("V3 range scanner chain unavailable")
	}
	// A capped invocation still has to notice an already-replaced cursor,
	// including when the head offers no newly confirmed block.
	if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
		// A previously published wallet projection cannot stay internally
		// readable when the next monitoring tick cannot establish even its
		// canonical header (including an RPC outage). A proven mismatch may
		// already have halted it; unknown is never a rollback instruction.
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision); haltErr != nil {
			return 0, errors.New("V3 range scanner cursor unavailable; projection revocation unavailable")
		}
		return 0, err
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil {
		// The last cursor was internally published on an earlier tick, but
		// this tick cannot establish its required confirmation depth. Do not
		// infer a reorg or orphan facts; revoke only the exact prior revision.
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision); haltErr != nil {
			return 0, errors.New("V3 range scanner head unavailable; projection revocation unavailable")
		}
		return 0, errors.New("V3 range scanner head unavailable")
	}
	if head < cursor.Block {
		// A provider reporting a head behind an already-published cursor is
		// inconsistent, not proof of a replaced block. The projection has
		// nevertheless lost its required confirmation depth; revoke only
		// this exact cursor, without orphaning any checkpoint or journal.
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision); haltErr != nil {
			return 0, errors.New("V3 range scanner lagging head; projection revocation unavailable")
		}
		return 0, errors.New("V3 range scanner head behind cursor")
	}
	// Another invocation can advance (or halt) the cursor while the head is
	// being fetched. Never start from the now-stale height even if the newer
	// cursor has an individually canonical checkpoint.
	var current struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	checkSnapshot := func() error {
		read := db.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
			WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`,
			binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&current)
		if read.Error != nil || read.RowsAffected != 1 || current.Block != cursor.Block || current.Hash != cursor.Hash ||
			current.Revision != cursor.Revision || current.State != "running" {
			return errors.New("V3 range scanner cursor changed while reading head")
		}
		return nil
	}
	if err := checkSnapshot(); err != nil {
		return 0, err
	}
	if !idov3whitelist.HasConfirmations(head, cursor.Block, minConfirmations) {
		// Even the already-published cursor has lost the requested depth.
		// This is not reorg proof. Halt and revoke the exact published view,
		// retaining the historical rows for independent reconciliation.
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision); haltErr != nil {
			return 0, errors.New("V3 range scanner cursor depth lost; projection revocation unavailable")
		}
		return 0, errors.New("V3 range scanner cursor confirmations unavailable")
	}
	if head < minConfirmations-1 || cursor.Block >= head-(minConfirmations-1) {
		// A second read cannot certify future finality, but it must not return
		// a successful idle tick if the cursor was replaced or becomes
		// unreadable while reading head. The old published projection must
		// close on unknown RPC as well as a proven mismatch.
		if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
			if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block, cursor.Revision); haltErr != nil {
				return 0, errors.New("V3 range scanner idle cursor revocation unavailable")
			}
			return 0, err
		}
		if err := checkSnapshot(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	end := head - (minConfirmations - 1)
	if end-cursor.Block > maxBlocks {
		end = cursor.Block + maxBlocks // maxBlocks <= 512 and end > cursor.Block
	}
	// Probe small, complete proxy-log windows before the expensive per-block
	// path. Only an independently verified event-free window may use the
	// atomic checkpoint/projection fast path; any event uses the original
	// receipt-aware scanner starting at its first block.
	var scanned uint64
	var lastWindow time.Duration
	for height := cursor.Block + 1; height <= end; {
		// Yield only before starting a new unit, after committed progress.
		// Expiry/failure during a unit retains the original revocation path;
		// final canonicality/depth checks still run after a voluntary yield.
		if emptyWindows && scanned > 0 && yieldV3RangeBetweenWindows(ctx, lastWindow) {
			end = cursor.Block + scanned // final fences certify only the committed prefix
			break
		}
		windowStarted := time.Now()
		if emptyWindows {
			last := end
			if end-height >= v3EmptyRangeLimit {
				last = height + v3EmptyRangeLimit - 1
			}
			var emptyCount uint64
			emptyCount, err = scanV3EmptyRange(ctx, db, rpc, binding, runtime, height, last, minConfirmations)
			if emptyCount > last-height+1 {
				return scanned, errors.New("V3 empty range exceeded its boundary")
			}
			scanned += emptyCount
			if err != nil {
				break
			}
			if emptyCount != 0 {
				lastWindow = time.Since(windowStarted)
				height += emptyCount
				continue
			}
		}
		err = v3ScanCheckedBlock(ctx, db, rpc, binding, runtime, height, minConfirmations)
		if err != nil {
			break
		}
		scanned++
		lastWindow = time.Since(windowStarted)
		height++
	}
	if err != nil {
		if errors.Is(err, errV3EmptyRangePostCommit) {
			return scanned, err // exact batch revision was already handled
		}
		// Earlier heights may already have published a projection before the
		// failing height. Check their final cursor too: a replacement arriving
		// while the later scan failed must revoke it, not just return that error.
		// An RPC failure is unknown, never evidence to guess an ancestor.
		if checkErr := checkV3CursorCanonical(ctx, db, rpc, binding); checkErr != nil {
			// Even when the first new height failed, the earlier published
			// projection cannot remain healthy if its canonical header is now
			// unknown. A revision/height fence protects a concurrent worker.
			if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block+scanned, cursor.Revision+scanned); haltErr != nil {
				return scanned, errors.New("V3 range scanner partial failure; projection revocation unavailable")
			}
			return scanned, errors.New("V3 range scanner partial failure; cursor canonicality unavailable")
		}
		return scanned, err
	}
	// A replacement after the last applyChecked would otherwise let this
	// invocation report success while the newly published projection is stale.
	// This is a transient fence, not a guarantee against later reorgs.
	if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block+scanned, cursor.Revision+scanned); haltErr != nil {
			return scanned, errors.New("V3 range scanner final projection revocation unavailable")
		}
		return scanned, err
	}
	read = db.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&current)
	if read.Error != nil || read.RowsAffected != 1 || current.Block != end || current.Revision < cursor.Revision ||
		current.Revision-cursor.Revision != scanned || current.State != "running" || current.Hash == "" {
		return scanned, errors.New("V3 range scanner final cursor changed")
	}
	checked := current
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(latest, end, minConfirmations) {
		// A lowered/unavailable head alone is not proof of a reorg. It does
		// invalidate the configured confirmation depth of blocks JUST
		// published, so stop serving their projection until separately
		// reconciled. Preserve canonical rows; do not guess an ancestor.
		checkErr := checkV3CursorCanonical(ctx, db, rpc, binding)
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block+scanned, cursor.Revision+scanned); haltErr != nil {
			return scanned, errors.New("V3 range scanner final projection revocation unavailable")
		}
		if checkErr != nil {
			return scanned, errors.New("V3 range scanner final confirmations and canonicality unavailable")
		}
		return scanned, errors.New("V3 range scanner final confirmations unavailable")
	}
	// The provider may replace the just-committed block while answering the
	// final head request. Recheck the exact persisted checkpoint after that
	// request, not only before it. This is still a transient observation: a
	// continuously running monitor must repeat it on subsequent ticks.
	if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
		if haltErr := (v3Journal{db: db}).haltRangeUnknown(ctx, binding, cursor.Block+scanned, cursor.Revision+scanned); haltErr != nil {
			return scanned, errors.New("V3 range scanner final projection revocation unavailable")
		}
		return scanned, err
	}
	// Do not report success if another worker rewrote the just-published
	// cursor while the final head was being fetched. The per-block journal
	// already fences writes; this fences the bounded caller's success result.
	read = db.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&current)
	if read.Error != nil || read.RowsAffected != 1 || current != checked {
		return scanned, errors.New("V3 range scanner final cursor changed")
	}
	return scanned, nil
}

// Keep the bounded traversal independent of RPC/DB so holes and early-stop
// behavior are testable without creating an approved scanner source.
func v3ScanBoundedHeights(start, end uint64, scan func(uint64) error) (uint64, error) {
	if start == 0 || end == math.MaxUint64 || end < start || end-start >= 512 || scan == nil {
		return 0, errors.New("V3 range scan bounds invalid")
	}
	var scanned uint64
	for height := start; height <= end; height++ {
		if err := scan(height); err != nil {
			return scanned, err
		}
		scanned++
	}
	return scanned, nil
}
