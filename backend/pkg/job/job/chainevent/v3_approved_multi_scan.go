package chainevent

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

// scanApprovedV3All restarts keyset enumeration from zero each tick. A
// persistent high-water mark would permanently abandon previously admitted
// projects, including those whose cursor must be monitored for reorgs. The
// caller's bounded context controls tick duration; no fixed project-count cap
// may exclude future projects. A partial tick returns an error, not success.
func scanApprovedV3All(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	ownerHash, governorHash common.Hash) error {
	const pageSize = 32
	var after uint64
	var failed bool
	for {
		if ctx == nil || ctx.Err() != nil {
			return errors.New("V3 approved scanner tick expired")
		}
		next, fetched, err := scanApprovedV3Page(ctx, db, rpc, after, pageSize, ownerHash, governorHash)
		if next < after || (fetched > 0 && next == after) {
			return errors.New("V3 approved scanner enumeration unavailable")
		}
		if err != nil {
			failed = true
		}
		if fetched < pageSize {
			if failed {
				return errors.New("V3 approved scanner partial tick unavailable")
			}
			return nil
		}
		if next == math.MaxUint64 {
			return errors.New("V3 approved scanner project ID exhausted")
		}
		after = next
	}
}

// scanApprovedV3Page is a bounded multi-project driver. It
// cannot turn a Set into a scanner source: each project is admitted afresh by
// an executed decision, the original signed create and canonical receipts.
// A per-project error halts that project's published projection and continues
// checking the remaining IDs. The whole tick still returns an error. The
// in-memory keyset is not persisted and restarts at zero on the next tick.
// A completed tick is published only after all pages and project checks pass.
func scanApprovedV3Page(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	after uint64, pageSize int, ownerHash, governorHash common.Hash) (uint64, int, error) {
	invalid := errors.New("V3 approved scanner page unavailable")
	if ctx == nil || db == nil || rpc == nil || pageSize < 1 || pageSize > 32 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return after, 0, invalid
	}
	ids, err := idov3whitelist.ListApprovedV3ProjectIDs(ctx, db, after, pageSize)
	if err != nil {
		return after, 0, invalid
	}
	var failed bool
	for _, id := range ids {
		if err := scanApprovedV3Project(ctx, db, rpc, id, ownerHash, governorHash); err != nil {
			// A previously published wallet projection must not survive a
			// failed admission tick. The revocation is fenced to the exact
			// cursor revision, and never guesses a canonical ancestor. RPC
			// failure may have exhausted ctx: use a separate bounded DB context.
			haltCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			haltErr := haltApprovedV3Projection(haltCtx, db, id)
			cancel()
			if haltErr != nil {
				failed = true
			}
			// Continue monitoring other projects: one failed admission must not
			// leave their previously published projections stale. Report the
			// partial tick as failed and restart enumeration at zero next tick.
			failed = true
			continue
		}
	}
	if len(ids) != 0 {
		after = ids[len(ids)-1]
	}
	if failed {
		return after, len(ids), invalid
	}
	return after, len(ids), nil
}

func haltApprovedV3Projection(ctx context.Context, database *gorm.DB, projectID uint64) error {
	invalid := errors.New("V3 approved projection halt unavailable")
	if ctx == nil || database == nil || projectID == 0 {
		return invalid
	}
	var rows []struct {
		Block        uint64 `gorm:"column:cursor_block"`
		Revision     uint64 `gorm:"column:revision"`
		State        string `gorm:"column:state"`
		ProjectToken string `gorm:"column:token_address"`
		PaymentToken string `gorm:"column:payment_token_address"`
	}
	read := database.WithContext(ctx).Raw(`SELECT s.cursor_block,s.revision,s.state,p.token_address,p.payment_token_address FROM ido_v3_scan_cursor s
		JOIN ido_v3_project_binding b ON b.project_id=s.project_id AND b.chain_id=s.chain_id
		 AND BINARY b.proxy_address=BINARY s.proxy_address AND BINARY b.contract_version=BINARY s.contract_version
		JOIN projects p ON p.project_id=b.project_id AND p.chain_id=b.chain_id
		 AND BINARY p.contract_address=BINARY b.proxy_address AND BINARY p.contract_version=BINARY b.contract_version
		WHERE s.project_id=? AND s.chain_id=56 AND BINARY s.proxy_address=BINARY ?
		 AND BINARY s.contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`, projectID,
		"0x0d4040875742975269c55e16c8884d3ee9510b8e").Scan(&rows)
	if read.Error != nil || len(rows) > 1 {
		return invalid
	}
	if len(rows) == 0 {
		// A pre-lock Set has no published cursor. A missing/corrupt cursor for
		// an already published projection is different: without its revision
		// fence we cannot safely claim that revocation succeeded.
		var projections int64
		if database.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_project_projection
			WHERE chain_id=56 AND project_id=? AND BINARY proxy_address=BINARY ?
			AND BINARY contract_version=BINARY 'ido_v3_stablecoin'`, projectID,
			"0x0d4040875742975269c55e16c8884d3ee9510b8e").Scan(&projections).Error != nil || projections != 0 {
			return invalid
		}
		return nil
	}
	if rows[0].State == "reorg_halted" {
		// A halted cursor is not itself proof that every published projection
		// was revoked. Refuse to acknowledge a partially committed halt.
		var published int64
		if database.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_project_projection
			WHERE chain_id=56 AND project_id=? AND BINARY proxy_address=BINARY ?
			AND BINARY contract_version=BINARY 'ido_v3_stablecoin'`, projectID,
			"0x0d4040875742975269c55e16c8884d3ee9510b8e").Scan(&published).Error != nil || published != 0 {
			return invalid
		}
		return nil
	}
	if rows[0].State != "running" || rows[0].Block == 0 || rows[0].Revision == 0 {
		return invalid
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: projectID,
		ProxyAddress: common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e"),
		ProjectToken: common.HexToAddress(rows[0].ProjectToken), PaymentToken: common.HexToAddress(rows[0].PaymentToken)}
	return (v3Journal{db: database}).haltRangeUnknown(ctx, binding, rows[0].Block, rows[0].Revision)
}

