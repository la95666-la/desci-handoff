package chainevent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/projecttracking"
)

type trackingFenceKey struct{}
type closureTupleRPC struct {
	v3ScannerRPC
	tuple   []byte
	project uint64
	proxy   common.Address
}

func (r *closureTupleRPC) CallContract(ctx context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	value, err := r.v3ScannerRPC.CallContract(ctx, m, n)
	if err == nil && m.To != nil && *m.To == r.proxy && len(m.Data) == 36 && bytes.Equal(m.Data[:4], crypto.Keccak256([]byte("getProject(uint256)"))[:4]) && new(big.Int).SetBytes(m.Data[4:]).Uint64() == r.project {
		r.tuple = append([]byte(nil), value...)
	}
	return value, err
}

type trackingFence struct {
	revision uint64
	mode     string
}

// A mode change while RPC preparation runs must fence the stale writer. This
// prevents a just-archived project being processed by an earlier queue item.
func checkV3TrackingFence(ctx context.Context, tx *gorm.DB, id uint64) error {
	f, ok := ctx.Value(trackingFenceKey{}).(trackingFence)
	if !ok {
		return nil
	}
	row, err := projecttracking.Read(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if row.Revision != f.revision || row.Mode != f.mode {
		return &v3ScanDeferred{stage: "tracking_mode_changed"}
	}
	return nil
}

func selectV3TrackingCandidates(ctx context.Context, db *gorm.DB, ids []uint64) ([]uint64, error) {
	if !projecttracking.Enabled() {
		return ids, nil
	}
	if err := projecttracking.SchemaReady(ctx, db); err != nil {
		return nil, err
	}
	var out []uint64
	for _, id := range ids {
		row, err := projecttracking.Read(ctx, db, id, false)
		if err != nil {
			return nil, err
		}
		if skipV3TrackingTurn(row) {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

func skipV3TrackingTurn(row projecttracking.Row) bool {
	if row.Urgent || row.RecoveryRequired {
		return false
	}
	if row.Mode == "archived" {
		return row.Finished && row.EvidenceCurrent && row.WakeBlock == 0
	}
	// Maintenance deliberately permits stale/missing closure evidence and an
	// acknowledged historical wake. New events set next_scan_at=NOW in the
	// producer transaction; explicit sync does the same. Once due, every batch
	// stays due until the existing fully-caught-up closure writer schedules the
	// next 12h interval. Partial progress/failure must never postpone backlog.
	return row.Mode == "maintenance" && !row.Due
}

// Atomic with new public coverage: bounded, persistent wake hints cannot be
// lost on Job restart. Known project events promote only their own project;
// governance/control events wake all non-normal projects for independent audit.
func wakeV3TrackingFromCoverageTx(tx *gorm.DB, s v3StreamScope, blocks []v3StreamBlock) error {
	if !projecttracking.Enabled() {
		return nil
	}
	for _, block := range blocks {
		for _, l := range block.Logs {
			if len(l.Topics) == 0 {
				return errV3StreamConflict
			}
			_, own := v3ProjectEventTopics[l.Topics[0]]
			if own || l.Topics[0] == v3PolicyUpdateTopic {
				if len(l.Topics) < 2 {
					return errV3StreamConflict
				}
				n := new(big.Int).SetBytes(l.Topics[1][:])
				if !n.IsUint64() {
					return errV3StreamConflict
				}
				if err := tx.Exec(`UPDATE ido_v3_project_tracking SET wake_block=GREATEST(wake_block,?),next_scan_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE project_id=? AND chain_id=? AND proxy_address=? AND contract_version=?`, l.BlockNumber, n.Uint64(), s.Chain, v3Lower(s.Proxy), v3StreamVersion).Error; err != nil {
					return err
				}
			} else if err := tx.Exec(`UPDATE ido_v3_project_tracking SET wake_block=GREATEST(wake_block,?),next_scan_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE chain_id=? AND proxy_address=? AND contract_version=? AND (mode<>'normal' OR finished=1)`, l.BlockNumber, s.Chain, v3Lower(s.Proxy), v3StreamVersion).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// New control epoch immediately withdraws closure availability; old evidence
// is not erased but no longer qualifies through tracking.Read's epoch check.
func invalidateV3TrackingSourceTx(tx *gorm.DB, s v3StreamScope) error {
	if !projecttracking.Enabled() {
		return nil
	}
	if err := tx.Exec(`INSERT INTO ido_v3_tracking_audit(project_id,action,admin_id,reason,before_mode,after_mode,tracking_revision) SELECT project_id,'source_invalidated',NULL,'source_recovery_required',mode,'normal',revision+1 FROM ido_v3_project_tracking WHERE chain_id=? AND proxy_address=? AND contract_version=? AND (finished=1 OR mode<>'normal')`, s.args()...).Error; err != nil {
		return err
	}
	return tx.Exec(`UPDATE ido_v3_project_tracking SET mode='normal',finished=0,finished_at=NULL,pending_reason='source_recovery_required',next_scan_at=UTC_TIMESTAMP(6),revision=revision+1,updated_at=UTC_TIMESTAMP(6) WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Error
}

// Evaluate only a fully caught-up independently admitted project, after live
// postcommit anchors. Amount candidate check uses stored canonical projection;
// a one-time full replay/chain flag audit occurs only when amounts can close.
func verifyV3ProjectClosure(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity, published v3ConsumerCursor, before v3StreamCursor) error {
	if !projecttracking.Enabled() || published.CursorBlock < before.CursorBlock {
		return nil
	}
	var rows []struct{ State, RaisedRaw, SoldRaw, DepositedRaw, RefundedRaw, ClaimedRaw, ProceedsPaidRaw, FeePaidRaw, ProjectTokensWithdrawnRaw, Wallets string }
	if db.WithContext(ctx).Raw(`SELECT state,raised_raw,sold_raw,deposited_raw,refunded_raw,claimed_raw,proceeds_paid_raw,fee_paid_raw,project_tokens_withdrawn_raw,wallets FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND contract_version=? AND project_id=? AND revision=? AND cursor_block=? AND cursor_hash=?`, append(s.args(), b.ProjectID, published.Revision, published.CursorBlock, published.CursorHash)...).Scan(&rows).Error != nil || len(rows) != 1 {
		return errors.New("V3 closure projection unavailable")
	}
	p := rows[0]
	reason := "obligations_pending"
	finished := false
	digest := ""
	var flags projecttracking.ExitFlags
	var feeRate uint16
	var reuse *projecttracking.Row
	if p.State == "pending" || p.State == "active" {
		reason = "lifecycle_not_terminal"
	} else {
		parse := func(raw string) *big.Int {
			n, ok := new(big.Int).SetString(raw, 10)
			if !ok || n.Sign() < 0 || n.String() != raw {
				return nil
			}
			return n
		}
		t := commonchain.V3ReplayTotals{Raised: parse(p.RaisedRaw), Sold: parse(p.SoldRaw), Deposited: parse(p.DepositedRaw), Refunded: parse(p.RefundedRaw), Claimed: parse(p.ClaimedRaw), ProceedsPaid: parse(p.ProceedsPaidRaw), FeePaid: parse(p.FeePaidRaw), ProjectTokensWithdrawn: parse(p.ProjectTokensWithdrawnRaw)}
		states := map[string]idov3whitelist.ProjectState{"successful": idov3whitelist.ProjectSuccessful, "failed": idov3whitelist.ProjectFailed, "cancelled": idov3whitelist.ProjectCancelled}
		state, ok := states[p.State]
		if !ok {
			return errors.New("V3 closure state unknown")
		}
		t.State = state
		// FeeRate is project snapshot evidence, not a global mutable setting.
		creation, err := readV3CanonicalCreationTx(db.WithContext(ctx), b, published.CursorBlock)
		if err != nil {
			return err
		}
		feeRate = creation.FeeRate
		candidate := projecttracking.ClosureReason(t, creation.FeeRate, projecttracking.ExitFlags{Funds: true, Fee: true, Tokens: true, Deposited: p.DepositedRaw})
		reason = candidate
		if candidate == "" {
			prior, err := projecttracking.Read(ctx, db, b.ProjectID, false)
			if err != nil {
				return err
			}
			if prior.Finished && prior.WakeBlock == 0 && prior.EvidenceEpoch == before.Epoch && prior.EvidenceBlock <= published.CursorBlock && len(prior.EvidenceDigest) == 64 {
				var changed int64
				if err := db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND contract_version=? AND project_id=? AND status='canonical' AND block_number>? AND block_number<=?`, append(s.args(), b.ProjectID, prior.EvidenceBlock, published.CursorBlock)...).Scan(&changed).Error; err != nil {
					return err
				}
				if changed == 0 {
					if err := v3StreamAnchor(ctx, rpc, s, prior.EvidenceBlock, common.HexToHash(prior.EvidenceHash)); err != nil {
						return err
					}
					reuse = &prior
					finished = true
					d := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%s/%d/%d", prior.EvidenceDigest, before.Epoch, published.CursorHash, published.Revision, b.ProjectID)))
					digest = hex.EncodeToString(d[:])
				}
			}
			if reuse == nil {
				hash := common.HexToHash(published.CursorHash)
				captured := &closureTupleRPC{v3ScannerRPC: rpc, project: b.ProjectID, proxy: b.ProxyAddress}
				observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, captured, idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(b.ChainID), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: b.ProxyAddress}, rt, published.CursorBlock, hash)
				if err != nil || commonchain.MatchV3CreationProjectImmutable(creation, b, observed) != nil {
					return errV3Stream
				}
				// Reuse the strictly parsed getProject ABI to read exit flags without
				// broadening shared ProjectIdentityObservation equality semantics.
				encoded := captured.tuple
				if len(encoded) < 32+23*32 {
					return errV3Stream
				}
				word := func(i int) *big.Int { return new(big.Int).SetBytes(encoded[32+i*32 : 32+(i+1)*32]) }
				for _, i := range []int{18, 19, 20} {
					if word(i).Cmp(big.NewInt(1)) > 0 {
						return errV3StreamConflict
					}
				}
				flags = projecttracking.ExitFlags{Funds: word(18).Sign() == 1, Fee: word(19).Sign() == 1, Tokens: word(20).Sign() == 1, Deposited: word(17).String()}
				reason = projecttracking.ClosureReason(t, creation.FeeRate, flags)
				if reason == "" {
					if commonchain.MatchV3ReplayProjectEndOfBlock(b, t, observed, published.CursorBlock, hash) != nil {
						return errV3StreamConflict
					}
					if err := v3StreamAnchor(ctx, rpc, s, published.CursorBlock, hash); err != nil {
						return err
					}
					d := sha256.Sum256(append(encoded, []byte(fmt.Sprintf("/%d/%s/%d/%d", before.Epoch, published.CursorHash, published.Revision, b.ProjectID))...))
					digest = hex.EncodeToString(d[:])
					finished = true
				}
			}
		}
	}
	return v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		// Global event/reorg lock comes before stream/project/outbox locks.
		if err := chainreorg.LockNoActiveIncidentTx(tx, b.ChainID, v3Lower(b.ProxyAddress)); err != nil {
			return err
		}
		current, err := v3StreamCursorRead(tx, s, true)
		if err != nil || !v3ConsumerStreamProgress(before, current) {
			return errV3Stream
		}
		c, err := readV3ConsumerCursor(tx, b, true)
		if err != nil || c != published {
			return &v3ScanDeferred{stage: "closure_cursor_changed"}
		}
		if err := checkV3TrackingFence(ctx, tx, b.ProjectID); err != nil {
			return err
		}
		if err := projecttracking.Ensure(tx, b.ProjectID); err != nil {
			return err
		}
		row, err := projecttracking.Read(ctx, tx, b.ProjectID, true)
		if err != nil {
			return err
		}
		if reuse != nil && (row.Revision != reuse.Revision || row.WakeBlock > 0 || row.EvidenceDigest != reuse.EvidenceDigest || !row.Finished) {
			return &v3ScanDeferred{stage: "closure_reuse_changed"}
		}
		blocked, err := projecttracking.HasUnresolvedWork(tx, b.ProjectID, b.ChainID, v3Lower(b.ProxyAddress))
		if err != nil {
			return err
		}
		if blocked {
			finished = false
			reason = "transactions_unresolved"
		}
		if finished && reuse == nil {
			// Full canonical journal/effects/cursor parity under the same locks.
			if _, err := readV3PublishedProjection(ctx, tx, b); err != nil {
				return err
			}
			var actual []struct{ Payload string }
			if tx.Raw(`SELECT payload FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND contract_version=? AND project_id=? AND status='canonical' ORDER BY block_number,log_index`, append(s.args(), b.ProjectID)...).Scan(&actual).Error != nil {
				return errV3Stream
			}
			var events []commonchain.V3ReplayEvent
			for _, a := range actual {
				var e commonchain.V3ReplayEvent
				if json.Unmarshal([]byte(a.Payload), &e) != nil {
					return errV3Stream
				}
				events = append(events, e)
			}
			totals, err := commonchain.RebuildV3Replay(b, events)
			if err != nil {
				return err
			}
			if why := projecttracking.ClosureReason(totals, feeRate, flags); why != "" {
				return errors.New("V3 closure full replay disagrees")
			}
			for _, kind := range []struct {
				action   string
				required bool
			}{{"ProjectProceedsWithdrawn", totals.State == idov3whitelist.ProjectSuccessful}, {"PlatformFeeWithdrawn", totals.State == idov3whitelist.ProjectSuccessful}, {"ProjectTokensWithdrawn", true}} {
				if !kind.required {
					continue
				}
				found := false
				for _, event := range events {
					if event.Lifecycle != nil && event.Lifecycle.Kind == kind.action {
						found = true
					}
				}
				if !found {
					return errors.New("V3 closure missing canonical withdrawal event")
				}
			}
			if _, err := v3ExpectedProjection(totals); err != nil {
				return err
			}
			expected := v3ParityProjection{State: p.State, RaisedRaw: p.RaisedRaw, SoldRaw: p.SoldRaw, DepositedRaw: p.DepositedRaw, RefundedRaw: p.RefundedRaw, ClaimedRaw: p.ClaimedRaw, Wallets: p.Wallets, ProceedsPaidRaw: p.ProceedsPaidRaw, FeePaidRaw: p.FeePaidRaw, ProjectTokensWithdrawnRaw: p.ProjectTokensWithdrawnRaw}
			if err := replayV3ProjectTx(tx, b, published.CursorBlock, common.HexToHash(published.CursorHash), published.Revision, &expected); err != nil {
				return err
			}
		}
		mode := row.Mode
		if mode == "archived" && (!finished || row.WakeBlock > 0) {
			mode = "normal"
		}
		next := "NULL"
		if mode == "maintenance" || mode == "archived" {
			next = "TIMESTAMPADD(HOUR,12,UTC_TIMESTAMP(6))"
		}
		if reason == "" {
			reason = "settled"
		}
		r := tx.Exec(`UPDATE ido_v3_project_tracking SET mode=?,finished=?,finished_at=IF(?,COALESCE(finished_at,UTC_TIMESTAMP(6)),NULL),evidence_epoch=?,evidence_block=?,evidence_hash=?,evidence_revision=?,evidence_digest=?,pending_reason=?,last_scan_at=UTC_TIMESTAMP(6),next_scan_at=`+next+`,wake_block=IF(wake_block<=?,0,wake_block),revision=revision+1,updated_at=UTC_TIMESTAMP(6) WHERE project_id=? AND revision=?`, mode, finished, finished, current.Epoch, published.CursorBlock, published.CursorHash, published.Revision, digest, reason, published.CursorBlock, b.ProjectID, row.Revision)
		if r.Error != nil || r.RowsAffected != 1 {
			return errors.New("V3 closure tracking CAS changed")
		}
		if row.Finished != finished || mode != row.Mode {
			return projecttracking.Audit(tx, b.ProjectID, "closure_check", nil, reason, row.Mode, mode, row.Revision+1)
		}
		return nil
	})
}
