package chainevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
)

// Rebaseline is NOT recovery. Every current binding must be explicitly listed
// as retired by the operator. Old journal/outbox/nonce rows remain unchanged;
// project authority is withdrawn, never rewritten to successful application.
func rebaselineV3Tests(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, expected uint64, ids []uint64, baseline uint64, hash common.Hash) error {
	if !s.valid() || expected == 0 || len(ids) == 0 || baseline == 0 || hash == (common.Hash{}) {
		return errV3Stream
	}
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] >= id) {
			return errV3Stream
		}
	}
	if e := v3StreamAnchor(ctx, rpc, s, baseline, hash); e != nil {
		return e
	}
	head, e := rpc.BlockNumber(ctx)
	if e != nil || head < baseline || head-baseline > 128 {
		return errV3Stream
	}
	h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(baseline))
	if e != nil || h == nil || h.Hash() != hash {
		return errV3Stream
	}
	if e = v3StreamRuntime(ctx, rpc, s, h); e != nil {
		return e
	}
	return v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		c, e := v3StreamCursorRead(tx, s, true)
		if e != nil || c.Mode != "shadow" || c.Revision != expected || c.Epoch == math.MaxUint64 || c.CursorBlock >= baseline {
			return errV3Stream
		}
		// Serialize against the approved Set writer's runtime lock. New bindings
		// cannot appear halfway through the exact retirement list comparison.
		var runtime uint64
		if r := tx.Raw(`SELECT chain_id FROM ido_v3_runtime WHERE chain_id=? AND proxy_address=? AND contract_version=? FOR UPDATE`, s.args()...).Scan(&runtime); r.Error != nil || r.RowsAffected != 1 {
			return errV3Stream
		}
		var rows []struct{ ProjectID uint64 }
		r := tx.Raw(`SELECT project_id FROM ido_v3_project_binding WHERE chain_id=? AND proxy_address=? AND contract_version=? ORDER BY project_id FOR UPDATE`, s.args()...).Scan(&rows)
		if r.Error != nil || len(rows) != len(ids) {
			return errV3Stream
		}
		for i, row := range rows {
			if row.ProjectID != ids[i] {
				return errV3Stream
			}
		}
		// Persist exact retirement receipt before advancing the new epoch. This
		// reuses immutable coverage payload as an audit envelope at the baseline.
		for _, id := range ids {
			var project struct{ Status string }
			r := tx.Raw(`SELECT status FROM projects WHERE project_id=? AND chain_id=? AND contract_address=? AND contract_version=? FOR UPDATE`, id, s.Chain, v3Lower(s.Proxy), v3StreamVersion).Scan(&project)
			if r.Error != nil || r.RowsAffected != 1 || project.Status == "archived" {
				return errV3Stream
			}
			if tx.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Error != nil {
				return errV3Stream
			}
			if tx.Exec(`UPDATE ido_v3_financial_effect SET status='suspended' WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND status='canonical'`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Error != nil {
				return errV3Stream
			}
			if v3PolicyJournalEnabled() {
				if tx.Exec(`UPDATE ido_v3_policy_event_journal SET state='suspended' WHERE chain_id=? AND proxy_address=? AND project_id=? AND state='canonical'`, s.Chain, v3Lower(s.Proxy), id).Error != nil {
					return errV3Stream
				}
			}
			// Deliberately do not suspend/restore outbox or release any nonce. Frozen
			// old cursor proves it is no longer a source for proof/finance admission.
			if tx.Exec(`UPDATE ido_v3_scan_cursor SET state='reorg_halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND state<>'reorg_halted'`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Error != nil {
				return errV3Stream
			}
			update := tx.Exec(`UPDATE projects SET status='archived' WHERE project_id=? AND chain_id=? AND contract_address=? AND contract_version=? AND BINARY status=BINARY ?`, id, s.Chain, v3Lower(s.Proxy), v3StreamVersion, project.Status)
			if update.Error != nil || update.RowsAffected != 1 {
				return errV3Stream
			}
		}
		if e = v3StreamAnchor(ctx, rpc, s, baseline, hash); e != nil {
			return e
		}
		update := tx.Exec(`UPDATE ido_v3_proxy_stream SET mode='stream',state='running',anchor_block=?,cursor_block=?,cursor_hash=?,revision=revision+1,epoch=epoch+1,observed_head=?,last_success_at=UTC_TIMESTAMP(6) WHERE chain_id=? AND proxy_address=? AND contract_version=? AND revision=? AND mode='shadow'`, append([]any{baseline, baseline, strings.ToLower(hash.Hex()), head}, append(s.args(), expected)...)...)
		if update.Error != nil || update.RowsAffected != 1 {
			return errV3Stream
		}
		b := v3StreamBlock{Header: (*v3StreamHeader)(types.CopyHeader(h)), Logs: []types.Log{}, Receipts: map[string]*v3StreamReceipt{}, RetiredTests: ids}
		payload, digest, e := v3StreamPayload(b)
		if e != nil {
			return e
		}
		if e = insertV3Coverage(tx, s, h, payload, digest); e != nil {
			return e
		}
		return tx.Exec(`UPDATE ido_v3_approved_scan_health SET state='checking',last_success_at=NULL WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Error
	})
}

func RetiredV3TestDigest(ids []uint64) string {
	raw, _ := json.Marshal(ids)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func OperatorV3TestRebaseline(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, expected uint64, ids []uint64, baseline uint64, hash common.Hash, digest string) error {
	if digest != RetiredV3TestDigest(ids) {
		return errV3Stream
	}
	return rebaselineV3Tests(ctx, db, rpc, productionV3StreamScope(), expected, ids, baseline, hash)
}
