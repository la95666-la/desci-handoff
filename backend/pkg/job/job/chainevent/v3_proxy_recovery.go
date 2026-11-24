package chainevent

import (
	"context"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// Explicit reviewed recovery, not an automatic rewind. Old coverage remains in
// its immutable epoch. Caller supplies an exact independently selected ancestor
// and expected halted revision. Project rollback/re-admission remains separate;
// this function never resumes financial projections or releases a nonce.
func recoverV3StreamEpoch(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, expected, ancestor uint64, hash common.Hash) error {
	if !s.valid() || expected == 0 || ancestor == 0 || hash == (common.Hash{}) {
		return errV3Stream
	}
	if e := v3StreamAnchor(ctx, rpc, s, ancestor, hash); e != nil {
		return e
	}
	h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(ancestor))
	if e != nil || h == nil || h.Hash() != hash {
		return errV3Stream
	}
	if e = v3StreamRuntime(ctx, rpc, s, h); e != nil {
		return e
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old, e := v3StreamCursorRead(tx, s, true)
		if e != nil || old.State != "halted" || old.Revision != expected || old.Epoch == math.MaxUint64 || ancestor < old.AnchorBlock || ancestor > old.CursorBlock {
			return errV3Stream
		}
		recorded, e := readV3Coverage(tx, s, ancestor, ancestor)
		if e != nil || recorded[ancestor].header().Hash() != hash {
			return errV3Stream
		}
		// Every live-mode project must remain halted through the new backfill.
		if old.Mode == "stream" {
			var live int64
			if tx.Raw(`SELECT COUNT(*) FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state<>'reorg_halted'`, s.args()...).Scan(&live).Error != nil || live != 0 {
				return errV3Stream
			}
		}
		if e = v3StreamAnchor(ctx, rpc, s, ancestor, hash); e != nil {
			return e
		}
		r := tx.Exec(`UPDATE ido_v3_proxy_stream SET state='running',anchor_block=?,cursor_block=?,cursor_hash=?,epoch=epoch+1,revision=revision+1,observed_head=0,last_success_at=NULL WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state='halted' AND revision=?`, append([]any{ancestor, ancestor, strings.ToLower(hash.Hex())}, append(s.args(), expected)...)...)
		if r.Error != nil || r.RowsAffected != 1 {
			return errV3Stream
		}
		payload, digest, e := v3StreamPayload(recorded[ancestor])
		if e != nil {
			return e
		}
		return insertV3Coverage(tx, s, h, payload, digest)
	})
}

// A recovered epoch must cover the project's full previously consumed prefix
// before existing original-create/reorg admission may resume it. No per-project
// loop may reconstruct omitted public coverage from a cursor or lone event.
func requireV3RecoveryCoverage(ctx context.Context, db *gorm.DB, s v3StreamScope, id uint64) error {
	c, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil || c.State != "running" || c.Mode != "stream" {
		return errV3Stream
	}
	var p struct{ CursorBlock uint64 }
	r := db.WithContext(ctx).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Scan(&p)
	if r.Error != nil {
		return errV3Stream
	}
	if r.RowsAffected == 0 {
		return nil
	}
	if p.CursorBlock < c.AnchorBlock || p.CursorBlock > c.CursorBlock {
		return errV3Stream
	}
	return nil
}
