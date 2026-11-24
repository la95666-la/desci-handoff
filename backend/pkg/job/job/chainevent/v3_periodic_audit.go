package chainevent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
)

const v3FullAuditMaximumAge = 10 * time.Minute
const v3AuditEntries = 1024
const v3AuditRoundReserve = 50 * time.Second

type v3AuditContextKey struct{}
type v3AuditGrantKey struct{}
type v3AuditTurn struct {
	cache *v3PeriodicAudit
	// Runtime source ownership is part of the cache namespace, not authority
	// supplied by YAML, an HTTP request, a database row or another process.
	source [32]byte
}
type v3AuditScope struct {
	source          [32]byte
	owner, governor common.Hash
	epoch, anchor   uint64
	proxy           common.Address
}
type v3AuditSnapshot struct {
	candidate idov3whitelist.ApprovedLockedCandidate
	attempt   [32]byte
}
type v3AuditEntry struct {
	scope              v3AuditScope
	snapshot           v3AuditSnapshot
	completed, expires time.Time
}

// Process-local, scanner-only history evidence. It is neither action/proof
// authority nor a persistent recovery receipt. Every restart starts empty.
type v3PeriodicAudit struct {
	mu      sync.Mutex
	entries map[uint64]v3AuditEntry
}

func (c *v3PeriodicAudit) forget(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}

func (c *v3PeriodicAudit) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
}

func (c *v3PeriodicAudit) needsFull(id uint64, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[id]
	return !ok || now.Before(e.completed) || !now.Add(v3AuditRoundReserve).Before(e.expires)
}

func (c *v3PeriodicAudit) retain(ids []uint64) {
	keep := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id := range c.entries {
		if !keep[id] {
			delete(c.entries, id)
		}
	}
}

// Stagger refreshes WITHOUT extending the hard ten-minute validity window.
// A project's initial audit is always full; refresh happens after 8..10m.
func v3AuditLifetime(id uint64) time.Duration {
	return v3FullAuditMaximumAge - 2*time.Minute + time.Duration(id%121)*time.Second
}

func sameV3AuditEvidence(a, b v3AuditSnapshot) bool {
	return a.attempt == b.attempt && chaintx.RecheckPolicyObservationCandidate(a.candidate, b.candidate) == nil
}

// Full and hot reads must BOTH finish on the same immutable evidence and
// monotone cursor. Expired/failed work never renews a certificate. The mutex
// covers this one serial scanner operation, not RPC-independent Job tasks.
func (c *v3PeriodicAudit) check(ctx context.Context, id uint64, scope v3AuditScope, now func() time.Time,
	read func() (v3AuditSnapshot, error), full func() (v3AuditSnapshot, error), hot func(v3AuditSnapshot) error) (v3AuditSnapshot, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	invalid := errors.New("V3 scanner audit unavailable")
	fail := func(mode string, err error) (v3AuditSnapshot, string, error) {
		delete(c.entries, id)
		return v3AuditSnapshot{}, mode, err
	}
	if ctx == nil || id == 0 || scope.epoch == 0 {
		return fail("invalid", invalid)
	}
	if ctx.Err() != nil {
		return fail("canceled", ctx.Err())
	}
	before, err := read()
	if err != nil {
		return fail("read", err)
	}
	t := now()
	entry, found := c.entries[id]
	valid := found && entry.scope == scope && !t.Before(entry.completed) && t.Add(v3AuditRoundReserve).Before(entry.expires) && sameV3AuditEvidence(entry.snapshot, before)
	mode := "reuse"
	if !valid {
		mode = "full"
		delete(c.entries, id)
		audited, err := full()
		if err != nil {
			return fail(mode, err)
		}
		if !sameV3AuditEvidence(before, audited) {
			return fail(mode, invalid)
		}
		// A full audit itself can take time. Its validity begins at START,
		// never at an arbitrarily delayed completion. No stale fallback.
		entry = v3AuditEntry{scope: scope, snapshot: audited, completed: t, expires: t.Add(v3AuditLifetime(id))}
		before = audited
	} else if err := hot(before); err != nil {
		return fail(mode, err)
	}
	after, err := read()
	if err != nil {
		return fail(mode, err)
	}
	if ctx.Err() != nil {
		return fail(mode, ctx.Err())
	}
	if !sameV3AuditEvidence(before, after) {
		return fail(mode, invalid)
	}
	t = now()
	if t.Before(entry.completed) || !t.Before(entry.expires) {
		return fail(mode, &v3ScanDeferred{stage: "full_audit_expired"})
	}
	entry.snapshot = after
	if c.entries == nil {
		c.entries = make(map[uint64]v3AuditEntry)
	}
	if len(c.entries) >= v3AuditEntries && !found {
		// Saturation must not produce an unbounded, expiry-less grant.
		return fail(mode, &v3ScanDeferred{stage: "audit_capacity"})
	}
	c.entries[id] = entry
	return after, mode, nil
}

