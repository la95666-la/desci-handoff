package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
)

// v3Journal is an isolated persistence engine. Its caller must
// obtain a trusted runtime/project admission and canonical block/receipt
// evidence before calling applyChecked. The registered Job supplies the
// admission; no public route may construct it from a candidate row or event.
// All V1/V2 tables remain untouched.
type v3Journal struct {
	db            *gorm.DB
	policyRPC     v3ScannerRPC
	policyRuntime idov3whitelist.RuntimeIdentity
	// Request-local policy evidence was fully checked outside the writer.
	preparedPolicy []*commonchain.V3PolicyEvent
	// Only the signed Set bootstrap bridge may require an original operator
	// attempt. The unregistered synthetic journal fixture cannot grant this.
	requireCreateAttempt bool
}

// haltRangeUnknown revokes only the exact revision this range invocation just
// committed when its post-commit canonical check cannot establish a healthy
// cursor. It does not label an RPC outage a reorg or orphan any facts. An
// unrelated worker's newer cursor must never be revoked by a stale caller.
func (j v3Journal) haltRangeUnknown(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, height, revision uint64) error {
	if j.db == nil || ctx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) || height == 0 || revision == 0 {
		return errors.New("V3 range halt scope invalid")
	}
	proxy := v3Lower(binding.ProxyAddress)
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		read := tx.Raw(`SELECT cursor_block,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, binding.ChainID, proxy, binding.ProjectID).Scan(&cursor)
		if read.Error != nil || read.RowsAffected != 1 || cursor.Block != height {
			return errors.New("V3 range halt cursor changed")
		}
		if cursor.State == "reorg_halted" {
			// Only acknowledge a halt of the SAME revision. A stale range
			// must not claim a later worker's halt as its own revocation.
			if cursor.Revision == revision+1 && revision != math.MaxUint64 {
				return nil
			}
			return errors.New("V3 range halt cursor revision changed")
		}
		if cursor.Revision != revision || revision == math.MaxUint64 || cursor.State != "running" {
			return errors.New("V3 range halt cursor state invalid")
		}
		if err := tx.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`, binding.ChainID, proxy, binding.ProjectID).Error; err != nil {
			return errors.New("V3 range halt projection revocation unavailable")
		}
		if err := suspendV3FinancialEffectsTx(tx, binding); err != nil {
			return err
		}
		if v3PolicyJournalEnabled() {
			if err := suspendV3PolicyFactsTx(tx, binding); err != nil {
				return err
			}
		}
		if err := suspendV3AppliedOutboxTx(tx, binding, nil); err != nil {
			return err
		}
		if err := haltV3BusinessStateTx(tx, binding); err != nil {
			return err
		}
		update := tx.Exec(`UPDATE ido_v3_scan_cursor SET state='reorg_halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND cursor_block=? AND revision=? AND state='running'`, binding.ChainID, proxy, binding.ProjectID, height, revision)
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("V3 range halt cursor fence changed")
		}
		return nil
	})
}

// haltBootstrapUnknown closes the publication window when a bootstrap has
// committed but its post-commit canonical read is unavailable. Unknown is NOT
// reorg proof: no journal/checkpoint is orphaned and no ancestor is selected.
// The exact first revision and cursor hash fence this against a concurrent
// scanner; only the independent recovery path may resume a halted cursor.
func (j v3Journal) haltBootstrapUnknown(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, height uint64, hash common.Hash) error {
	if j.db == nil || ctx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) || height == 0 || hash == (common.Hash{}) {
		return errors.New("V3 bootstrap halt scope invalid")
	}
	proxy, digest := v3Lower(binding.ProxyAddress), strings.ToLower(hash.Hex())
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Hash     string `gorm:"column:cursor_hash"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		read := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, binding.ChainID, proxy, binding.ProjectID).Scan(&cursor)
		if read.Error != nil || read.RowsAffected != 1 || cursor.Block != height || cursor.Hash != digest || (cursor.State != "running" && cursor.State != "reorg_halted") {
			return errors.New("V3 bootstrap halt cursor changed")
		}
		if cursor.State == "reorg_halted" {
			if cursor.Revision == 2 { // only the exact first publication can have been revoked
				return nil
			}
			return errors.New("V3 bootstrap halt revision changed")
		}
		if cursor.Revision != 1 {
			return errors.New("V3 bootstrap halt revision changed")
		}
		if err := tx.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`, binding.ChainID, proxy, binding.ProjectID).Error; err != nil {
			return errors.New("V3 bootstrap halt projection revocation unavailable")
		}
		if err := suspendV3FinancialEffectsTx(tx, binding); err != nil {
			return err
		}
		if v3PolicyJournalEnabled() {
			if err := suspendV3PolicyFactsTx(tx, binding); err != nil {
				return err
			}
		}
		if err := suspendV3AppliedOutboxTx(tx, binding, nil); err != nil {
			return err
		}
		if err := haltV3BusinessStateTx(tx, binding); err != nil {
			return err
		}
		update := tx.Exec(`UPDATE ido_v3_scan_cursor SET state='reorg_halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND cursor_block=? AND cursor_hash=? AND revision=1 AND state='running'`, binding.ChainID, proxy, binding.ProjectID, height, digest)
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("V3 bootstrap halt cursor fence changed")
		}
		return nil
	})
}

