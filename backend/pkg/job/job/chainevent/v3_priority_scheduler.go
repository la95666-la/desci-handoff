package chainevent

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/projecttracking"
)

const (
	v3PriorityInterval = 30 * time.Second
	v3PendingInterval  = 45 * time.Second
	v3NormalInterval   = time.Minute
	v3SetInterval      = 2 * time.Minute
	v3PriorityAging    = 90 * time.Second
	v3NormalQuantum    = 25 * time.Second
)

type v3PriorityKey struct{}
type v3PriorityTurnBudgetKey struct{}
type v3NormalAuditTurnKey struct{}
type v3PriorityHint struct {
	ID           uint64 `gorm:"column:project_id"`
	Status       string
	Locked       bool `gorm:"column:onchain_locked"`
	NearBoundary bool `gorm:"column:near_boundary"`
	Unresolved   bool
	Cursor       uint64 `gorm:"column:cursor_block"`
	EventBlock   uint64 `gorm:"-"`
	FullAudit    bool   `gorm:"-"`
	ProducerTip  uint64 `gorm:"-"`
}

func (h v3PriorityHint) urgent() bool {
	return h.Status == "Active" || h.NearBoundary || h.Unresolved || h.EventBlock > h.Cursor
}
func (h v3PriorityHint) interval() time.Duration {
	if h.urgent() {
		return v3PriorityInterval
	}
	if h.Status == "Set" && !h.Locked {
		return v3SetInterval
	}
	if h.Status == "Pending" {
		return v3PendingInterval
	}
	return v3NormalInterval
}

type v3PriorityEntry struct {
	firstSeen, lastAttempt, lastSuccess, nextDue time.Time
	consumeBudget                                time.Duration
	lastProgress                                 time.Time
	followup                                     bool
	noProgress                                   uint
}
type v3PrioritySchedule struct {
	mu                sync.Mutex
	entries           map[uint64]v3PriorityEntry
	highBurst         int
	events            map[uint64]uint64
	epoch, lastPublic uint64
}

func (s *v3PrioritySchedule) retain(ids []uint64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[uint64]v3PriorityEntry)
	}
	keep := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
		if _, ok := s.entries[id]; !ok {
			s.entries[id] = v3PriorityEntry{firstSeen: now}
		}
	}
	for id := range s.entries {
		if !keep[id] {
			delete(s.entries, id)
			delete(s.events, id)
		}
	}
}

// Scheduling inputs only, not admission. Refresh after every atomic project
// turn so a status transition or new durable transaction can preempt backfill.
func loadV3PriorityHints(ctx context.Context, db *gorm.DB, ids []uint64, now time.Time) ([]v3PriorityHint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []v3PriorityHint
	query := `SELECT p.project_id,p.status,p.onchain_locked,COALESCE(c.cursor_block,0) AS cursor_block,
 ((p.status IN ('Pending','Active') AND ((p.start_time BETWEEN ? AND ?) OR (p.end_time BETWEEN ? AND ?)))) AS near_boundary,
 EXISTS(SELECT 1 FROM chain_tx_submission t WHERE t.project_id=p.project_id AND t.chain_id=p.chain_id
 AND BINARY LOWER(t.contract_address)=BINARY LOWER(p.contract_address) AND t.active_slot=1
 AND t.state IN ('intent','ready','signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','ambiguous')) AS unresolved
 FROM projects p LEFT JOIN ido_v3_scan_cursor c ON c.project_id=p.project_id AND c.chain_id=p.chain_id
 AND BINARY c.proxy_address=BINARY p.contract_address AND BINARY c.contract_version=BINARY p.contract_version
 WHERE p.project_id IN ? AND p.chain_id=56 AND BINARY p.contract_address=BINARY ?
 AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.status<>BINARY 'archived' ORDER BY p.project_id`
	// Application DATETIME fields use the application/container zone, not an
	// independently inferred DB timezone. These times change scheduling only.
	r := db.WithContext(ctx).Raw(query, now.Add(-5*time.Minute), now.Add(5*time.Minute), now.Add(-5*time.Minute), now.Add(5*time.Minute), ids, v3Lower(productionV3StreamScope().Proxy)).Scan(&rows)
	if r.Error != nil || len(rows) != len(ids) {
		return nil, errors.New("V3 priority metadata unavailable")
	}
	for i, h := range rows {
		if h.ID != ids[i] {
			return nil, errors.New("V3 priority candidate changed")
		}
		if projecttracking.Enabled() {
			tracking, err := projecttracking.Read(ctx, db, h.ID, false)
			if err != nil {
				return nil, err
			}
			mergeV3TrackingPriority(&rows[i], tracking)
		}
	}
	return rows, nil
}