func readV3AuditSnapshot(ctx context.Context, database *gorm.DB, id uint64) (v3AuditSnapshot, error) {
	c, err := idov3whitelist.ReadApprovedLockedCandidate(ctx, database, id)
	if err != nil {
		return v3AuditSnapshot{}, err
	}
	// Detect mutation of the original applied-attempt evidence, including signed
	// bytes/calldata/finality/signing time, not just the candidate join subset.
	var rows []entity.ChainTxSubmission
	r := database.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE id=? AND project_id=? AND chain_id=? AND BINARY action=BINARY 'ido_v3_create' AND BINARY contract_address=BINARY ? AND BINARY state=BINARY 'applied' AND active_slot IS NULL LIMIT 2`, c.SubmissionID, id, c.Binding.ChainID, c.Record.Identity.Proxy).Scan(&rows)
	if r.Error != nil || len(rows) != 1 {
		return v3AuditSnapshot{}, errors.New("V3 scanner original attempt unavailable")
	}
	// Model JSON omits signed bytes/active_slot by design. Hash them explicitly
	// as well; only the digest stays in memory and nothing is logged.
	digest, err := v3AuditAttemptDigest(rows[0])
	if err != nil {
		return v3AuditSnapshot{}, err
	}
	return v3AuditSnapshot{candidate: c, attempt: digest}, nil
}

func v3AuditAttemptDigest(row entity.ChainTxSubmission) ([32]byte, error) {
	raw, err := json.Marshal(struct {
		Row             entity.ChainTxSubmission
		RawTx, CallData []byte
		Slot            *uint8
		SignedAt        *time.Time
	}{row, row.RawTx, row.CallData, row.ActiveSlot, row.V3SignedAt})
	if err != nil {
		return [32]byte{}, errors.New("V3 scanner attempt digest unavailable")
	}
	return sha256.Sum256(raw), nil
}

func recheckV3AuditLive(ctx context.Context, rpc v3ScannerRPC, snapshot v3AuditSnapshot, owner, governor common.Hash) error {
	return recheckV3AuditLiveWithDeployment(ctx, rpc, snapshot, func(ctx context.Context, c idov3whitelist.ApprovedLockedCandidate, height uint64, hash common.Hash) error {
		return idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, c.Record, owner, governor, height, hash)
	})
}

func recheckV3AuditLiveWithDeployment(ctx context.Context, rpc v3ScannerRPC, snapshot v3AuditSnapshot,
	verify func(context.Context, idov3whitelist.ApprovedLockedCandidate, uint64, common.Hash) error) error {
	invalid := errors.New("V3 scanner live audit unavailable")
	c := snapshot.candidate
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, c.CursorBlock, 15) || !idov3whitelist.HasConfirmations(head, c.CreationBlock, 15) {
		return invalid
	}
	// Creation anchors remain live even though receipt/log decoding and
	// creation-block economics/signature audits are no longer repeated here.
	for _, anchor := range []struct {
		height uint64
		hash   common.Hash
	}{{c.CreationBlock, c.CreationHash}, {c.CursorBlock, c.CursorHash}} {
		h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(anchor.height))
		if e != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != anchor.height || h.Hash() != anchor.hash {
			return invalid
		}
	}
	confirmed := head - 14
	var sharedHash common.Hash
	if shared, ok := ctx.Value(v3SharedConfirmedKey{}).(v3SharedConfirmed); ok {
		if !idov3whitelist.HasConfirmations(head, shared.height, 15) || shared.height < c.CreationBlock {
			return invalid
		}
		confirmed, sharedHash = shared.height, shared.hash
	}
	h, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(confirmed))
	if err != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != confirmed {
		return invalid
	}
	// Current deployment/owner/governor/operator are NOT cached across turns.
	if sharedHash != (common.Hash{}) && h.Hash() != sharedHash {
		return errV3StreamConflict
	}
	if verify(ctx, c, confirmed, h.Hash()) != nil {
		return invalid
	}
	var creation commonchain.V3ReplayEvent
	if json.Unmarshal([]byte(c.CreationPayload), &creation) != nil || creation.Created == nil {
		return invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(c.Binding.ChainID), ProjectID: new(big.Int).SetUint64(c.Binding.ProjectID), ProxyAddress: c.Binding.ProxyAddress}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, c.Runtime, confirmed, h.Hash())
	if err != nil || commonchain.MatchV3CreationProjectImmutable(creation.Created, c.Binding, observed) != nil {
		return invalid
	}
	for _, anchor := range []struct {
		height uint64
		hash   common.Hash
	}{{c.CreationBlock, c.CreationHash}, {c.CursorBlock, c.CursorHash}, {confirmed, h.Hash()}} {
		after, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(anchor.height))
		if e != nil || after == nil || after.Number == nil || !after.Number.IsUint64() || after.Number.Uint64() != anchor.height || after.Hash() != anchor.hash {
			return invalid
		}
	}
	return ctx.Err()
}

func admitV3ScannerProject(ctx context.Context, database *gorm.DB, rpc v3ScannerRPC, id uint64, owner, governor common.Hash) (idov3whitelist.AdmittedProject, error) {
	turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn)
	if !ok || turn.cache == nil {
		return chaintx.AdmitApprovedLockedProject(ctx, database, rpc, id, owner, governor)
	}
	streamScope := productionV3StreamScope()
	stream, err := v3StreamCursorRead(database.WithContext(ctx), streamScope, false)
	if err != nil || stream.State != "running" || stream.Mode != "stream" {
		turn.cache.forget(id)
		return idov3whitelist.AdmittedProject{}, errV3Stream
	}
	scope := v3AuditScope{source: turn.source, owner: owner, governor: governor, epoch: stream.Epoch, anchor: stream.AnchorBlock, proxy: streamScope.Proxy}
	started := time.Now()
	tracked := &v3TrackedRPC{v3ScannerRPC: rpc}
	rpc = tracked
	snapshot, mode, err := turn.cache.check(ctx, id, scope, time.Now,
		func() (v3AuditSnapshot, error) { return readV3AuditSnapshot(ctx, database, id) },
		func() (v3AuditSnapshot, error) {
			_, audited, err := chaintx.AuditApprovedLockedProjectForScanner(ctx, database, rpc, id, owner, governor)
			if err != nil {
				return v3AuditSnapshot{}, err
			}
			now, err := readV3AuditSnapshot(ctx, database, id)
			if err != nil || chaintx.RecheckPolicyObservationCandidate(audited, now.candidate) != nil {
				return v3AuditSnapshot{}, errors.New("V3 scanner full audit changed")
			}
			return now, nil
		}, func(s v3AuditSnapshot) error { return recheckV3AuditLive(ctx, rpc, s, owner, governor) })
	current, readErr := v3StreamCursorRead(database.WithContext(ctx), streamScope, false)
	if readErr != nil || current.Epoch != scope.epoch || current.AnchorBlock != scope.anchor || current.State != "running" || current.Mode != "stream" {
		turn.cache.forget(id)
		return idov3whitelist.AdmittedProject{}, &v3ScanDeferred{stage: "audit_stream_changed"}
	}
	if logger := log.GetLogger(); logger != nil {
		tracked.mu.Lock()
		reads := tracked.reads
		tracked.mu.Unlock()
		logger.WithFields(map[string]any{"project_id": strconv.FormatUint(id, 10), "mode": mode, "elapsed_ms": time.Since(started).Milliseconds(), "rpc_reads": reads, "success": err == nil, "maximum_age_seconds": int(v3FullAuditMaximumAge.Seconds())}).Info("V3 scanner admission audit")
	}
	if err != nil {
		return idov3whitelist.AdmittedProject{}, err
	}
	if mode == "full" && ctx.Value(v3NormalAuditTurnKey{}) == true {
		// Full history audit has its own 35s opportunity. Persist only the
		// process-local baseline and return to the priority selector; do not
		// start a clipped consumer or count this as consumed/fresh evidence.
		return idov3whitelist.AdmittedProject{}, &v3ScanDeferred{stage: "audit_ready_yield"}
	}
	c := snapshot.candidate
	return idov3whitelist.AdmittedProject{Binding: c.Binding, Runtime: c.Runtime, StartBlock: 128805696, CreationBlock: c.CreationBlock, Confirmations: 15}, nil
}

func v3ScannerAuditDeadline(ctx context.Context, id uint64) (context.Context, context.CancelFunc) {
	if turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn); ok && turn.cache != nil {
		turn.cache.mu.Lock()
		entry, found := turn.cache.entries[id]
		turn.cache.mu.Unlock()
		if found {
			bounded, cancel := context.WithDeadline(ctx, entry.expires)
			return context.WithValue(bounded, v3AuditGrantKey{}, entry), cancel
		}
	}
	return context.WithCancel(ctx)
}

// Run inside the existing stream->project transaction before business writes,
// as well as before preparing a window. Never publishes a proof or releases a
// nonce. Existing canonical/economic/checkpoint/CAS checks remain independent.
func recheckV3ScannerAuditGrant(ctx context.Context, database *gorm.DB, b idov3whitelist.ExpectedProjectBinding, stream v3StreamCursor) error {
	grant, ok := ctx.Value(v3AuditGrantKey{}).(v3AuditEntry)
	if !ok {
		return nil
	} // non-cached legacy/recovery/full-only path
	return validateV3ScannerAuditGrant(ctx, grant, b, stream, func() (v3AuditSnapshot, error) { return readV3AuditSnapshot(ctx, database, b.ProjectID) })
}

func validateV3ScannerAuditGrant(ctx context.Context, grant v3AuditEntry, b idov3whitelist.ExpectedProjectBinding, stream v3StreamCursor, read func() (v3AuditSnapshot, error)) error {
	invalid := errors.New("V3 scanner audit grant changed")
	if ctx.Err() != nil || time.Now().Before(grant.completed) || !time.Now().Before(grant.expires) {
		return &v3ScanDeferred{stage: "audit_grant_expired"}
	}
	if stream.Mode != "stream" || stream.State != "running" || stream.Epoch != grant.scope.epoch || stream.AnchorBlock != grant.scope.anchor {
		return &v3ScanDeferred{stage: "audit_epoch_changed"}
	}
	if grant.snapshot.candidate.Binding != b {
		return invalid
	}
	current, err := read()
	if err != nil || !sameV3AuditEvidence(grant.snapshot, current) {
		return invalid
	}
	return nil
}