// haltMismatch revokes every published projection of the exact V3 project as
// soon as a persisted cursor fails canonicality. No ancestor is guessed and
// no orphan rows are changed until an authenticated rollback is reviewed.
func (j v3Journal) haltMismatch(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, height uint64, expectedHash common.Hash) error {
	if j.db == nil || ctx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) || height == 0 || expectedHash == (common.Hash{}) {
		return errors.New("V3 halt input invalid")
	}
	proxy := v3Lower(binding.ProxyAddress)
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Hash     string `gorm:"column:cursor_hash"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		read := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, binding.ChainID, proxy, binding.ProjectID).Scan(&cursor)
		if read.Error != nil || read.RowsAffected != 1 || cursor.Block != height || cursor.Hash != strings.ToLower(expectedHash.Hex()) || cursor.Revision == math.MaxUint64 || cursor.State != "running" {
			return errors.New("V3 halt cursor fence changed")
		}
		if err := tx.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`, binding.ChainID, proxy, binding.ProjectID).Error; err != nil {
			return errors.New("V3 halt projection revocation unavailable")
		}
		if err := suspendV3FinancialEffectsTx(tx, binding); err != nil {
			return err
		}
		if v3PolicyJournalEnabled() {
			if err := suspendV3PolicyFactsTx(tx, binding); err != nil {
				return err
			}
		}
		if err := suspendV3AppliedOutboxTx(tx, binding, nil); err != nil {
			return err
		}
		if err := haltV3BusinessStateTx(tx, binding); err != nil {
			return err
		}
		update := tx.Exec(`UPDATE ido_v3_scan_cursor SET state='reorg_halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND cursor_block=? AND cursor_hash=? AND revision=? AND state='running'`, binding.ChainID, proxy, binding.ProjectID, height, cursor.Hash, cursor.Revision)
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("V3 halt cursor update unavailable")
		}
		return nil
	})
}

type v3JournalRow struct {
	BlockNumber uint64 `gorm:"column:block_number"`
	BlockHash   string `gorm:"column:block_hash"`
	TxHash      string `gorm:"column:tx_hash"`
	LogIndex    uint   `gorm:"column:log_index"`
	EventKind   string `gorm:"column:event_kind"`
	Payload     string `gorm:"column:payload"`
}

func v3Lower(address common.Address) string { return strings.ToLower(address.Hex()) }

