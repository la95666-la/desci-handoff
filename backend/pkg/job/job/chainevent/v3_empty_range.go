package chainevent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

const v3EmptyRangeLimit uint64 = 16

// Fetch each header twice as before, but overlap independent HTTP reads.
// Every height, hash and parent link is still checked in order by the caller;
// no cached header, omitted block or JSON-RPC batch can certify an empty range.
func readV3EmptyHeaders(ctx context.Context, rpc v3ScannerRPC, start, end uint64) ([]*types.Header, error) {
	invalid := errors.New("V3 empty range headers unavailable")
	if ctx == nil || rpc == nil || start == 0 || end < start || end-start >= v3EmptyRangeLimit {
		return nil, invalid
	}
	headers := make([]*types.Header, end-start+1)
	jobs := make(chan uint64)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed bool
	var conflicting bool
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for height := range jobs {
				h, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
				if err != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != height || h.Hash() == (common.Hash{}) {
					mu.Lock()
					failed = true
					if err == nil {
						conflicting = true
					}
					mu.Unlock()
				} else {
					headers[height-start] = h
				}
			}
		}()
	}
dispatch:
	for height := start; ; height++ {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- height:
		}
		if height == end {
			break
		}
	}
	close(jobs)
	wg.Wait()
	if conflicting {
		return nil, v3Conflict("malformed_header")
	}
	if failed || ctx.Err() != nil {
		return nil, invalid
	}
	return headers, nil
}

// A published batch has already advanced the durable cursor. Its caller must
// not run the ordinary partial-block halt a second time using the old revision.
var errV3EmptyRangePostCommit = errors.New("V3 empty range post-commit check failed")