func mergeV3TrackingPriority(h *v3PriorityHint, t projecttracking.Row) {
	h.EventBlock = t.WakeBlock
	// Lifecycle urgency prevents maintenance deferral; only actual unresolved
	// work or recovery should promote a Pending project to the urgent lane.
	h.Unresolved = h.Unresolved || t.Unresolved || t.RecoveryRequired
}

// New public events promote an ended project too (refund/claim/settlement).
// Inspect at most 128 new confirmed coverage heights per scheduling boundary.
// A large initial backlog is handled by ordinary fairness, not payload scans.
func (s *v3PrioritySchedule) eventHints(ctx context.Context, db *gorm.DB, scope v3StreamScope, rows []v3PriorityHint) ([]v3PriorityHint, error) {
	c, err := v3StreamCursorRead(db.WithContext(ctx), scope, false)
	if err != nil || c.State != "running" || c.Mode != "stream" {
		return nil, errV3Stream
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.events == nil || s.epoch != c.Epoch {
		s.events = make(map[uint64]uint64)
		s.lastPublic = 0
		s.epoch = c.Epoch
	}
	start := s.lastPublic + 1
	if s.lastPublic == 0 || s.lastPublic > c.CursorBlock || start < c.AnchorBlock || c.CursorBlock-s.lastPublic > 128 {
		start = c.CursorBlock
		if start >= 127 {
			start -= 127
		}
		if start < c.AnchorBlock {
			start = c.AnchorBlock
		}
	}
	if start <= c.CursorBlock {
		blocks, err := readV3ConsumerCoverage(ctx, db, scope, c.Epoch, start, c.CursorBlock)
		if err != nil {
			return nil, err
		}
		allowed := make(map[uint64]bool, len(rows))
		for _, h := range rows {
			allowed[h.ID] = true
		}
		for n := start; n <= c.CursorBlock; n++ {
			for _, l := range blocks[n].Logs {
				if len(l.Topics) < 2 {
					continue
				}
				_, known := v3ProjectEventTopics[l.Topics[0]]
				if !known && l.Topics[0] != v3PolicyUpdateTopic {
					continue
				}
				id := new(big.Int).SetBytes(l.Topics[1][:])
				if id.IsUint64() && allowed[id.Uint64()] && n > s.events[id.Uint64()] {
					s.events[id.Uint64()] = n
				}
			}
		}
		s.lastPublic = c.CursorBlock
	}
	for i := range rows {
		rows[i].ProducerTip = c.CursorBlock
		if s.events[rows[i].ID] > rows[i].EventBlock {
			rows[i].EventBlock = s.events[rows[i].ID]
		}
		if rows[i].Cursor >= rows[i].EventBlock {
			delete(s.events, rows[i].ID)
		}
	}
	return rows, nil
}

func (s *v3PrioritySchedule) selectNext(rows []v3PriorityHint, visited, failed map[uint64]bool, blocked func(uint64) bool, now time.Time) (v3PriorityHint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var high, normal []v3PriorityHint
	for _, h := range rows {
		if failed[h.ID] || blocked(h.ID) {
			continue
		}
		e := s.entries[h.ID]
		if h.urgent() {
			due := e.lastSuccess.IsZero() || now.Sub(e.lastSuccess) >= v3PriorityInterval
			if due {
				high = append(high, h)
			}
		} else if !visited[h.ID] {
			// Backlog is always eligible. Caught-up/Set/Pending polling honors
			// its interval instead of filling every leftover tick opportunity.
			if !e.nextDue.IsZero() && now.Before(e.nextDue) && !e.followup && (h.ProducerTip <= h.Cursor || h.Status == "Set" && !h.Locked) {
				continue
			}
			normal = append(normal, h)
		}
	}
	older := func(a, b v3PriorityHint) bool {
		x, y := s.entries[a.ID], s.entries[b.ID]
		if !a.urgent() && !b.urgent() {
			// One reserved follow-up after a successful historical audit. It
			// survives round clipping, but expires after ONE consuming attempt.
			if x.followup != y.followup {
				return x.followup
			}
			if x.lastAttempt.IsZero() != y.lastAttempt.IsZero() {
				return x.lastAttempt.IsZero()
			}
			// Repeated zero-progress yields cannot pin a healthy peer, but a
			// single audit/quantum yield is not a completed service interval.
			xDeferred, yDeferred := x.noProgress >= 2, y.noProgress >= 2
			if xDeferred && now.Sub(x.lastAttempt) >= 2*v3PriorityAging {
				xDeferred = false
			}
			if yDeferred && now.Sub(y.lastAttempt) >= 2*v3PriorityAging {
				yDeferred = false
			}
			if xDeferred != yDeferred {
				return !xDeferred
			}
			if xDeferred && !x.lastAttempt.Equal(y.lastAttempt) {
				return x.lastAttempt.Before(y.lastAttempt)
			}
			xService, yService := x.lastProgress, y.lastProgress
			if xService.IsZero() {
				xService = x.firstSeen
			}
			if yService.IsZero() {
				yService = y.firstSeen
			}
			if !xService.Equal(yService) {
				return xService.Before(yService)
			}
		}
		xDue, yDue := x.nextDue, y.nextDue
		if xDue.IsZero() {
			xDue = x.firstSeen.Add(a.interval())
		}
		if yDue.IsZero() {
			yDue = y.firstSeen.Add(b.interval())
		}
		if !xDue.Equal(yDue) {
			return xDue.Before(yDue)
		}
		if !x.lastAttempt.Equal(y.lastAttempt) {
			return x.lastAttempt.Before(y.lastAttempt)
		}
		return a.ID < b.ID
	}
	sort.Slice(high, func(i, j int) bool { return older(high[i], high[j]) })
	sort.Slice(normal, func(i, j int) bool { return older(normal[i], normal[j]) })
	if len(normal) > 0 {
		e := s.entries[normal[0].ID]
		since := e.lastProgress
		if since.IsZero() {
			since = e.firstSeen
		}
		// At most two high-priority turns before one normal opportunity. Age
		// advances a waiting project without forging a successful observation.
		if len(high) == 0 || s.highBurst >= 2 || s.highBurst >= 1 && now.Sub(since) >= v3PriorityAging {
			return normal[0], true
		}
	}
	if len(high) > 0 {
		return high[0], true
	}
	return v3PriorityHint{}, false
}

func (s *v3PrioritySchedule) record(h v3PriorityHint, started, finished time.Time, err error) {
	s.recordProgress(h, started, finished, err, false)
}

func (s *v3PrioritySchedule) recordProgress(h v3PriorityHint, started, finished time.Time, err error, advanced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[h.ID]
	e.lastAttempt = started
	// Consume a reservation on any attempt, successful or not. Only a new
	// successful full audit can create a new one; arbitrary errors cannot.
	e.followup = v3SchedulingReason(err) == "audit_ready_yield"
	if advanced || err == nil {
		e.lastProgress = finished
		e.noProgress = 0
	} else {
		if e.noProgress < 2 {
			e.noProgress++
		}
	}
	if v3SchedulingReason(err) == "consumer_budget_yield" || v3SchedulingReason(err) == "publication_budget_yield" {
		e.consumeBudget = approvedV3ProjectBudget
	}
	if err == nil {
		e.lastSuccess = finished
		e.nextDue = finished.Add(h.interval())
		e.consumeBudget = 0
	} else if e.followup {
		e.consumeBudget = approvedV3ProjectBudget
		e.nextDue = finished
	} else if v3SchedulingYield(err) {
		e.nextDue = finished
	} else {
		e.nextDue = finished.Add(5 * time.Second)
	}
	s.entries[h.ID] = e
	if h.urgent() {
		if s.highBurst < 2 {
			s.highBurst++
		}
	} else {
		s.highBurst = 0
	}
}

// Every started operation remains bounded and finishes/revokes before the
// next selection; no preemption inside an atomic writer and no concurrency.
func runV3PriorityRound(ctx context.Context, ids []uint64, s *v3PrioritySchedule, clock func() time.Time,
	refresh func() ([]v3PriorityHint, error), blocked func(uint64) bool, scan func(context.Context, uint64) error, revoke func(uint64) error) (approvedV3ScanResult, error) {
	result := approvedV3ScanResult{Total: len(ids)}
	invalid := errors.New("V3 priority round incomplete")
	if ctx == nil || s == nil || clock == nil || refresh == nil || scan == nil || revoke == nil || blocked == nil {
		return result, invalid
	}
	for i, id := range ids {
		if id == 0 || i > 0 && id <= ids[i-1] {
			return result, invalid
		}
	}
	s.retain(ids, clock())
	visited, failed, completed, deferred := map[uint64]bool{}, map[uint64]bool{}, map[uint64]bool{}, map[uint64]bool{}
	// Repeated urgent success is allowed between normal batches, but cannot
	// increase whole-round completion counts or erase any earlier failure.
	for turns := 0; turns < 2*len(ids)+2 && len(completed) < len(ids); turns++ {
		if ctx.Err() != nil {
			break
		}
		rows, err := refresh()
		if err != nil {
			return result, err
		}
		if len(rows) != len(ids) {
			return result, invalid
		}
		for i, h := range rows {
			if h.ID != ids[i] {
				return result, invalid
			}
		}
		h, ok := s.selectNext(rows, visited, failed, blocked, clock())
		if !ok {
			break
		}
		budget := v3NormalQuantum
		s.mu.Lock()
		reserved := s.entries[h.ID].consumeBudget
		s.mu.Unlock()
		if reserved > budget {
			budget = reserved
		}
		if h.urgent() || h.FullAudit {
			budget = approvedV3ProjectBudget
		}
		if h.Locked && h.ProducerTip > h.Cursor {
			budget = approvedV3ProjectBudget
		}
		// Never start an Active in the clipped tail of a background round.
		// Return to the five-second scheduler instead of adding failure backoff.
		if !v3HasProjectBudget(ctx, budget) {
			break
		}
		started := clock()
		s.mu.Lock()
		previous := s.entries[h.ID]
		s.mu.Unlock()
		project, cancel := context.WithTimeout(ctx, budget)
		project = context.WithValue(project, v3PriorityTurnBudgetKey{}, true)
		if !h.urgent() {
			project = context.WithValue(project, v3NormalAuditTurnKey{}, true)
		}
		project = newV3ScanTrace(project, h.ID)
		if h.Locked && h.ProducerTip > h.Cursor && h.ProducerTip-h.Cursor > idov3whitelist.ProjectFreshnessMaxLag {
			project = context.WithValue(project, v3CatchupBatchKey{}, true)
		}
		err = scan(project, h.ID)
		if err == nil {
			err = project.Err()
		}
		visited[h.ID] = true
		if err != nil {
			failed[h.ID] = true
			delete(completed, h.ID)
			deferred[h.ID] = v3Deferred(err)
			decision := "defer"
			if !v3Deferred(err) {
				decision = "persistent_revoke"
				if re := revoke(h.ID); re != nil {
					logV3ScanFailure(project, h.ID, re, "revoke_failed")
				}
			}
			logV3ScanFailure(project, h.ID, err, decision)
		} else {
			completed[h.ID] = true
		}
		cancel()
		advanced := false
		if trace, ok := project.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
			trace.mu.Lock()
			advanced = trace.consumerObserved && trace.consumerTo > trace.consumerFrom
			trace.mu.Unlock()
		}
		s.recordProgress(h, started, clock(), err, advanced)
		if logger := log.GetLogger(); logger != nil {
			s.mu.Lock()
			e := s.entries[h.ID]
			s.mu.Unlock()
			age := int64(-1)
			if !previous.lastSuccess.IsZero() {
				age = int64(started.Sub(previous.lastSuccess).Seconds())
			}
			late := int64(0)
			if !previous.nextDue.IsZero() && started.After(previous.nextDue) {
				late = started.Sub(previous.nextDue).Milliseconds()
			}
			logger.WithFields(map[string]any{"project_id": strconv.FormatUint(h.ID, 10), "urgent": h.urgent(), "status": h.Status, "duration_ms": clock().Sub(started).Milliseconds(), "success": err == nil, "yield_reason": v3SchedulingReason(err), "previous_success_age_seconds": age, "queue_overdue_ms": late, "next_due_at": e.nextDue.UTC().Format(time.RFC3339Nano), "advanced": advanced, "audit_followup": e.followup, "no_progress_turns": e.noProgress}).Info("V3 priority project turn")
		}
	}
	result.Attempted = len(visited)
	result.Completed = len(completed)
	result.Skipped = len(ids) - len(visited)
	result.Failed = len(failed)
	for _, isDeferred := range deferred {
		if isDeferred {
			result.Deferred++
		}
	}
	if ctx.Err() != nil || result.Completed != len(ids) || result.Failed != 0 {
		return result, invalid
	}
	return result, nil
}
