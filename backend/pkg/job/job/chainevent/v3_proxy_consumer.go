package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// Historical coverage is used only under a locked running stream revision,
// after validating the full contiguous prefix/digests and live end anchors.
// It never substitutes state calls or creation admission. Preparation repeats
// independent live checks; publication replays only that request's evidence
// and then rechecks live anchors outside the public row lock.
type v3CoverageRPC struct {
	v3ScannerRPC
	blocks map[uint64]v3StreamBlock
	proxy  common.Address
}

func (r v3CoverageRPC) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if n == nil || !n.IsUint64() {
		return r.v3ScannerRPC.HeaderByNumber(ctx, n)
	}
	b, ok := r.blocks[n.Uint64()]
	if !ok {
		return nil, errV3Stream
	}
	return types.CopyHeader(b.header()), nil
}
func (r v3CoverageRPC) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	if q.BlockHash != nil || q.FromBlock == nil || q.ToBlock == nil || !q.FromBlock.IsUint64() || !q.ToBlock.IsUint64() || q.FromBlock.Cmp(q.ToBlock) > 0 || q.ToBlock.Uint64()-q.FromBlock.Uint64() >= 128 || len(q.Addresses) != 1 || q.Addresses[0] != r.proxy || len(q.Topics) != 0 {
		return nil, errV3Stream
	}
	out := []types.Log{}
	for n := q.FromBlock.Uint64(); ; n++ {
		b, ok := r.blocks[n]
		if !ok {
			return nil, errV3Stream
		}
		out = append(out, v3CopyLogs(b.Logs)...)
		if n == q.ToBlock.Uint64() {
			break
		}
	}
	return out, nil
}
func (r v3CoverageRPC) TransactionReceipt(ctx context.Context, h common.Hash) (*types.Receipt, error) {
	for _, b := range r.blocks {
		if receipt, ok := b.Receipts[h.Hex()]; ok {
			return v3CloneReceipt((*types.Receipt)(receipt)), nil
		}
	}
	return nil, errV3Stream // never silently fall back across a coverage hole
}

func readV3Coverage(tx *gorm.DB, s v3StreamScope, start, end uint64) (map[uint64]v3StreamBlock, error) {
	if end < start || end-start > 128 {
		return nil, errV3Stream
	}
	var rows []struct {
		BlockNumber                            uint64
		BlockHash, ParentHash, Digest, Payload string
	}
	r := tx.Raw(`SELECT c.block_number,c.block_hash,c.parent_hash,c.digest,c.payload FROM ido_v3_proxy_coverage c JOIN ido_v3_proxy_stream s ON s.chain_id=c.chain_id AND s.proxy_address=c.proxy_address AND s.contract_version=c.contract_version AND s.epoch=c.epoch WHERE c.chain_id=? AND c.proxy_address=? AND c.contract_version=? AND c.block_number BETWEEN ? AND ? ORDER BY c.block_number`, append(s.args(), start, end)...).Scan(&rows)
	if r.Error != nil || len(rows) != int(end-start+1) {
		return nil, errV3Stream
	}
	blocks := make(map[uint64]v3StreamBlock, len(rows))
	var previous common.Hash
	for i, row := range rows {
		// Exact validated JSON text retains 256-bit numeric header/receipt fields.
		var b v3StreamBlock
		if json.Unmarshal([]byte(row.Payload), &b) != nil || b.Header == nil || b.header().Number == nil || !b.header().Number.IsUint64() {
			return nil, errV3Stream
		}
		_, digest, e := v3StreamPayload(b)
		if e != nil || digest != row.Digest || row.BlockNumber != start+uint64(i) || b.header().Number.Uint64() != row.BlockNumber || row.BlockHash != strings.ToLower(b.header().Hash().Hex()) || row.ParentHash != strings.ToLower(b.header().ParentHash.Hex()) || (i > 0 && b.header().ParentHash != previous) {
			return nil, errV3Stream
		}
		for _, l := range b.Logs {
			receipt, ok := b.Receipts[l.TxHash.Hex()]
			if !ok || receipt == nil || l.BlockHash != b.header().Hash() || l.BlockNumber != row.BlockNumber || v3StreamMatchReceipt((*types.Receipt)(receipt), l, s.Proxy) != nil {
				return nil, errV3Stream
			}
		}
		previous = b.header().Hash()
		blocks[row.BlockNumber] = b
	}
	return blocks, nil
}

