package chainevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

const v3StreamVersion = "ido_v3_stablecoin"

var errV3Stream = errors.New("V3 proxy stream unavailable")
var errV3StreamConflict = errors.New("V3 proxy stream evidence conflict")

type v3StreamScope struct {
	Chain         uint64
	Proxy         common.Address
	Runtime       idov3whitelist.RuntimeIdentity
	Confirmations uint64
}

func (s v3StreamScope) args() []any { return []any{s.Chain, v3Lower(s.Proxy), v3StreamVersion} }
func (s v3StreamScope) valid() bool {
	return s.Chain > 0 && s.Proxy != (common.Address{}) && s.Runtime.Implementation != (common.Address{}) && s.Runtime.ProxyCodeHash != (common.Hash{}) && s.Runtime.CodeHash != (common.Hash{}) && s.Confirmations > 0
}

type v3StreamCursor struct {
	Mode, State              string
	AnchorBlock, CursorBlock uint64
	CursorHash               string
	Revision                 uint64
	Epoch                    uint64
	ObservedHead             uint64
	LastSuccessAt            time.Time
}

// Alias types preserve ALL metadata without geth Receipt's binary/gob encoding
// (which drops inclusion fields) or its strict JSON required-field roundtrip.
type v3StreamHeader types.Header
type v3StreamReceipt types.Receipt
type v3StreamBlock struct {
	Header       *v3StreamHeader
	Logs         []types.Log
	Receipts     map[string]*v3StreamReceipt
	RetiredTests []uint64 `json:",omitempty"` // explicit operator baseline, NOT historical coverage
}

func (b v3StreamBlock) header() *types.Header { return (*types.Header)(b.Header) }
func v3StreamPayload(b v3StreamBlock) (string, string, error) {
	raw, e := json.Marshal(b)
	sum := sha256.Sum256(raw)
	return string(raw), hex.EncodeToString(sum[:]), e
}

func v3StreamCursorRead(db *gorm.DB, s v3StreamScope, lock bool) (v3StreamCursor, error) {
	var c v3StreamCursor
	q := `SELECT mode,state,anchor_block,cursor_block,cursor_hash,revision,epoch,observed_head,last_success_at FROM ido_v3_proxy_stream WHERE chain_id=? AND proxy_address=? AND contract_version=?`
	if lock {
		q += " FOR UPDATE"
	}
	r := db.Raw(q, s.args()...).Scan(&c)
	if r.Error != nil || r.RowsAffected != 1 || c.Revision == 0 || c.Revision == math.MaxUint64 {
		return c, errV3Stream
	}
	return c, nil
}

func v3StreamAnchor(ctx context.Context, rpc v3ScannerRPC, s v3StreamScope, height uint64, hash common.Hash) error {
	if ctx == nil || ctx.Err() != nil {
		return errV3Stream
	}
	chain, e := rpc.ChainID(ctx)
	if e != nil || chain == nil || !chain.IsUint64() || chain.Uint64() != s.Chain {
		return errV3Stream
	}
	h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if e != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != height {
		return errV3Stream
	}
	if h.Hash() != hash {
		return errV3StreamConflict
	}
	head, e := rpc.BlockNumber(ctx)
	if e != nil || !idov3whitelist.HasConfirmations(head, height, s.Confirmations) {
		return errV3Stream
	}
	if ctx.Err() != nil {
		return errV3Stream
	}
	return nil
}

