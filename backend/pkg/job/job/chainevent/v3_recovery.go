package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

// v3CheckSurvivingPrefix is an unregistered, bounded READ-ONLY recovery
// preflight. It verifies the entire surviving project-scoped journal prefix
// against fresh block headers, complete proxy filters and successful receipts.
// It does not approve the caller's runtime/binding, select an ancestor,
// rollback, resume a cursor, or publish financial positions. A later reorg
// invalidates this transient result; a future recovery coordinator must fence
// its writes separately and recheck chain canonicality after publication.
func v3CheckSurvivingPrefix(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	creationBlock, ancestor, minConfirmations uint64, expectedAncestor common.Hash) error {
	const maxBlocks = 512
	if ctx == nil || db == nil || rpc == nil || creationBlock < 2 || ancestor < creationBlock ||
		ancestor-creationBlock >= maxBlocks || minConfirmations == 0 || expectedAncestor == (common.Hash{}) ||
		binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) || binding.PaymentToken == binding.ProjectToken {
		return errors.New("V3 recovery prefix scope invalid")
	}
	var checkpoints []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}
	read := db.WithContext(ctx).Raw(`SELECT block_number,block_hash FROM ido_v3_block_checkpoint
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, creationBlock-1, ancestor).Scan(&checkpoints)
	if read.Error != nil || len(checkpoints) != int(ancestor-creationBlock+2) {
		return errors.New("V3 recovery prefix checkpoints unavailable")
	}
	var rows []v3JournalRow
	read = db.WithContext(ctx).Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number,log_index`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, creationBlock, ancestor).Scan(&rows)
	if read.Error != nil || len(rows) == 0 {
		return errors.New("V3 recovery prefix journal unavailable")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		return errors.New("V3 recovery prefix chain mismatched")
	}
	var prior common.Hash
	var creation *commonchain.V3CreatedEvent
	position := 0
	created := false
	var replay []commonchain.V3ReplayEvent
	// Retain independently owned snapshots, not provider-owned result buffers.
	// The final pass revisits earlier blocks after the whole prefix has been
	// audited; endpoint headers alone do not fence mutable filter/receipt APIs.
	type blockSnapshot struct {
		height   uint64
		hash     common.Hash
		observed idov3whitelist.ProjectIdentityObservation
		complete []types.Log
		selected []types.Log
	}
	var snapshots []blockSnapshot
	for index, checkpoint := range checkpoints {
		height := creationBlock - 1 + uint64(index)
		if checkpoint.Number != height || !v3CanonicalRecoveryHash.MatchString(checkpoint.Hash) || common.HexToHash(checkpoint.Hash) == (common.Hash{}) {
			return errors.New("V3 recovery prefix checkpoint malformed")
		}
		header, headerErr := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
		if headerErr != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height || v3LowerHashHeader(header) != checkpoint.Hash ||
			(index > 0 && header.ParentHash != prior) {
			return errors.New("V3 recovery prefix header mismatched")
		}
		prior = header.Hash()
		if index == 0 {
			continue
		}
		var complete []types.Log
		parent, hash, logs, scanErr := v3ScanBlock(ctx, rpc, binding, runtime, height, minConfirmations, &complete)
		if scanErr != nil || parent != common.HexToHash(checkpoints[index-1].Hash) || hash != prior {
			return errors.New("V3 recovery prefix receipt or filter unavailable")
		}
		scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(binding.ChainID), ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
		observed, observedErr := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, height, hash)
		if observedErr != nil || idov3whitelist.MatchProjectIdentityObservation(binding, observed) != nil ||
			(creation != nil && commonchain.MatchV3CreationProjectImmutable(creation, binding, observed) != nil) {
			return errors.New("V3 recovery prefix project changed")
		}
		if err := v3RecheckCompleteFilter(ctx, rpc, binding, height, complete); err != nil {
			return err
		}
		snapshots = append(snapshots, blockSnapshot{height: height, hash: hash, observed: observed, complete: complete, selected: v3CopyLogs(logs)})
		for _, log := range logs {
			// The project read follows the scanner's two receipt passes. Treat a
			// changed third answer as unknown rather than accepting the stored row.
			receipt, receiptErr := rpc.TransactionReceipt(ctx, log.TxHash)
			if receiptErr != nil || receipt == nil {
				return errors.New("V3 recovery prefix receipt changed")
			}
			if position >= len(rows) {
				return errors.New("V3 recovery prefix journal missing event")
			}
			row := rows[position]
			if row.BlockNumber != height || row.BlockHash != checkpoint.Hash || row.TxHash != strings.ToLower(log.TxHash.Hex()) || row.LogIndex != log.Index {
				return errors.New("V3 recovery prefix journal log mismatched")
			}
			actual, decodeErr := commonchain.DecodeV3ReplayLog(log, binding)
			var matchErr error
			if decodeErr == nil {
				switch {
				case actual.Created != nil:
					matchErr = commonchain.MatchV3CreatedReceipt(actual.Created, binding, receipt)
				case actual.Purchase != nil:
					matchErr = commonchain.MatchV3PurchaseReceipt(actual.Purchase, commonchain.V3PurchaseBinding{
						ChainID: binding.ChainID, ProxyAddress: binding.ProxyAddress, ProjectID: binding.ProjectID,
						PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
					}, receipt)
				case actual.Lifecycle != nil:
					matchErr = commonchain.MatchV3LifecycleReceipt(actual.Lifecycle, binding, receipt)
				default:
					return errors.New("V3 recovery prefix event unsupported")
				}
			}
			var persisted commonchain.V3ReplayEvent
			if decodeErr != nil || matchErr != nil || json.Unmarshal([]byte(row.Payload), &persisted) != nil || !reflect.DeepEqual(actual, persisted) {
				return errors.New("V3 recovery prefix payload mismatched")
			}
			kind := "created"
			if actual.Purchase != nil {
				kind = "purchase"
			}
			if actual.Lifecycle != nil {
				kind = actual.Lifecycle.Kind
			}
			if kind != row.EventKind || (actual.Created != nil && (created || position != 0 || height != creationBlock)) ||
				(actual.Created == nil && !created) {
				return errors.New("V3 recovery prefix event order invalid")
			}
			if actual.Created != nil {
				if commonchain.MatchV3CreationProjectAtBlock(actual.Created, binding, observed) != nil {
					return errors.New("V3 recovery prefix creation economics changed")
				}
				created = true
				creation = actual.Created
			}
			replay = append(replay, actual)
			position++
		}
		if !created {
			return errors.New("V3 recovery prefix creation missing")
		}
		totals, replayErr := commonchain.RebuildV3Replay(binding, replay)
		if replayErr != nil || commonchain.MatchV3ReplayProjectEndOfBlock(binding, totals, observed, height, hash) != nil {
			return errors.New("V3 recovery prefix counters disagreed with chain")
		}
	}
	if !created || position != len(rows) || prior != expectedAncestor {
		return errors.New("V3 recovery prefix incomplete")
	}
	// A DB row replaced while the RPC scan was in progress must not be
	// interpreted as the prefix we just checked. This comparison alone does
	// not lock the rows or certify a future rollback; it is a read-only fence.
	var finalCheckpoints []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}
	read = db.WithContext(ctx).Raw(`SELECT block_number,block_hash FROM ido_v3_block_checkpoint
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, creationBlock-1, ancestor).Scan(&finalCheckpoints)
	if read.Error != nil || !reflect.DeepEqual(checkpoints, finalCheckpoints) {
		return errors.New("V3 recovery prefix checkpoints changed")
	}
	var finalRows []v3JournalRow
	read = db.WithContext(ctx).Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number,log_index`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, creationBlock, ancestor).Scan(&finalRows)
	if read.Error != nil || !reflect.DeepEqual(rows, finalRows) {
		return errors.New("V3 recovery prefix journal changed")
	}
	// The scan can take time. Re-read EVERY header, including empty interior
	// blocks: matching endpoints do not establish a stable interior when an
	// RPC provider changes its answer mid-scan. This is still not a finality
	// certificate or authority for a DB mutation.
	if err := v3RecheckPrefixHeaders(ctx, rpc, creationBlock-1, checkpoints); err != nil {
		return err
	}
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(latest, ancestor, minConfirmations) {
		return errors.New("V3 recovery prefix confirmations changed")
	}
	end, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(ancestor))
	if err != nil || end == nil || end.Number == nil || !end.Number.IsUint64() || end.Number.Uint64() != ancestor || end.Hash() != expectedAncestor {
		return errors.New("V3 recovery prefix ancestor changed")
	}
	first, firstErr := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(creationBlock-1))
	if firstErr != nil || first == nil || first.Number == nil || !first.Number.IsUint64() || first.Number.Uint64() != creationBlock-1 || v3LowerHashHeader(first) != checkpoints[0].Hash {
		return errors.New("V3 recovery prefix starting anchor changed")
	}
	for _, snapshot := range snapshots {
		scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(binding.ChainID), ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
		again, readErr := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, snapshot.height, snapshot.hash)
		if readErr != nil || again != snapshot.observed {
			return errors.New("V3 recovery prefix final project changed")
		}
		if err := v3RecheckCompleteFilter(ctx, rpc, binding, snapshot.height, snapshot.complete); err != nil {
			return err
		}
		for _, log := range snapshot.selected {
			entry, decodeErr := commonchain.DecodeV3ReplayLog(log, binding)
			if decodeErr != nil {
				return errors.New("V3 recovery prefix final event invalid")
			}
			receipt, readErr := rpc.TransactionReceipt(ctx, log.TxHash)
			if readErr != nil || receipt == nil {
				return errors.New("V3 recovery prefix final receipt unavailable")
			}
			var matchErr error
			switch {
			case entry.Created != nil:
				matchErr = commonchain.MatchV3CreatedReceipt(entry.Created, binding, receipt)
			case entry.Purchase != nil:
				matchErr = commonchain.MatchV3PurchaseReceipt(entry.Purchase, commonchain.V3PurchaseBinding{
					ChainID: binding.ChainID, ProxyAddress: binding.ProxyAddress, ProjectID: binding.ProjectID,
					PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
				}, receipt)
			case entry.Lifecycle != nil:
				matchErr = commonchain.MatchV3LifecycleReceipt(entry.Lifecycle, binding, receipt)
			default:
				return errors.New("V3 recovery prefix final event unsupported")
			}
			if matchErr != nil {
				return errors.New("V3 recovery prefix final receipt changed")
			}
		}
	}
	// The final RPC pass above can take long enough for an earlier checkpoint
	// or journal payload to change. Re-read the entire bounded database prefix
	// once more after that pass; never return a reusable recovery certificate.
	if err := v3RecheckPrefixRows(ctx, db, binding, creationBlock, ancestor, checkpoints, rows); err != nil {
		return err
	}
	if err := v3RecheckPrefixHeaders(ctx, rpc, creationBlock-1, checkpoints); err != nil {
		return err
	}
	return nil
}