// readV3CanonicalCreationTx returns the single surviving creation fact for
// comparing immutable economics with a later pinned project observation.
// It is DB evidence only: it does not authenticate the original receipt.
func readV3CanonicalCreationTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, height uint64) (*commonchain.V3CreatedEvent, error) {
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) || height == 0 {
		return nil, errors.New("V3 creation journal scope invalid")
	}
	var rows []v3JournalRow
	read := tx.Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND event_kind='created' AND status='canonical' AND block_number<=? LIMIT 2`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, height).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return nil, errors.New("V3 canonical creation fact unavailable or ambiguous")
	}
	row := rows[0]
	var event commonchain.V3ReplayEvent
	if json.Unmarshal([]byte(row.Payload), &event) != nil || event.Created == nil || event.Purchase != nil || event.Lifecycle != nil || row.EventKind != "created" ||
		row.BlockNumber == 0 || row.BlockNumber > height || row.BlockHash != strings.ToLower(event.Created.BlockHash.Hex()) ||
		row.TxHash != strings.ToLower(event.Created.TxHash.Hex()) || row.LogIndex != event.Created.LogIndex ||
		event.Created.BlockNumber != row.BlockNumber || commonchain.MatchV3CreatedBinding(event.Created, binding) != nil {
		return nil, errors.New("V3 canonical creation payload mismatched")
	}
	var checkpoint struct {
		Hash string `gorm:"column:block_hash"`
	}
	read = tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND status='canonical'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, row.BlockNumber).Scan(&checkpoint)
	if read.Error != nil || read.RowsAffected != 1 || checkpoint.Hash != row.BlockHash {
		return nil, errors.New("V3 canonical creation checkpoint mismatched")
	}
	return event.Created, nil
}

// apply accepts only consecutive, block-complete batches. The caller's
// validation of canonical headers, successful receipts and runtime identity
// is indispensable. Failure leaves the cursor and every projection intact.
func (j v3Journal) apply(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, height uint64, parent, hash common.Hash, logs []types.Log) error {
	return j.applyBlock(ctx, binding, height, parent, hash, logs, nil)
}

// applyChecked additionally fences a complete block's rebuilt financial
// counters against an independently pinned getProject observation in the same
// DB transaction. The observation is NOT authenticated here: callers must
// establish trusted runtime/binding, confirmations and canonical receipts.
// Only the registered scanner can supply the verified observation. This is
// not a public wallet entitlement path.
func (j v3Journal) applyChecked(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, height uint64, parent, hash common.Hash, logs []types.Log, observed idov3whitelist.ProjectIdentityObservation) error {
	return j.applyBlock(ctx, binding, height, parent, hash, logs, &observed)
}

func (j v3Journal) applyBlock(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, height uint64, parent, hash common.Hash, logs []types.Log, observed *idov3whitelist.ProjectIdentityObservation) error {
	if j.db == nil || ctx == nil || binding.ChainID == 0 || binding.ProjectID == 0 ||
		binding.ProxyAddress == (common.Address{}) || binding.PaymentToken == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.ProjectToken == binding.PaymentToken ||
		height == 0 || parent == (common.Hash{}) || hash == (common.Hash{}) || parent == hash {
		return errors.New("V3 journal input invalid")
	}
	proxy := v3Lower(binding.ProxyAddress)
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Hash     string `gorm:"column:cursor_hash"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		v3TracePhase(ctx, "journal_cursor_lock")
		cursorRead := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, binding.ChainID, proxy, binding.ProjectID).Scan(&cursor)
		if cursorRead.Error != nil || cursorRead.RowsAffected != 1 {
			return errors.New("V3 journal cursor unavailable")
		}
		if cursor.Block == 0 || cursor.Block == math.MaxUint64 || cursor.Revision == math.MaxUint64 || cursor.State != "running" || cursor.Block+1 != height || cursor.Hash != strings.ToLower(parent.Hex()) {
			return errors.New("V3 journal not consecutive or halted")
		}
		// The cursor alone cannot attest the parent block. Even an empty parent
		// must have an exact canonical checkpoint, checked under the cursor lock
		// before any event or new checkpoint is inserted.
		var parentCheckpoint struct {
			Hash string `gorm:"column:block_hash"`
		}
		v3TracePhase(ctx, "journal_parent_checkpoint")
		parentRead := tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND status='canonical'`, binding.ChainID, proxy, binding.ProjectID, cursor.Block).Scan(&parentCheckpoint)
		if parentRead.Error != nil || parentRead.RowsAffected != 1 || parentCheckpoint.Hash != cursor.Hash {
			return errors.New("V3 journal parent checkpoint unavailable or mismatched")
		}
		// The prototype apply path has no authenticated fixed-block project
		// observation. It must not persist raw events that a later checked
		// block could silently promote without their financial effects.
		if observed == nil && len(logs) != 0 {
			return errors.New("V3 unverified event batch unavailable")
		}
		// An empty block still gets a checkpoint; adjacent hash is verified by
		// the caller's canonical header read, not inferred from event logs.
		v3TracePhase(ctx, "journal_event_insert")
		seen := make(map[uint]struct{}, len(logs))
		var last uint
		for _, log := range logs {
			if len(seen) != 0 && log.Index <= last {
				return errors.New("V3 journal logs out of order")
			}
			last = log.Index
			if log.BlockNumber != height || log.BlockHash != hash || log.TxHash == (common.Hash{}) || log.Address != binding.ProxyAddress {
				return errors.New("V3 journal log placement invalid")
			}
			if _, exists := seen[log.Index]; exists {
				return errors.New("V3 journal duplicate log index")
			}
			seen[log.Index] = struct{}{}
			event, err := commonchain.DecodeV3ReplayLog(log, binding)
			if err != nil {
				return err
			}
			payload, err := json.Marshal(event)
			if err != nil {
				return errors.New("V3 journal payload invalid")
			}
			kind := "created"
			if event.Purchase != nil {
				kind = "purchase"
			}
			if event.Lifecycle != nil {
				kind = event.Lifecycle.Kind
			}
			inserted := tx.Exec(`INSERT INTO ido_v3_event_journal (chain_id,proxy_address,project_id,contract_version,block_number,block_hash,tx_hash,log_index,event_kind,payload,status)
				VALUES (?,?,?,'ido_v3_stablecoin',?,?,?,?,?,?,'canonical')`, binding.ChainID, proxy, binding.ProjectID, height, strings.ToLower(hash.Hex()), strings.ToLower(log.TxHash.Hex()), log.Index, kind, string(payload))
			if inserted.Error != nil || inserted.RowsAffected != 1 {
				return errors.New("V3 journal insert unavailable")
			}
		}
		if err := tx.Exec(`INSERT INTO ido_v3_block_checkpoint (chain_id,proxy_address,project_id,contract_version,block_number,block_hash,status) VALUES (?,?,?,'ido_v3_stablecoin',?,?,'canonical')`, binding.ChainID, proxy, binding.ProjectID, height, strings.ToLower(hash.Hex())).Error; err != nil {
			return errors.New("V3 checkpoint insert unavailable")
		}
		if observed != nil && j.policyRPC != nil && v3PolicyJournalEnabled() {
			v3TracePhase(ctx, "journal_policy_live")
			if err := v3PolicyFactsAtBlockTx(ctx, tx, j.policyRPC, binding, j.policyRuntime, height, hash); err != nil {
				return err
			}
		}
		if observed != nil && v3PolicyJournalEnabled() {
			v3TracePhase(ctx, "journal_policy_apply")
			if err := applyV3PreparedPolicyFactsTx(tx, binding, j.preparedPolicy); err != nil {
				return err
			}
		}
		v3TracePhase(ctx, "journal_replay")
		if err := rebuildV3ProjectTx(tx, binding, height, hash, cursor.Revision+1); err != nil {
			return err
		}
		if observed != nil {
			v3TracePhase(ctx, "journal_creation_match")
			created, creationErr := readV3CanonicalCreationTx(tx, binding, height)
			if creationErr != nil || commonchain.MatchV3CreationProjectImmutable(created, binding, *observed) != nil {
				return errors.New("V3 verified projection lacks matching creation fact")
			}
			var projection struct {
				State  string `gorm:"column:state"`
				Raised string `gorm:"column:raised_raw"`
				Sold   string `gorm:"column:sold_raw"`
			}
			v3TracePhase(ctx, "journal_projection_match")
			read := tx.Raw(`SELECT state,raised_raw,sold_raw FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND revision=? AND cursor_block=? AND cursor_hash=?`, binding.ChainID, proxy, binding.ProjectID, cursor.Revision+1, height, strings.ToLower(hash.Hex())).Scan(&projection)
			if read.Error != nil || read.RowsAffected != 1 {
				return errors.New("V3 verified projection unavailable")
			}
			states := map[string]idov3whitelist.ProjectState{"pending": idov3whitelist.ProjectPending, "active": idov3whitelist.ProjectActive, "successful": idov3whitelist.ProjectSuccessful, "failed": idov3whitelist.ProjectFailed, "cancelled": idov3whitelist.ProjectCancelled}
			state, ok := states[projection.State]
			raised, raisedOK := new(big.Int).SetString(projection.Raised, 10)
			sold, soldOK := new(big.Int).SetString(projection.Sold, 10)
			if !ok || !raisedOK || !soldOK || commonchain.MatchV3ReplayProjectEndOfBlock(binding, commonchain.V3ReplayTotals{State: state, Raised: raised, Sold: sold}, *observed, height, hash) != nil {
				return errors.New("V3 verified projection disagrees with pinned project")
			}
			// Only the authenticated, complete-block path may materialize
			// finance. The unregistered journal.apply prototype remains inert.
			v3TracePhase(ctx, "journal_financial_apply")
			for _, log := range logs {
				event, err := commonchain.DecodeV3ReplayLog(log, binding)
				if err != nil || materializeV3FinancialEffectTx(tx, binding, log, event) != nil {
					return errors.New("V3 verified financial effect unavailable")
				}
			}
			v3TracePhase(ctx, "journal_business_apply")
			if err := applyV3BusinessStateTx(tx, binding, projection.State); err != nil {
				return err
			}
			v3TracePhase(ctx, "journal_outbox_apply")
			for _, log := range logs {
				event, err := commonchain.DecodeV3ReplayLog(log, binding)
				if err != nil {
					return errors.New("V3 canonical outbox event invalid")
				}
				action := ""
				switch {
				case event.Created != nil:
					action = "ido_v3_create"
				case event.Lifecycle != nil:
					switch event.Lifecycle.Kind {
					case "ProjectActivated":
						action = "ido_v3_activate"
					case "ProjectEnded":
						action = "ido_v3_end"
					case "PlatformFeeWithdrawn":
						action = "ido_v3_withdraw_platform_fee"
					}
				}
				if action == "" {
					continue
				}
				if action == "ido_v3_create" && !j.requireCreateAttempt {
					continue // only the signed creation bridge binds the outbox
				}
				var ids []struct {
					ID uint64 `gorm:"column:id"`
				}
				if read := tx.Raw(`SELECT id FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND project_id=?
					AND contract_version='ido_v3_stablecoin' AND block_number=? AND block_hash=? AND tx_hash=?
					AND log_index=? AND status='canonical' LIMIT 2`, binding.ChainID, proxy, binding.ProjectID, height,
					strings.ToLower(hash.Hex()), strings.ToLower(log.TxHash.Hex()), log.Index).Scan(&ids); read.Error != nil || len(ids) != 1 ||
					chaintx.ApplyV3CanonicalEventTx(tx, binding, action, log.TxHash, hash, height, ids[0].ID, action == "ido_v3_create") != nil {
					return errors.New("V3 canonical outbox application unavailable")
				}
			}
		}
		v3TracePhase(ctx, "journal_cursor_update")
		update := tx.Exec(`UPDATE ido_v3_scan_cursor SET cursor_block=?,cursor_hash=?,revision=revision+1 WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND cursor_block=? AND cursor_hash=? AND revision=? AND state='running'`, height, strings.ToLower(hash.Hex()), binding.ChainID, proxy, binding.ProjectID, cursor.Block, cursor.Hash, cursor.Revision)
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("V3 cursor fence changed")
		}
		return nil
	})
}

// rollback marks old facts orphaned, revokes the projection in the SAME
// transaction, and fences the cursor. It deliberately halts: continuing to
// serve wallet totals or scanning a replacement fork needs fresh admission,
// receipt checks and an explicit recovery decision. Old rows stay auditable.
// A caller must establish the ancestor's canonical hash *on chain* before
// calling rollback; a stored hash alone cannot prove it is still canonical.
// expectedRevision only fences concurrent DB changes; it is not a recovery
// credential or a substitute for authenticated ancestor/source selection.
func (j v3Journal) rollback(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, ancestor uint64, hash common.Hash, expectedRevision uint64) error {
	if j.db == nil || ctx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) || ancestor == 0 || hash == (common.Hash{}) || expectedRevision == 0 {
		return errors.New("V3 rollback input invalid")
	}
	proxy := v3Lower(binding.ProxyAddress)
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Hash     string `gorm:"column:cursor_hash"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		cursorRead := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, binding.ChainID, proxy, binding.ProjectID).Scan(&cursor)
		if cursorRead.Error != nil || cursorRead.RowsAffected != 1 || cursor.Block == 0 || cursor.Revision == math.MaxUint64 || cursor.Revision != expectedRevision || cursor.State != "reorg_halted" || ancestor >= cursor.Block {
			return errors.New("V3 rollback cursor unavailable or invalid")
		}
		var checkpoint struct {
			Hash string `gorm:"column:block_hash"`
		}
		checkpointRead := tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number=? AND status='canonical'`, binding.ChainID, proxy, binding.ProjectID, ancestor).Scan(&checkpoint)
		if checkpointRead.Error != nil || checkpointRead.RowsAffected != 1 || checkpoint.Hash != strings.ToLower(hash.Hex()) {
			return errors.New("V3 rollback ancestor not recorded")
		}
		if err := tx.Exec(`UPDATE ido_v3_financial_effect SET status='orphaned' WHERE chain_id=? AND proxy_address=? AND project_id=?
			AND contract_version='ido_v3_stablecoin' AND block_number>? AND status IN ('canonical','suspended')`,
			binding.ChainID, proxy, binding.ProjectID, ancestor).Error; err != nil {
			return errors.New("V3 rollback financial effect unavailable")
		}
		if v3PolicyJournalEnabled() {
			if err := orphanV3PolicyFactsTx(tx, binding, ancestor); err != nil {
				return err
			}
		}
		if err := suspendV3AppliedOutboxTx(tx, binding, &ancestor); err != nil {
			return err
		}
		for _, table := range []string{"ido_v3_event_journal", "ido_v3_block_checkpoint"} {
			if err := tx.Exec(`UPDATE `+table+` SET status='orphaned' WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number>? AND status='canonical'`, binding.ChainID, proxy, binding.ProjectID, ancestor).Error; err != nil {
				return errors.New("V3 rollback orphan update unavailable")
			}
		}
		if err := tx.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=?`, binding.ChainID, proxy, binding.ProjectID).Error; err != nil {
			return errors.New("V3 rollback entitlement revocation unavailable")
		}
		if err := haltV3BusinessStateTx(tx, binding); err != nil {
			return err
		}
		result := tx.Exec(`UPDATE ido_v3_scan_cursor SET cursor_block=?,cursor_hash=?,revision=revision+1,state='reorg_halted' WHERE chain_id=? AND proxy_address=? AND project_id=? AND cursor_block=? AND cursor_hash=? AND revision=? AND state='reorg_halted'`, ancestor, strings.ToLower(hash.Hex()), binding.ChainID, proxy, binding.ProjectID, cursor.Block, cursor.Hash, expectedRevision)
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("V3 rollback cursor fence changed")
		}
		return nil
	})
}