func scanApprovedV3Project(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	projectID uint64, ownerHash, governorHash common.Hash) error {
	return scanApprovedV3ProjectWithRange(ctx, db, rpc, projectID, ownerHash, governorHash, nil)
}

func scanApprovedV3ProjectWithRange(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	projectID uint64, ownerHash, governorHash common.Hash,
	consume func(context.Context, *gorm.DB, v3ScannerRPC, idov3whitelist.AdmittedProject) error) (resultErr error) {
	if ctx == nil || db == nil || rpc == nil {
		return errors.New("V3 approved project scanner unavailable")
	}
	if turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn); ok && turn.cache != nil {
		defer func() {
			if resultErr != nil && !v3Deferred(resultErr) {
				turn.cache.forget(projectID)
			}
		}()
	}
	tracked := &v3TrackedRPC{v3ScannerRPC: rpc}
	rpc = tracked
	if db != nil {
		failures := &v3DBFailures{Interface: db.Logger}
		db = db.Session(&gorm.Session{Logger: failures})
		defer func() { resultErr = failures.finish(resultErr) }()
	}
	defer func() { resultErr = tracked.finish(ctx, "project_admission_or_consume", resultErr) }()
	invalid := errors.New("V3 approved project scanner unavailable")
	v3TracePhase(ctx, "approved_set")
	if ctx == nil || db == nil || rpc == nil || projectID == 0 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, db, projectID)
	if err != nil {
		return invalid
	}
	var state []struct {
		Locked bool   `gorm:"column:onchain_locked"`
		Status string `gorm:"column:status"`
	}
	v3TracePhase(ctx, "project_identity")
	read := db.WithContext(ctx).Raw(`SELECT onchain_locked,status FROM projects WHERE project_id=?
		AND chain_id=56 AND BINARY contract_address=BINARY ?
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY funding_mode=BINARY 'capped_presale' LIMIT 2`, projectID, record.Identity.Proxy).Scan(&state)
	if read.Error != nil || len(state) != 1 {
		return invalid
	}
	if !state[0].Locked {
		if turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn); ok && turn.cache != nil {
			turn.cache.forget(projectID)
		}
		v3TracePhase(ctx, "creation_bridge")
		if state[0].Status != constant.ProjectStatusSet {
			return invalid
		}
		attempts, err := approvedV3CreateAttempts(ctx, db, projectID, record.Identity.Proxy, record.Identity.Operator)
		if err != nil {
			return invalid
		}
		if len(attempts) == 0 {
			return cancelApprovedV3StaleUnsigned(ctx, db, rpc, record, ownerHash, governorHash)
		} // Set without a signed create is not a scanner source.
		if attempts[0].State != constant.ChainTxStateFinalSuccess {
			finalized, err := chaintx.ReconcileApprovedV3CreationReceipt(ctx, db, rpc, projectID, attempts[0].ID,
				ownerHash, governorHash)
			if err != nil {
				return invalid
			}
			if finalized == nil || finalized.State == constant.ChainTxStateFinalRevert {
				return nil // Revert releases the original nonce; only the application can create a new attempt.
			}
			if finalized.State != constant.ChainTxStateFinalSuccess {
				return invalid
			}
		}
		return bridgeApprovedV3Creation(ctx, db, rpc, projectID, ownerHash, governorHash)
	}
	var cursor []struct {
		State string `gorm:"column:state"`
	}
	v3TracePhase(ctx, "cursor_state")
	read = db.WithContext(ctx).Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=56
		AND project_id=? AND BINARY proxy_address=BINARY ?
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`,
		projectID, record.Identity.Proxy).Scan(&cursor)
	if read.Error != nil || len(cursor) != 1 {
		return invalid
	}
	if cursor[0].State == "reorg_halted" {
		if turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn); ok && turn.cache != nil {
			turn.cache.forget(projectID)
		}
		v3TracePhase(ctx, "legacy_recovery")
		// A halted row is not ordinary scanner admission. The existing bounded
		// recovery coordinator verifies the entire surviving prefix and the
		// original creation hash before it may republish a cursor.
		candidate, err := idov3whitelist.ReadApprovedHaltedCandidate(ctx, db, projectID)
		if err != nil || candidate.Record.Identity != record.Identity || candidate.CreationBlock == 0 ||
			idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, ownerHash, governorHash,
				candidate.CreationBlock, candidate.CreationHash) != nil {
			return invalid
		}
		admitted := idov3whitelist.AdmittedProject{Binding: candidate.Binding, Runtime: candidate.Runtime,
			StartBlock: 128805696, CreationBlock: candidate.CreationBlock, Confirmations: 15}
		if err := recoverV3HaltedPrefix(ctx, db, rpc, admitted); err != nil {
			return invalid
		}
		// Recovery changed cursor/outbox state. Re-admit from DB and chain as
		// a running source; the historical candidate never reaches a proof.
	} else if cursor[0].State != "running" {
		return invalid
	}
	// A locked project requires its original applied hash, canonical creation
	// log and running cursor. Never borrow another project's scanner manifest.
	v3TracePhase(ctx, "creation_admission")
	admitted, err := admitV3ScannerProject(ctx, db, rpc, projectID, ownerHash, governorHash)
	if err != nil {
		if v3Deferred(err) {
			return err
		}
		return invalid
	}
	if admitted.Binding.ProjectID != projectID ||
		strings.ToLower(admitted.Binding.ProxyAddress.Hex()) != record.Identity.Proxy {
		return invalid
	}
	ctx, cancelAudit := v3ScannerAuditDeadline(ctx, projectID)
	defer cancelAudit()
	if consume != nil {
		if err := consume(ctx, db, rpc, admitted); err != nil {
			return err
		}
	} else if _, err := v3ScanApprovedConfirmedRange(ctx, db, rpc, admitted.Binding, admitted.Runtime, 15, approvedV3ProjectBlocks); err != nil {
		return invalid
	}
	// Lifecycle receipts are original-hash only. An absent receipt is unknown:
	// keep its signed nonce fenced and retry on the next scan tick, never send
	// the same raw transaction or manufacture a success state.
	var lifecycle []struct {
		ID uint64 `gorm:"column:id"`
	}
	v3TracePhase(ctx, "lifecycle_receipt")
	read = db.WithContext(ctx).Raw(`SELECT id FROM chain_tx_submission WHERE chain_id=56 AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY LOWER(sender_address)=BINARY ?
		AND BINARY action IN ('ido_v3_activate','ido_v3_end','ido_v3_withdraw_platform_fee')
		AND active_slot=1 AND state IN ('signed','submitted','broadcast_unknown','mined_success','mined_revert')
		LIMIT 2`, projectID, record.Identity.Proxy, record.Identity.Operator).Scan(&lifecycle)
	if read.Error != nil || len(lifecycle) > 1 {
		return invalid
	}
	if len(lifecycle) == 1 {
		if _, err := chaintx.ReconcileApprovedV3LifecycleReceipt(ctx, db, rpc, projectID,
			lifecycle[0].ID, ownerHash, governorHash); err != nil {
			// The already-scanned projection remains independently canonical;
			// receipt unavailability must not revoke it or release the nonce.
			return nil
		}
	}
	return nil
}

type approvedV3CreateAttempt struct {
	ID    uint64 `gorm:"column:id"`
	State string `gorm:"column:state"`
}

// The outbox persists the EIP-55 checksummed sender while the four-eyes
// identity stores canonical lowercase. Match the same address, not its case;
// retain the project, chain, proxy, action and active signed-slot fences.
func approvedV3CreateAttempts(ctx context.Context, database *gorm.DB, projectID uint64, proxy, operator string) ([]approvedV3CreateAttempt, error) {
	if ctx == nil || database == nil || projectID == 0 || proxy == "" || operator == "" {
		return nil, errors.New("V3 approved create attempt unavailable")
	}
	var attempts []approvedV3CreateAttempt
	read := database.WithContext(ctx).Raw(`SELECT id,state FROM chain_tx_submission WHERE chain_id=56 AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY LOWER(sender_address)=BINARY ?
		AND BINARY action=BINARY 'ido_v3_create' AND active_slot=1
		AND state IN ('signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success')
		LIMIT 2`, projectID, proxy, operator).Scan(&attempts)
	if read.Error != nil || len(attempts) > 1 {
		return nil, errors.New("V3 approved create attempt unavailable")
	}
	return attempts, nil
}