// SET is connection/session scoped; nested GORM transactions stay on this
// pinned SQL transaction. Always reset before handing the connection back.
func v3StreamWriterTx(ctx context.Context, db *gorm.DB, work func(*gorm.DB) error) error {
	withMarker := func(conn *gorm.DB) (err error) {
		var prior int
		if conn.WithContext(ctx).Raw(`SELECT COALESCE(@jc_v3_stream_writer,0)`).Scan(&prior).Error != nil {
			return errV3Stream
		}
		if conn.WithContext(ctx).Exec(`SET @jc_v3_stream_writer=1`).Error != nil {
			return errV3Stream
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if e := conn.WithContext(cleanup).Exec(fmt.Sprintf(`SET @jc_v3_stream_writer=%d`, prior)).Error; e != nil {
				err = errV3Stream
			}
		}()
		return conn.WithContext(ctx).Transaction(work)
	}
	// Nested stream work already has the outer physical connection marker.
	// Top-level pinning is essential: an expired sql.Tx auto-rolls back and
	// cannot reset MySQL session variables, but its live sql.Conn still can.
	if _, nested := db.Statement.ConnPool.(gorm.TxCommitter); nested {
		return withMarker(db)
	}
	return db.WithContext(context.WithoutCancel(ctx)).Connection(withMarker)
}