func v3StreamRuntime(ctx context.Context, rpc v3ScannerRPC, s v3StreamScope, h *types.Header) error {
	n := h.Number
	for _, v := range []struct {
		a    common.Address
		hash common.Hash
	}{{s.Proxy, s.Runtime.ProxyCodeHash}, {s.Runtime.Implementation, s.Runtime.CodeHash}} {
		code, e := rpc.CodeAt(ctx, v.a, n)
		if e != nil {
			return errV3Stream
		}
		if crypto.Keccak256Hash(code) != v.hash {
			return errV3StreamConflict
		}
	}
	slot, e := rpc.StorageAt(ctx, s.Proxy, common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"), n)
	if e != nil {
		return e
	}
	if len(slot) != 32 || common.BytesToAddress(slot) != s.Runtime.Implementation || new(big.Int).SetBytes(slot[:12]).Sign() != 0 {
		return v3Conflict("implementation_slot")
	}
	v, e := rpc.CallContract(ctx, ethereum.CallMsg{To: &s.Proxy, Data: crypto.Keccak256([]byte("contractVersion()"))[:4]}, n)
	if e != nil {
		return e
	}
	if len(v) != 32 || common.BytesToHash(v) != crypto.Keccak256Hash([]byte("joincare.ido_v3_stablecoin.v1")) {
		return v3Conflict("contract_version")
	}
	return v3StreamAnchor(ctx, rpc, s, h.Number.Uint64(), h.Hash())
}

// Explicit bootstrap anchor must be at/before EVERY current project cursor.
// Never chooses MAX(cursor) or manufactures project/financial history.
func bootstrapV3Stream(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, mode string, anchor uint64, hash common.Hash) error {
	if !s.valid() || (mode != "shadow" && mode != "stream") || anchor == 0 || hash == (common.Hash{}) {
		return errV3Stream
	}
	if e := v3StreamAnchor(ctx, rpc, s, anchor, hash); e != nil {
		return e
	}
	h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(anchor))
	if e != nil || h == nil || h.Hash() != hash {
		return errV3Stream
	}
	if e = v3StreamRuntime(ctx, rpc, s, h); e != nil {
		return e
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var below int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND contract_version=? AND cursor_block<?`, append(s.args(), anchor)...).Scan(&below).Error != nil || below != 0 {
			return errV3Stream
		}
		r := tx.Exec(`INSERT INTO ido_v3_proxy_stream(chain_id,proxy_address,contract_version,mode,state,anchor_block,cursor_block,cursor_hash,revision) VALUES(?,?,?,?,'running',?,?,?,1)`, append(s.args(), mode, anchor, anchor, strings.ToLower(hash.Hex()))...)
		if r.Error != nil || r.RowsAffected != 1 {
			return errV3Stream
		}
		payload, digest, e := v3StreamPayload(v3StreamBlock{Header: (*v3StreamHeader)(h), Logs: []types.Log{}, Receipts: map[string]*v3StreamReceipt{}})
		if e != nil {
			return e
		}
		return insertV3Coverage(tx, s, h, payload, digest)
	})
}

func insertV3Coverage(tx *gorm.DB, s v3StreamScope, h *types.Header, payload, digest string) error {
	r := tx.Exec(`INSERT INTO ido_v3_proxy_coverage(chain_id,proxy_address,contract_version,epoch,block_number,block_hash,parent_hash,digest,payload) SELECT chain_id,proxy_address,contract_version,epoch,?,?,?,?,? FROM ido_v3_proxy_stream WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state='running'`, append([]any{h.Number.Uint64(), strings.ToLower(h.Hash().Hex()), strings.ToLower(h.ParentHash.Hex()), digest, payload}, s.args()...)...)
	if r.Error != nil || r.RowsAffected != 1 {
		return errV3Stream
	}
	return nil
}

// Collect a complete bounded Proxy range: every header twice, full logs twice,
// each unique tx receipt twice, then a live canonical/depth boundary. Project
// routing is deliberately deferred until the admitted consumer.
func collectV3Stream(ctx context.Context, rpc v3ScannerRPC, s v3StreamScope, start, end uint64, parent common.Hash) ([]v3StreamBlock, error) {
	if ctx == nil || ctx.Err() != nil || !s.valid() || start == 0 || end < start || end-start >= 128 || end == math.MaxUint64 {
		return nil, errV3Stream
	}
	blocks := make([]v3StreamBlock, end-start+1)
	for offset := uint64(0); offset <= end-start; {
		last := offset + v3EmptyRangeLimit - 1
		if last > end-start {
			last = end - start
		}
		hs, e := readV3EmptyHeaders(ctx, rpc, start+offset, start+last)
		if e != nil {
			return nil, e
		}
		for i, h := range hs {
			if h.ParentHash != parent {
				return nil, errV3StreamConflict
			}
			parent = h.Hash()
			blocks[offset+uint64(i)] = v3StreamBlock{Header: (*v3StreamHeader)(types.CopyHeader(h)), Logs: []types.Log{}, Receipts: map[string]*v3StreamReceipt{}}
		}
		offset = last + 1
	}
	if e := v3StreamRuntime(ctx, rpc, s, blocks[0].header()); e != nil {
		return nil, e
	}
	if e := v3StreamRuntime(ctx, rpc, s, blocks[len(blocks)-1].header()); e != nil {
		return nil, e
	}
	q := ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(start), ToBlock: new(big.Int).SetUint64(end), Addresses: []common.Address{s.Proxy}}
	logs, e := v3ReadBoundedLogs(ctx, rpc, q)
	if e != nil {
		return nil, e
	}
	if len(logs) > 4096 {
		return nil, errV3Stream
	}
	logs = v3CopyLogs(logs)
	sort.Slice(logs, func(i, j int) bool {
		if logs[i].BlockNumber != logs[j].BlockNumber {
			return logs[i].BlockNumber < logs[j].BlockNumber
		}
		return logs[i].Index < logs[j].Index
	})
	receipts := map[common.Hash]*types.Receipt{}
	for i, l := range logs {
		if l.BlockNumber < start || l.BlockNumber > end || l.Address != s.Proxy || l.Removed || len(l.Topics) == 0 || l.TxHash == (common.Hash{}) || l.BlockHash != blocks[l.BlockNumber-start].header().Hash() {
			return nil, errV3StreamConflict
		}
		if i > 0 && logs[i-1].BlockNumber == l.BlockNumber && logs[i-1].Index == l.Index {
			return nil, errV3StreamConflict
		}
		_, financial := v3ProjectEventTopics[l.Topics[0]]
		_, global := v3NonProjectTopics[l.Topics[0]]
		if !financial && !global {
			return nil, errV3StreamConflict
		}
		if (financial || l.Topics[0] == v3PolicyUpdateTopic) && len(l.Topics) < 2 {
			return nil, errV3Stream
		}
		r, ok := receipts[l.TxHash]
		if !ok {
			if len(receipts) >= 1024 {
				return nil, errV3Stream
			}
			r, e = rpc.TransactionReceipt(ctx, l.TxHash)
			if e != nil || r == nil {
				return nil, errV3Stream
			}
			r = v3CloneReceipt(r)
			receipts[l.TxHash] = r
		}
		if e := v3StreamMatchReceipt(r, l, s.Proxy); e != nil {
			return nil, e
		}
		b := &blocks[l.BlockNumber-start]
		b.Logs = append(b.Logs, l)
		b.Receipts[l.TxHash.Hex()] = (*v3StreamReceipt)(r)
	}
	// Every Proxy log in every fetched receipt must be in the complete filter.
	for _, r := range receipts {
		for _, l := range r.Logs {
			if l != nil && l.Address == s.Proxy {
				found := false
				for _, want := range logs {
					if v3EqualLog(*l, want) {
						found = true
						break
					}
				}
				if !found {
					return nil, errV3StreamConflict
				}
			}
		}
	}
	again, e := v3ReadBoundedLogs(ctx, rpc, q)
	if e != nil {
		return nil, errV3Stream
	}
	if len(again) != len(logs) {
		return nil, errV3StreamConflict
	}
	again = v3CopyLogs(again)
	sort.Slice(again, func(i, j int) bool {
		if again[i].BlockNumber != again[j].BlockNumber {
			return again[i].BlockNumber < again[j].BlockNumber
		}
		return again[i].Index < again[j].Index
	})
	for i := range logs {
		if !v3EqualLog(logs[i], again[i]) {
			return nil, errV3StreamConflict
		}
	}
	for hash, first := range receipts {
		r, e := rpc.TransactionReceipt(ctx, hash)
		if e != nil || r == nil {
			return nil, errV3Stream
		}
		a, _ := json.Marshal((*v3StreamReceipt)(first))
		b, _ := json.Marshal((*v3StreamReceipt)(r))
		if string(a) != string(b) {
			return nil, errV3StreamConflict
		}
	}
	for offset := uint64(0); offset <= end-start; {
		last := offset + v3EmptyRangeLimit - 1
		if last > end-start {
			last = end - start
		}
		hs, e := readV3EmptyHeaders(ctx, rpc, start+offset, start+last)
		if e != nil {
			return nil, e
		}
		for i, h := range hs {
			if h.Hash() != blocks[offset+uint64(i)].header().Hash() {
				return nil, errV3StreamConflict
			}
		}
		offset = last + 1
	}
	if e := v3StreamAnchor(ctx, rpc, s, end, blocks[len(blocks)-1].header().Hash()); e != nil {
		return nil, e
	}
	return blocks, nil
}

func v3EqualLog(a, b types.Log) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func v3StreamMatchReceipt(r *types.Receipt, l types.Log, proxy common.Address) error {
	if r.Status != 1 || r.TxHash != l.TxHash || r.BlockNumber == nil || !r.BlockNumber.IsUint64() || r.BlockNumber.Uint64() != l.BlockNumber || r.BlockHash != l.BlockHash {
		return errV3StreamConflict
	}
	count := 0
	for _, v := range r.Logs {
		if v != nil && v.Index == l.Index {
			if v.Address != proxy || !v3EqualLog(*v, l) {
				return errV3StreamConflict
			}
			count++
		}
	}
	if count != 1 {
		return errV3StreamConflict
	}
	return nil
}

func produceV3Stream(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, limit uint64) error {
	if limit == 0 || limit > 128 {
		return errV3Stream
	}
	old, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil || old.State != "running" {
		return errV3Stream
	}
	if e = v3StreamAnchor(ctx, rpc, s, old.CursorBlock, common.HexToHash(old.CursorHash)); e != nil {
		return e
	}
	head, e := rpc.BlockNumber(ctx)
	if e != nil || head < s.Confirmations {
		return errV3Stream
	}
	end := head - s.Confirmations + 1
	if end <= old.CursorBlock {
		r := db.WithContext(ctx).Exec(`UPDATE ido_v3_proxy_stream SET observed_head=?,last_success_at=UTC_TIMESTAMP(6) WHERE chain_id=? AND proxy_address=? AND contract_version=? AND revision=? AND state='running'`, append([]any{head}, append(s.args(), old.Revision)...)...)
		if r.Error != nil || r.RowsAffected != 1 {
			return errV3Stream
		}
		return nil
	}
	if end-old.CursorBlock > limit {
		end = old.CursorBlock + limit
	}
	blocks, e := collectV3Stream(ctx, rpc, s, old.CursorBlock+1, end, common.HexToHash(old.CursorHash))
	if e != nil {
		return e
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, e := v3StreamCursorRead(tx, s, true)
		if e != nil {
			return errV3Stream
		}
		if current != old {
			return v3StreamSnapshotChanged(old, current)
		}
		if e = v3StreamAnchor(ctx, rpc, s, end, blocks[len(blocks)-1].header().Hash()); e != nil {
			return e
		}
		for _, b := range blocks {
			payload, digest, e := v3StreamPayload(b)
			if e != nil || len(payload) > 8<<20 {
				return errV3Stream
			}
			if e = insertV3Coverage(tx, s, b.header(), payload, digest); e != nil {
				return e
			}
		}
		if err := wakeV3TrackingFromCoverageTx(tx, s, blocks); err != nil {
			return err
		}
		// A reorg while persisting a long batch must not leave its public
		// cursor advertised as canonical after the pre-insert observation.
		if e = v3StreamAnchor(ctx, rpc, s, end, blocks[len(blocks)-1].header().Hash()); e != nil {
			return e
		}
		r := tx.Exec(`UPDATE ido_v3_proxy_stream SET cursor_block=?,cursor_hash=?,observed_head=?,revision=revision+1,last_success_at=UTC_TIMESTAMP(6) WHERE chain_id=? AND proxy_address=? AND contract_version=? AND revision=? AND state='running'`, append([]any{end, strings.ToLower(blocks[len(blocks)-1].header().Hash().Hex()), head}, append(s.args(), old.Revision)...)...)
		if r.Error != nil || r.RowsAffected != 1 {
			return errV3Stream
		}
		return nil
	})
}