func v3RecheckPrefixRows(ctx context.Context, db *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	creationBlock, ancestor uint64, checkpoints []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}, rows []v3JournalRow) error {
	if ctx == nil || db == nil || creationBlock < 2 || ancestor < creationBlock || ancestor-creationBlock >= 512 ||
		binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) {
		return errors.New("V3 recovery prefix final database scope invalid")
	}
	var againCheckpoints []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}
	read := db.WithContext(ctx).Raw(`SELECT block_number,block_hash FROM ido_v3_block_checkpoint
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, creationBlock-1, ancestor).Scan(&againCheckpoints)
	if read.Error != nil || !reflect.DeepEqual(checkpoints, againCheckpoints) {
		return errors.New("V3 recovery prefix final checkpoints changed")
	}
	var againRows []v3JournalRow
	read = db.WithContext(ctx).Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number,log_index`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, creationBlock, ancestor).Scan(&againRows)
	if read.Error != nil || !reflect.DeepEqual(rows, againRows) {
		return errors.New("V3 recovery prefix final journal changed")
	}
	return nil
}

var v3CanonicalRecoveryHash = regexp.MustCompile(`^0x[0-9a-f]{64}$`)

// v3RecheckPrefixHeaders validates a bounded, contiguous historical header
// sequence against the original DB snapshot. No result escapes as a reusable
// recovery credential; a future writer must lock and independently recheck.
func v3RecheckPrefixHeaders(ctx context.Context, rpc v3ScannerRPC, start uint64, checkpoints []struct {
	Number uint64 `gorm:"column:block_number"`
	Hash   string `gorm:"column:block_hash"`
}) error {
	if ctx == nil || rpc == nil || start == 0 || len(checkpoints) == 0 || len(checkpoints) > 513 {
		return errors.New("V3 recovery prefix header scope invalid")
	}
	var previous common.Hash
	for i, checkpoint := range checkpoints {
		height := start + uint64(i)
		if height < start || checkpoint.Number != height || !v3CanonicalRecoveryHash.MatchString(checkpoint.Hash) || common.HexToHash(checkpoint.Hash) == (common.Hash{}) {
			return errors.New("V3 recovery prefix header scope invalid")
		}
		header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
		if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height || v3LowerHashHeader(header) != checkpoint.Hash || (i > 0 && header.ParentHash != previous) {
			return errors.New("V3 recovery prefix intermediate header changed")
		}
		previous = header.Hash()
	}
	return nil
}