// Retained as a differential-test reference until prepared consumption is
// independently validated. Production uses consumeV3Stream's explicit plan.
func consumeV3StreamLegacy(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity, limit uint64) error {
	if !s.valid() || limit == 0 || limit > 128 || b.ChainID != s.Chain || b.ProxyAddress != s.Proxy || runtime != s.Runtime {
		return errV3Stream
	}
	before, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil {
		return e
	}
	var published struct {
		CursorBlock, Revision uint64
		CursorHash            string
	}
	work := func(tx *gorm.DB, rpc v3ScannerRPC, lock bool) error {
		stream, e := v3StreamCursorRead(tx, s, lock)
		if e != nil || stream.Mode != "stream" || stream.State != "running" {
			return errV3Stream
		}
		if stream.Epoch != before.Epoch || stream.AnchorBlock != before.AnchorBlock || stream.CursorBlock < before.CursorBlock {
			return errV3Stream
		}
		stream = before // consume only the prepared immutable prefix
		var project struct {
			CursorBlock uint64
			CursorHash  string
			Revision    uint64
			State       string
		}
		r := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? FOR UPDATE`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion).Scan(&project)
		if r.Error != nil || r.RowsAffected != 1 || project.State != "running" || project.CursorBlock < stream.AnchorBlock || project.CursorBlock > stream.CursorBlock {
			return errV3Stream
		}
		end := stream.CursorBlock
		if end-project.CursorBlock > limit {
			end = project.CursorBlock + limit
		}
		blocks, e := readV3Coverage(tx, s, project.CursorBlock, end)
		if e != nil || strings.ToLower(blocks[project.CursorBlock].header().Hash().Hex()) != project.CursorHash {
			return errV3Stream
		}
		// Check the producer tip too: old consumer history must not serve while the
		// stream's current canonical anchor is unknown/replaced.
		if e = v3StreamAnchor(ctx, rpc, s, stream.CursorBlock, common.HexToHash(stream.CursorHash)); e != nil {
			return e
		}
		tip := blocks[end].header()
		if e = v3StreamAnchor(ctx, rpc, s, end, tip.Hash()); e != nil {
			return e
		}
		if end > project.CursorBlock {
			replay := v3CoverageRPC{v3ScannerRPC: rpc, blocks: blocks, proxy: s.Proxy}
			// Empty windows preserve the existing journal/projection/effect/outbox
			// transaction. Coverage's head is capped to this consumer batch.
			limited := v3CoverageHead{v3CoverageRPC: replay, head: end + s.Confirmations - 1}
			rangeCtx := ctx
			if evidence, ok := rpc.(*v3PreparedRPC); ok {
				rangeCtx = context.WithValue(ctx, v3PreparedBudgetKey{}, evidence)
			}
			if _, e = v3ScanApprovedConfirmedRange(rangeCtx, tx, limited, b, runtime, s.Confirmations, end-project.CursorBlock); e != nil {
				return e
			}
		}
		var actual struct {
			CursorBlock uint64
			CursorHash  string
			Revision    uint64
			State       string
		}
		r = tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion).Scan(&actual)
		if r.Error != nil || r.RowsAffected != 1 || actual.State != "running" || actual.CursorBlock < project.CursorBlock || actual.CursorBlock > end {
			return errV3Stream
		}
		if e = v3StreamAnchor(ctx, rpc, s, actual.CursorBlock, common.HexToHash(actual.CursorHash)); e != nil {
			return e
		}
		if e = v3StreamAnchor(ctx, rpc, s, stream.CursorBlock, common.HexToHash(stream.CursorHash)); e != nil {
			return e
		}
		update := tx.Exec(`INSERT INTO ido_v3_proxy_consumer(chain_id,proxy_address,contract_version,project_id,cursor_block,cursor_hash,revision,last_success_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE cursor_block=VALUES(cursor_block),cursor_hash=VALUES(cursor_hash),revision=VALUES(revision),last_success_at=VALUES(last_success_at)`, append(s.args(), b.ProjectID, actual.CursorBlock, actual.CursorHash, actual.Revision)...)
		if update.Error != nil {
			return errV3Stream
		}
		if evidence, ok := rpc.(*v3PreparedRPC); ok && evidence.replay {
			published.CursorBlock = actual.CursorBlock
			published.CursorHash = actual.CursorHash
			published.Revision = actual.Revision
		}
		return nil
	}
	// Probe journal work using only the recorded request-local evidence. On a
	// missing RPC, roll back BEFORE fetching it: writes/foreign keys/triggers
	// can implicitly lock the public row even without explicit FOR UPDATE.
	// Only a subsequent epoch/cursor-fenced short transaction publishes.
	var original struct {
		CursorBlock, Revision uint64
		CursorHash, State     string
	}
	if r := db.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion).Scan(&original); r.Error != nil || r.RowsAffected != 1 {
		return errV3Stream
	}
	prepared := newV3PreparedRPC(rpc)
	prepared.deferNetwork = true
	rollback := errors.New("V3 consumer prepared; rollback tentative writes")
	for probes := 0; probes < 4096; probes++ {
		prepared.position = 0
		prepared.pending = nil
		e = v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
			if e := work(tx, prepared, false); e != nil {
				return e
			}
			return rollback
		})
		if errors.Is(e, rollback) {
			break
		}
		// Scanner helpers deliberately wrap unavailable errors; the pending
		// slot, not the wrapper, identifies an interrupted evidence read.
		if prepared.pending == nil {
			return e
		}
		if e := prepared.pending(); e != nil {
			return e
		}
	}
	if !errors.Is(e, rollback) {
		return errV3Stream
	}
	prepared.deferNetwork = false
	prepared.replay = true
	prepared.position = 0
	e = v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		current, e := v3StreamCursorRead(tx, s, true)
		if e != nil || current.Mode != before.Mode || current.State != "running" || current.Epoch != before.Epoch || current.AnchorBlock != before.AnchorBlock || current.CursorBlock < before.CursorBlock {
			return errV3Stream
		}
		if current.Revision < before.Revision || (current.CursorBlock == before.CursorBlock && (current.Revision != before.Revision || current.CursorHash != before.CursorHash)) || (current.CursorBlock > before.CursorBlock && (current.Revision == before.Revision || current.Revision-before.Revision > current.CursorBlock-before.CursorBlock)) {
			return errV3Stream
		}
		prefix, e := readV3Coverage(tx, s, before.CursorBlock, before.CursorBlock)
		if e != nil || strings.ToLower(prefix[before.CursorBlock].header().Hash().Hex()) != before.CursorHash {
			return errV3Stream
		}
		var now struct {
			CursorBlock, Revision uint64
			CursorHash, State     string
		}
		if r := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? FOR UPDATE`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion).Scan(&now); r.Error != nil || r.RowsAffected != 1 || now != original {
			return errV3Stream
		}
		// Cap replay to the prepared public tip; producer may advance normally.
		if e := work(tx, prepared, true); e != nil {
			return e
		}
		if prepared.position != len(prepared.answers) {
			return errV3Stream
		}
		return nil
	})
	if e != nil {
		return e
	}
	// Live post-publication anchor fences remain OUTSIDE the public row lock.
	if published.Revision > 0 && v3StreamAnchor(ctx, rpc, s, published.CursorBlock, common.HexToHash(published.CursorHash)) == nil && v3StreamAnchor(ctx, rpc, s, before.CursorBlock, common.HexToHash(before.CursorHash)) == nil {
		return nil
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if published.Revision > 0 {
		if haltErr := v3StreamWriterTx(cleanup, db, func(tx *gorm.DB) error {
			return (v3Journal{db: tx}).haltRangeUnknown(cleanup, b, published.CursorBlock, published.Revision)
		}); haltErr != nil {
			return fmt.Errorf("%w: consumer post-publication revocation unavailable", errV3Stream)
		}
	}
	return errV3Stream
}