// scanV3EmptyRange returns the number of verified empty blocks. No caller may
// infer an empty block from a project-specific topic filter.
// The entire range is committed atomically; failures leave its old cursor.
func scanV3EmptyRange(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	b idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	start, end, confirmations uint64) (uint64, error) {
	invalid := errors.New("V3 empty range unavailable")
	if ctx == nil || db == nil || rpc == nil || b.ChainID == 0 || b.ProjectID == 0 ||
		b.ProxyAddress == (common.Address{}) || start < 2 || end < start || end-start >= v3EmptyRangeLimit || confirmations == 0 {
		return 0, invalid
	}
	from, to := new(big.Int).SetUint64(start), new(big.Int).SetUint64(end)
	query := ethereum.FilterQuery{FromBlock: from, ToBlock: to, Addresses: []common.Address{b.ProxyAddress}}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != b.ChainID {
		return 0, invalid
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, end, confirmations) {
		return 0, invalid
	}
	first, err := v3ReadBoundedLogs(v3FirstRead(ctx), rpc, query)
	if err != nil {
		return 0, invalid
	}
	if len(first) != 0 {
		// A later event must not force a known-empty prefix back through the
		// per-block path. The smaller prefix gets its OWN two full-filter reads;
		// a changed provider answer fails closed before publication.
		boundary := end
		for _, log := range first {
			if log.BlockNumber < start || log.BlockNumber > end || log.Address != b.ProxyAddress || log.Removed ||
				log.BlockHash == (common.Hash{}) || log.TxHash == (common.Hash{}) || len(log.Topics) == 0 {
				return 0, invalid
			}
			if log.BlockNumber < boundary {
				boundary = log.BlockNumber
			}
		}
		if boundary == start {
			return 0, nil // event at start takes full receipt-aware path
		}
		return scanV3EmptyRange(ctx, db, rpc, b, runtime, start, boundary-1, confirmations)
	}
	if err := checkV3CursorCanonical(ctx, db, rpc, b); err != nil {
		return 0, err
	}
	previous, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(start-1))
	if err != nil || previous == nil || previous.Number == nil || !previous.Number.IsUint64() || previous.Number.Uint64() != start-1 || previous.Hash() == (common.Hash{}) {
		return 0, invalid
	}
	parentHash := previous.Hash()
	blocks, err := readV3EmptyHeaders(v3FirstRead(ctx), rpc, start, end)
	if err != nil {
		return 0, invalid
	}
	for _, h := range blocks {
		if h.ParentHash != parentHash {
			return 0, invalid
		}
		parentHash = h.Hash()
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(b.ChainID), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: b.ProxyAddress}
	begin, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, start, blocks[0].Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(b, begin) != nil {
		return 0, invalid
	}
	finish, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, end, blocks[len(blocks)-1].Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(b, finish) != nil {
		return 0, invalid
	}
	begin.BlockNumber, begin.BlockHash = 0, common.Hash{}
	finish.BlockNumber, finish.BlockHash = 0, common.Hash{}
	if begin != finish {
		return 0, invalid
	}
	policyStart, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, runtime, start, blocks[0].Hash())
	if err != nil {
		return 0, invalid
	}
	policyEnd, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, runtime, end, blocks[len(blocks)-1].Hash())
	if err != nil || policyStart.Policy != policyEnd.Policy {
		return 0, invalid
	}
	second, err := v3ReadBoundedLogs(ctx, rpc, query)
	if err != nil || len(second) != 0 {
		return 0, invalid
	}
	confirmed, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(confirmed, end, confirmations) {
		return 0, invalid
	}
	repeated, err := readV3EmptyHeaders(ctx, rpc, start, end)
	if err != nil {
		return 0, invalid
	}
	for i, block := range blocks {
		again := repeated[i]
		if again.Hash() != block.Hash() || again.ParentHash != block.ParentHash {
			return 0, invalid
		}
	}
	revision, err := (v3Journal{db: db}).applyVerifiedEmptyRange(ctx, b, blocks, previous.Hash(), begin)
	if err != nil {
		return 0, err
	}
	// Publication happened in one transaction. A provider changing the final
	// canonical anchor or confirmation depth afterwards must revoke the exact
	// new revision; never report a successful prefix while serving it.
	if e := checkV3CursorCanonical(ctx, db, rpc, b); e != nil {
		haltCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if (v3Journal{db: db}).haltRangeUnknown(haltCtx, b, end, revision) != nil {
			return uint64(len(blocks)), fmt.Errorf("%w: revocation unavailable", errV3EmptyRangePostCommit)
		}
		return uint64(len(blocks)), errV3EmptyRangePostCommit
	}
	finalParent := previous
	if len(blocks) > 1 {
		finalParent = blocks[len(blocks)-2]
	}
	parentAgain, parentErr := rpc.HeaderByNumber(ctx, finalParent.Number)
	if parentErr != nil || parentAgain == nil || parentAgain.Hash() != finalParent.Hash() ||
		parentAgain.Number == nil || parentAgain.Number.Cmp(finalParent.Number) != 0 {
		haltCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if (v3Journal{db: db}).haltRangeUnknown(haltCtx, b, end, revision) != nil {
			return uint64(len(blocks)), fmt.Errorf("%w: parent revocation unavailable", errV3EmptyRangePostCommit)
		}
		return uint64(len(blocks)), errV3EmptyRangePostCommit
	}
	finalHead, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(finalHead, end, confirmations) {
		haltCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if (v3Journal{db: db}).haltRangeUnknown(haltCtx, b, end, revision) != nil {
			return uint64(len(blocks)), fmt.Errorf("%w: depth revocation unavailable", errV3EmptyRangePostCommit)
		}
		return uint64(len(blocks)), errV3EmptyRangePostCommit
	}
	return uint64(len(blocks)), nil
}

func (j v3Journal) applyVerifiedEmptyRange(ctx context.Context, b idov3whitelist.ExpectedProjectBinding,
	blocks []*types.Header, parent common.Hash, observed idov3whitelist.ProjectIdentityObservation) (uint64, error) {
	return j.applyVerifiedEmptyRangeBounded(ctx, b, blocks, parent, observed, v3EmptyRangeLimit)
}