// resumeAtAncestor rebuilds the surviving prefix before restarting a halted
// cursor. It does not prove the supplied ancestor canonical: the caller must
// recheck the exact chain/header/runtime identity and receipts beforehand.
// The expected revision fences any cursor mutation since that separate read;
// this does NOT make an old read-only preflight an authorization to publish.
// A replacement fork is consumed one verified block at a time by apply.
func (j v3Journal) resumeAtAncestor(ctx context.Context, binding idov3whitelist.ExpectedProjectBinding, ancestor uint64, hash common.Hash, expectedRevision uint64) error {
	if j.db == nil || ctx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) || binding.ProjectToken == binding.PaymentToken ||
		ancestor == 0 || hash == (common.Hash{}) || expectedRevision == 0 {
		return errors.New("V3 resume input invalid")
	}
	proxy := v3Lower(binding.ProxyAddress)
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor struct {
			Block    uint64 `gorm:"column:cursor_block"`
			Hash     string `gorm:"column:cursor_hash"`
			Revision uint64 `gorm:"column:revision"`
			State    string `gorm:"column:state"`
		}
		read := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' FOR UPDATE`, binding.ChainID, proxy, binding.ProjectID).Scan(&cursor)
		if read.Error != nil || read.RowsAffected != 1 || cursor.State != "reorg_halted" || cursor.Block != ancestor || cursor.Hash != strings.ToLower(hash.Hex()) || cursor.Revision == math.MaxUint64 || cursor.Revision != expectedRevision {
			return errors.New("V3 resume cursor not halted at ancestor")
		}
		var checkpoint struct {
			Hash string `gorm:"column:block_hash"`
		}
		read = tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND block_number=? AND status='canonical'`, binding.ChainID, proxy, binding.ProjectID, ancestor).Scan(&checkpoint)
		if read.Error != nil || read.RowsAffected != 1 || checkpoint.Hash != cursor.Hash {
			return errors.New("V3 resume ancestor checkpoint unavailable")
		}
		if err := rebuildV3ProjectTx(tx, binding, ancestor, hash, cursor.Revision+1); err != nil {
			return err
		}
		var projection struct {
			State string `gorm:"column:state"`
		}
		if read := tx.Raw(`SELECT state FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=?
			AND contract_version='ido_v3_stablecoin' AND cursor_block=? AND cursor_hash=? AND revision=?`,
			binding.ChainID, proxy, binding.ProjectID, ancestor, cursor.Hash, cursor.Revision+1).Scan(&projection); read.Error != nil || read.RowsAffected != 1 {
			return errors.New("V3 resume business projection unavailable")
		}
		if err := resumeV3BusinessStateTx(tx, binding, projection.State); err != nil {
			return err
		}
		if err := resumeV3FinancialEffectsTx(tx, binding, ancestor); err != nil {
			return err
		}
		if v3PolicyJournalEnabled() {
			if err := resumeV3PolicyFactsTx(tx, binding, ancestor); err != nil {
				return err
			}
		}
		if err := resumeV3AppliedOutboxTx(tx, binding, ancestor); err != nil {
			return err
		}
		update := tx.Exec(`UPDATE ido_v3_scan_cursor SET revision=revision+1,state='running' WHERE chain_id=? AND proxy_address=? AND project_id=? AND cursor_block=? AND cursor_hash=? AND revision=? AND state='reorg_halted'`, binding.ChainID, proxy, binding.ProjectID, ancestor, cursor.Hash, expectedRevision)
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("V3 resume cursor fence changed")
		}
		return nil
	})
}

// readV3PublishedProjection is an internal, consistency-gated lookup. It is
// intentionally NOT a Core wallet API: scanner admission and ongoing chain
// reconciliation are absent. A halted cursor or stale projection is never a
// balance, even if its JSON remains on disk after an interrupted operation.
func readV3PublishedProjection(ctx context.Context, db *gorm.DB, binding idov3whitelist.ExpectedProjectBinding) (string, error) {
	if ctx == nil || db == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) || binding.PaymentToken == binding.ProjectToken {
		return "", errors.New("V3 projection read scope invalid")
	}
	var row struct {
		Wallets string `gorm:"column:wallets"`
		State   string `gorm:"column:state"`
		Status  string `gorm:"column:status"`
	}
	read := db.WithContext(ctx).Raw(`SELECT p.wallets,p.state,project.status FROM ido_v3_project_projection p
		JOIN ido_v3_scan_cursor c ON c.chain_id=p.chain_id AND c.proxy_address=p.proxy_address AND c.project_id=p.project_id AND c.contract_version=p.contract_version
		JOIN ido_v3_block_checkpoint b ON b.chain_id=c.chain_id AND b.proxy_address=c.proxy_address AND b.project_id=c.project_id AND b.contract_version=c.contract_version AND b.block_number=c.cursor_block AND b.block_hash=c.cursor_hash AND b.status='canonical'
		JOIN ido_v3_project_binding binding ON binding.chain_id=p.chain_id AND binding.proxy_address=p.proxy_address AND binding.project_id=p.project_id AND binding.contract_version=p.contract_version
		JOIN projects project ON project.project_id=binding.project_id AND project.chain_id=binding.chain_id
			AND BINARY project.contract_address=BINARY binding.proxy_address AND BINARY project.contract_version=BINARY binding.contract_version
		WHERE p.chain_id=? AND p.proxy_address=? AND p.project_id=? AND p.contract_version='ido_v3_stablecoin'
		AND BINARY project.funding_mode=BINARY 'capped_presale' AND project.onchain_locked=1
		AND BINARY project.token_address=BINARY ? AND BINARY project.payment_token_address=BINARY ?
		AND c.state='running' AND p.revision=c.revision AND p.cursor_block=c.cursor_block AND p.cursor_hash=c.cursor_hash`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID,
		v3Lower(binding.ProjectToken), v3Lower(binding.PaymentToken)).Scan(&row)
	status, err := v3BusinessStatus(row.State)
	if read.Error != nil || read.RowsAffected != 1 || err != nil || row.Status != status || !json.Valid([]byte(row.Wallets)) {
		return "", errors.New("V3 projection stale, halted or unavailable")
	}
	return row.Wallets, nil
}

func rebuildV3ProjectTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, height uint64, hash common.Hash, revision uint64) error {
	return replayV3ProjectTx(tx, binding, height, hash, revision, nil)
}

// A read-only parity check must not UPSERT an unchanged projection merely
// to validate it. Keep the same complete journal/checkpoint validation.
func replayV3ProjectTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, height uint64, hash common.Hash, revision uint64, expected *v3ParityProjection) error {
	var rows []v3JournalRow
	if err := tx.Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND status='canonical'
		ORDER BY block_number,log_index`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&rows).Error; err != nil {
		return errors.New("V3 journal read unavailable")
	}
	if len(rows) == 0 {
		if expected != nil {
			return errors.New("V3 projection lacks journal")
		}
		if err := tx.Exec(`DELETE FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=?`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Error; err != nil {
			return errors.New("V3 projection reset unavailable")
		}
		return nil
	}
	events := make([]commonchain.V3ReplayEvent, 0, len(rows))
	// A payload with an internally consistent block hash still is not a
	// canonical journal fact. Require the project-scoped checkpoint that was
	// committed with each block, including when rebuilding a halted prefix.
	// Do not silently discard a corrupted row via an INNER JOIN.
	checkedBlocks := make(map[uint64]string)
	if expected != nil {
		// A LEFT JOIN preserves missing/mismatched checkpoint evidence rather
		// than silently filtering corrupt journal facts out of the replay.
		var anchors []struct {
			BlockNumber               uint64
			BlockHash, CheckpointHash string
		}
		r := tx.Raw(`SELECT DISTINCT j.block_number,j.block_hash,COALESCE(c.block_hash,'') AS checkpoint_hash FROM ido_v3_event_journal j LEFT JOIN ido_v3_block_checkpoint c ON c.chain_id=j.chain_id AND c.proxy_address=j.proxy_address AND c.project_id=j.project_id AND c.contract_version=j.contract_version AND c.block_number=j.block_number AND c.status='canonical' WHERE j.chain_id=? AND j.proxy_address=? AND j.project_id=? AND j.contract_version='ido_v3_stablecoin' AND j.status='canonical'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&anchors)
		if r.Error != nil {
			return errors.New("V3 journal checkpoint batch unavailable")
		}
		for _, a := range anchors {
			if a.CheckpointHash == "" || a.CheckpointHash != a.BlockHash {
				return errors.New("V3 journal canonical checkpoint mismatch")
			}
			if hash, ok := checkedBlocks[a.BlockNumber]; ok && hash != a.BlockHash {
				return errors.New("V3 journal block hash split")
			}
			checkedBlocks[a.BlockNumber] = a.BlockHash
		}
	}
	for _, row := range rows {
		if row.BlockNumber == 0 || row.BlockNumber > height || row.BlockHash == "" ||
			(row.BlockNumber == height && row.BlockHash != strings.ToLower(hash.Hex())) {
			return errors.New("V3 journal block outside cursor or mismatched")
		}
		if previous, found := checkedBlocks[row.BlockNumber]; found {
			if previous != row.BlockHash {
				return errors.New("V3 journal block hash split")
			}
		} else {
			var checkpoint struct {
				Hash string `gorm:"column:block_hash"`
			}
			read := tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND status='canonical'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, row.BlockNumber).Scan(&checkpoint)
			if read.Error != nil || read.RowsAffected != 1 || checkpoint.Hash != row.BlockHash {
				return errors.New("V3 journal canonical checkpoint mismatch")
			}
			checkedBlocks[row.BlockNumber] = row.BlockHash
		}
		var entry commonchain.V3ReplayEvent
		if json.Unmarshal([]byte(row.Payload), &entry) != nil {
			return errors.New("V3 journal payload corrupt")
		}
		kind := "created"
		if entry.Purchase != nil {
			kind = "purchase"
		} else if entry.Lifecycle != nil {
			kind = entry.Lifecycle.Kind
		}
		if row.EventKind != kind || boolIntV3(entry.Created != nil)+boolIntV3(entry.Purchase != nil)+boolIntV3(entry.Lifecycle != nil) != 1 {
			return errors.New("V3 journal event kind disagrees with payload")
		}
		var block uint64
		var logIndex uint
		var txHash, blockHash common.Hash
		switch {
		case entry.Created != nil:
			block, logIndex, txHash, blockHash = entry.Created.BlockNumber, entry.Created.LogIndex, entry.Created.TxHash, entry.Created.BlockHash
		case entry.Purchase != nil:
			block, logIndex, txHash, blockHash = entry.Purchase.BlockNumber, entry.Purchase.LogIndex, entry.Purchase.TxHash, entry.Purchase.BlockHash
		case entry.Lifecycle != nil:
			block, logIndex, txHash, blockHash = entry.Lifecycle.BlockNumber, entry.Lifecycle.LogIndex, entry.Lifecycle.TxHash, entry.Lifecycle.BlockHash
		default:
			return errors.New("V3 journal payload kind invalid")
		}
		if block != row.BlockNumber || logIndex != row.LogIndex || !strings.EqualFold(txHash.Hex(), row.TxHash) || !strings.EqualFold(blockHash.Hex(), row.BlockHash) {
			return errors.New("V3 journal payload identity mismatch")
		}
		events = append(events, entry)
	}
	totals, err := commonchain.RebuildV3Replay(binding, events)
	if err != nil {
		return fmt.Errorf("V3 projection invariant: %w", err)
	}
	wallets := make(map[string]map[string]string, len(totals.Wallets))
	for address, wallet := range totals.Wallets {
		wallets[v3Lower(address)] = map[string]string{"payment": wallet.Payment.String(), "tokens": wallet.Tokens.String(), "refunded": wallet.Refunded.String(), "claimed": wallet.Claimed.String()}
	}
	walletPayload, err := json.Marshal(wallets)
	if err != nil {
		return errors.New("V3 wallet projection invalid")
	}
	states := []string{"pending", "active", "successful", "failed", "cancelled"}
	if int(totals.State) >= len(states) {
		return errors.New("V3 state projection invalid")
	}
	if expected != nil {
		actual := v3ParityProjection{State: states[totals.State], RaisedRaw: totals.Raised.String(), SoldRaw: totals.Sold.String(), DepositedRaw: totals.Deposited.String(), RefundedRaw: totals.Refunded.String(), ClaimedRaw: totals.Claimed.String(), Wallets: string(walletPayload), ProceedsPaidRaw: totals.ProceedsPaid.String(), FeePaidRaw: totals.FeePaid.String(), ProjectTokensWithdrawnRaw: totals.ProjectTokensWithdrawn.String()}
		if !v3ProjectionEqual(actual, *expected) {
			return errors.New("V3 projection parity mismatch")
		}
		return nil
	}
	result := tx.Exec(`INSERT INTO ido_v3_project_projection (chain_id,proxy_address,project_id,contract_version,revision,cursor_block,cursor_hash,state,raised_raw,sold_raw,deposited_raw,refunded_raw,claimed_raw,wallets,proceeds_paid_raw,fee_paid_raw,project_tokens_withdrawn_raw)
		VALUES (?,?,?,'ido_v3_stablecoin',?,?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE revision=VALUES(revision),cursor_block=VALUES(cursor_block),cursor_hash=VALUES(cursor_hash),state=VALUES(state),raised_raw=VALUES(raised_raw),sold_raw=VALUES(sold_raw),deposited_raw=VALUES(deposited_raw),refunded_raw=VALUES(refunded_raw),claimed_raw=VALUES(claimed_raw),wallets=VALUES(wallets),proceeds_paid_raw=VALUES(proceeds_paid_raw),fee_paid_raw=VALUES(fee_paid_raw),project_tokens_withdrawn_raw=VALUES(project_tokens_withdrawn_raw)`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, revision, height, strings.ToLower(hash.Hex()), states[totals.State], totals.Raised.String(), totals.Sold.String(), totals.Deposited.String(), totals.Refunded.String(), totals.Claimed.String(), string(walletPayload), totals.ProceedsPaid.String(), totals.FeePaid.String(), totals.ProjectTokensWithdrawn.String())
	if result.Error != nil {
		return errors.New("V3 projection write unavailable")
	}
	return nil
}

func boolIntV3(value bool) int {
	if value {
		return 1
	}
	return 0
}