// Forward live head validation, cap only its returned traversal boundary to
// admitted coverage. Never manufacture a head when the source is behind it.
type v3CoverageHead struct {
	v3CoverageRPC
	head uint64
}

func (r v3CoverageHead) BlockNumber(ctx context.Context) (uint64, error) {
	h, e := r.v3ScannerRPC.BlockNumber(ctx)
	if e != nil || h < r.head {
		return 0, errV3Stream
	}
	return r.head, nil
}

// Shared halt is one transaction: no crash can leave a stopped producer with
// published project projections. Old immutable coverage is retained. Recovery
// is intentionally fail-closed; no automatic ancestor guess or raw tx replay.
func haltV3Stream(ctx context.Context, db *gorm.DB, s v3StreamScope) error {
	return v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		_, e := v3StreamCursorRead(tx, s, true)
		if e != nil {
			return e
		}
		var rows []struct {
			ProjectID                  uint64
			CursorBlock, Revision      uint64
			State                      string
			ProjectToken, PaymentToken string
		}
		if tx.Raw(`SELECT c.project_id,c.cursor_block,c.revision,c.state,p.token_address AS project_token,p.payment_token_address AS payment_token FROM ido_v3_scan_cursor c JOIN projects p ON p.project_id=c.project_id WHERE c.chain_id=? AND c.proxy_address=? AND c.contract_version=? AND BINARY p.status<>BINARY 'archived' ORDER BY c.project_id FOR UPDATE`, s.args()...).Scan(&rows).Error != nil {
			return errV3Stream
		}
		if e := invalidateV3TrackingSourceTx(tx, s); e != nil {
			return e
		}
		for _, row := range rows {
			if row.State == "running" {
				b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: row.ProjectID, ProjectToken: common.HexToAddress(row.ProjectToken), PaymentToken: common.HexToAddress(row.PaymentToken)}
				if e := (v3Journal{db: tx}).haltRangeUnknown(ctx, b, row.CursorBlock, row.Revision); e != nil {
					return e
				}
			}
		}
		if tx.Exec(`UPDATE ido_v3_approved_scan_health SET state='checking' WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Error != nil {
			return errV3Stream
		}
		return tx.Exec(`UPDATE ido_v3_proxy_stream SET state='halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state='running'`, s.args()...).Error
	})
}

func shadowV3Consumer(ctx context.Context, db *gorm.DB, s v3StreamScope) error {
	c, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil || c.State != "running" || c.Mode != "shadow" {
		return errV3Stream
	}
	// Read-only parity: compare every old canonical checkpoint covered so far.
	var mismatches int64
	e = db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint p JOIN ido_v3_proxy_coverage c ON c.chain_id=p.chain_id AND c.proxy_address=p.proxy_address AND c.contract_version=p.contract_version AND c.block_number=p.block_number JOIN ido_v3_proxy_stream s ON s.chain_id=c.chain_id AND s.proxy_address=c.proxy_address AND s.contract_version=c.contract_version AND s.epoch=c.epoch WHERE p.chain_id=? AND p.proxy_address=? AND p.contract_version=? AND p.status='canonical' AND BINARY p.block_hash<>BINARY c.block_hash`, s.args()...).Scan(&mismatches).Error
	if e != nil || mismatches != 0 {
		return errV3Stream
	}
	// Compare raw event placement too; matching tip hashes alone cannot prove
	// that the producer retained the event set consumed by the old journal.
	var events []struct {
		BlockNumber       uint64
		TxHash, BlockHash string
		LogIndex          uint
	}
	if db.WithContext(ctx).Raw(`SELECT j.block_number,j.block_hash,j.tx_hash,j.log_index FROM ido_v3_event_journal j WHERE j.chain_id=? AND j.proxy_address=? AND j.contract_version=? AND j.status='canonical' AND j.block_number>? AND j.block_number<=? ORDER BY j.block_number,j.log_index`, append(s.args(), c.AnchorBlock, c.CursorBlock)...).Scan(&events).Error != nil {
		return errV3Stream
	}
	for _, event := range events {
		blocks, e := readV3Coverage(db.WithContext(ctx), s, event.BlockNumber, event.BlockNumber)
		if e != nil {
			return e
		}
		found := 0
		for _, l := range blocks[event.BlockNumber].Logs {
			if l.Index == event.LogIndex && strings.EqualFold(l.TxHash.Hex(), event.TxHash) && strings.EqualFold(l.BlockHash.Hex(), event.BlockHash) {
				found++
			}
		}
		if found != 1 {
			return errV3Stream
		}
	}
	return nil
}