// Only the private event-driven public-stream path may supply bounded catchup
// verified quiet heights. Live legacy scanning retains its 16-height boundary;
// the old prepared consumer remains bounded at 128.
func (j v3Journal) applyVerifiedEmptyRangeBounded(ctx context.Context, b idov3whitelist.ExpectedProjectBinding, blocks []*types.Header, parent common.Hash, observed idov3whitelist.ProjectIdentityObservation, limit uint64) (uint64, error) {
	invalid := errors.New("V3 empty range application unavailable")
	if j.db == nil || ctx == nil || limit == 0 || limit > v3ConsumerBlockLimit(ctx) || len(blocks) == 0 || len(blocks) > int(limit) || parent == (common.Hash{}) ||
		observed.ProjectID != b.ProjectID || observed.ChainID != b.ChainID || blocks[0] == nil || blocks[0].Number == nil {
		return 0, invalid
	}
	proxy := v3Lower(b.ProxyAddress)
	var publishedRevision uint64
	err := j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Hash     string `gorm:"column:cursor_hash"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		read := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, b.ChainID, proxy, b.ProjectID).Scan(&cursor)
		if read.Error != nil || read.RowsAffected != 1 || cursor.State != "running" || cursor.Block == math.MaxUint64 ||
			cursor.Block+1 != blocks[0].Number.Uint64() || cursor.Hash != strings.ToLower(parent.Hex()) ||
			cursor.Revision == 0 || cursor.Revision > math.MaxUint64-uint64(len(blocks)) {
			return invalid
		}
		var checkpoint struct {
			Hash string `gorm:"column:block_hash"`
		}
		read = tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND status='canonical'`, b.ChainID, proxy, b.ProjectID, cursor.Block).Scan(&checkpoint)
		if read.Error != nil || read.RowsAffected != 1 || checkpoint.Hash != cursor.Hash {
			return invalid
		}
		var existing int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND status='canonical' AND block_number>?`, b.ChainID, proxy, b.ProjectID, cursor.Block).Scan(&existing).Error != nil || existing != 0 {
			return invalid
		}
		var projection struct {
			State     string `gorm:"column:state"`
			Raised    string `gorm:"column:raised_raw"`
			Sold      string `gorm:"column:sold_raw"`
			Deposited string `gorm:"column:deposited_raw"`
			Refunded  string `gorm:"column:refunded_raw"`
			Claimed   string `gorm:"column:claimed_raw"`
			Wallets   string `gorm:"column:wallets"`
			Proceeds  string `gorm:"column:proceeds_paid_raw"`
			Fee       string `gorm:"column:fee_paid_raw"`
			Tokens    string `gorm:"column:project_tokens_withdrawn_raw"`
			Revision  uint64 `gorm:"column:revision"`
			Block     uint64 `gorm:"column:cursor_block"`
			Hash      string `gorm:"column:cursor_hash"`
		}
		const projectionQuery = `SELECT state,raised_raw,sold_raw,deposited_raw,refunded_raw,claimed_raw,wallets,proceeds_paid_raw,fee_paid_raw,project_tokens_withdrawn_raw,revision,cursor_block,cursor_hash FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`
		read = tx.Raw(projectionQuery+` FOR UPDATE`, b.ChainID, proxy, b.ProjectID).Scan(&projection)
		if read.Error != nil || read.RowsAffected != 1 || projection.Revision != cursor.Revision || projection.Block != cursor.Block || projection.Hash != cursor.Hash {
			return invalid
		}
		// Preserve corrupt-projection rejection without writing/re-reading the
		// same projection. Removing the full replay requires durable provenance;
		// no unaudited cache is allowed to turn corruption into a valid balance.
		v3TracePhase(ctx, "empty_projection_replay")
		expected := v3ParityProjection{State: projection.State, RaisedRaw: projection.Raised, SoldRaw: projection.Sold, DepositedRaw: projection.Deposited, RefundedRaw: projection.Refunded, ClaimedRaw: projection.Claimed, Wallets: projection.Wallets, ProceedsPaidRaw: projection.Proceeds, FeePaidRaw: projection.Fee, ProjectTokensWithdrawnRaw: projection.Tokens}
		if replayV3ProjectTx(tx, b, cursor.Block, parent, cursor.Revision, &expected) != nil {
			return invalid
		}
		states := map[string]idov3whitelist.ProjectState{"pending": idov3whitelist.ProjectPending, "active": idov3whitelist.ProjectActive, "successful": idov3whitelist.ProjectSuccessful, "failed": idov3whitelist.ProjectFailed, "cancelled": idov3whitelist.ProjectCancelled}
		state, ok := states[projection.State]
		raised, rOK := new(big.Int).SetString(projection.Raised, 10)
		sold, sOK := new(big.Int).SetString(projection.Sold, 10)
		if !ok || !rOK || !sOK || projection.Raised != raised.String() || projection.Sold != sold.String() {
			return invalid
		}
		last := blocks[len(blocks)-1]
		v3TracePhase(ctx, "empty_project_counters")
		observed.BlockNumber, observed.BlockHash = last.Number.Uint64(), last.Hash()
		if commonchain.MatchV3ReplayProjectEndOfBlock(b, commonchain.V3ReplayTotals{State: state, Raised: raised, Sold: sold}, observed, last.Number.Uint64(), last.Hash()) != nil {
			return invalid
		}
		v3TracePhase(ctx, "empty_creation_match")
		created, err := readV3CanonicalCreationTx(tx, b, cursor.Block)
		if err != nil || commonchain.MatchV3CreationProjectImmutable(created, b, observed) != nil {
			return invalid
		}
		v3TracePhase(ctx, "empty_business_apply")
		if err := applyV3BusinessStateTx(tx, b, projection.State); err != nil {
			return invalid
		}
		previous := parent
		v3TracePhase(ctx, "empty_checkpoint_publish")
		// One statement per complete window, not one SQL round trip per
		// height. The unique checkpoint fence still aborts the entire window.
		values := make([]string, 0, len(blocks))
		args := make([]any, 0, 5*len(blocks))
		for _, block := range blocks {
			if block == nil || block.Number == nil || block.Number.Uint64() != cursor.Block+1 || block.ParentHash != previous {
				return invalid
			}
			values = append(values, `(?,?,?,'ido_v3_stablecoin',?,?,'canonical')`)
			args = append(args, b.ChainID, proxy, b.ProjectID, block.Number.Uint64(), strings.ToLower(block.Hash().Hex()))
			cursor.Block++
			previous = block.Hash()
		}
		result := tx.Exec(`INSERT INTO ido_v3_block_checkpoint (chain_id,proxy_address,project_id,contract_version,block_number,block_hash,status) VALUES `+strings.Join(values, ","), args...)
		if result.Error != nil || result.RowsAffected != int64(len(blocks)) {
			return invalid
		}
		hash := strings.ToLower(previous.Hex())
		revision := cursor.Revision + uint64(len(blocks))
		if result := tx.Exec(`UPDATE ido_v3_project_projection SET revision=?,cursor_block=?,cursor_hash=? WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND revision=? AND cursor_block=? AND cursor_hash=?`, revision, cursor.Block, hash, b.ChainID, proxy, b.ProjectID, cursor.Revision, blocks[0].Number.Uint64()-1, cursor.Hash); result.Error != nil || result.RowsAffected != 1 {
			return invalid
		}
		if result := tx.Exec(`UPDATE ido_v3_scan_cursor SET revision=?,cursor_block=?,cursor_hash=? WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND state='running' AND revision=? AND cursor_block=? AND cursor_hash=?`, revision, cursor.Block, hash, b.ChainID, proxy, b.ProjectID, cursor.Revision, blocks[0].Number.Uint64()-1, cursor.Hash); result.Error != nil || result.RowsAffected != 1 {
			return invalid
		}
		publishedRevision = revision
		return nil
	})
	if err != nil {
		return 0, err
	}
	return publishedRevision, nil
}
