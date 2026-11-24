package chainevent

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

const (
	approvedV3ScanWorkers   = 3
	approvedV3ProjectBudget = 35 * time.Second
	// Admission is read-only for running stream projects. Its timeout is
	// transient, never an inferred identity failure. Actual consume windows
	// separately reserve 15 seconds after the current durable tips are read.
	approvedV3StreamAdmissionFloor = 5 * time.Second
	// Upper bound only: the driver yields between atomic windows when its
	// remaining budget cannot safely cover another window plus final checks.
	approvedV3ProjectBlocks uint64 = 128
)

type approvedV3ScanResult struct {
	Attempted, Completed, Failed, Total int
	Skipped, Deferred                   int
	SharedReads, SourceReads            uint64 // marked first observations only, not all RPC traffic
}

// fairApprovedV3Scan dispatches every project at most once, starts after the
// next priority from the previous tick, and wraps on EVERY new tick. Complete
// rounds rotate the FIRST slot, not the last dispatched ID (which would reset
// a fully dispatched list forever). Partial rounds resume at the unstarted ID.
// This is an in-memory priority, never a durable scan high-water mark: IDs
// below after, new bindings and failed projects remain eligible forever.
// A canceled/partial tick cannot report success. All workers finish (including
// revocation) before the job releases its writer mutex.
func fairApprovedV3Scan(ctx context.Context, ids []uint64, after *uint64, budget time.Duration,
	scan func(context.Context, uint64) error, revoke func(uint64) error) (approvedV3ScanResult, error) {
	return fairApprovedV3ScanWorkers(ctx, ids, after, budget, approvedV3ScanWorkers, scan, revoke)
}
func fairApprovedV3ScanWorkers(ctx context.Context, ids []uint64, after *uint64, budget time.Duration, workerCount int,
	scan func(context.Context, uint64) error, revoke func(uint64) error) (approvedV3ScanResult, error) {
	return fairApprovedV3ScanWithStartBudget(ctx, ids, after, budget, budget, workerCount, scan, revoke)
}

// Stream admission can run with a smaller remaining slice than the maximum
// project timeout. The consumer independently reserves its atomic-window budget
// AFTER admission. Legacy full-range scans retain the full start reservation.
func fairApprovedV3ScanWithStartBudget(ctx context.Context, ids []uint64, after *uint64, budget, startBudget time.Duration, workerCount int,
	scan func(context.Context, uint64) error, revoke func(uint64) error) (approvedV3ScanResult, error) {
	result := approvedV3ScanResult{Total: len(ids)}
	invalid := errors.New("V3 approved fair scan incomplete")
	if ctx == nil || after == nil || scan == nil || revoke == nil || budget <= 0 || startBudget <= 0 || startBudget > budget || workerCount < 1 || workerCount > approvedV3ScanWorkers {
		return result, invalid
	}
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] >= id) {
			return result, invalid
		}
	}
	if len(ids) == 0 {
		return result, ctx.Err()
	}
	start := sort.Search(len(ids), func(i int) bool { return ids[i] > *after })
	if start == len(ids) {
		start = 0
	}
	type scanJob struct {
		id     uint64
		offset int
	}
	jobs := make(chan scanJob)
	started := make([]bool, len(ids))
	var workers sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				id := job.id
				// Check at worker start, not channel send. No authority or failure
				// is inferred for a project that did not run. Stream work has a
				// separate admission floor and post-admission consumer reservation.
				if !v3HasProjectBudget(ctx, startBudget) {
					continue
				}
				started[job.offset] = true
				projectCtx, cancel := context.WithTimeout(ctx, budget)
				projectCtx = newV3ScanTrace(projectCtx, id)
				err := scan(projectCtx, id)
				if err == nil {
					err = projectCtx.Err()
				}
				if err != nil {
					decision := "defer"
					if !v3Deferred(err) {
						decision = "persistent_revoke"
					}
					logV3ScanFailure(projectCtx, id, err, decision)
				}
				if err != nil && !v3Deferred(err) {
					// Preserve fail-closed publication. Only projects that actually
					// started work are revoked; unvisited ones have no new authority
					// and global freshness remains closed for this incomplete tick.
					if revokeErr := revoke(id); revokeErr != nil {
						logV3ScanFailure(projectCtx, id, revokeErr, "revoke_failed")
					} else {
						logV3ScanFailure(projectCtx, id, err, "revoked")
					}
				}
				cancel()
				mu.Lock()
				if err != nil {
					result.Failed++
					if v3Deferred(err) {
						result.Deferred++
					}
				} else {
					result.Completed++
				}
				mu.Unlock()
			}
		}()
	}
	dispatched := 0
dispatch:
	for dispatched < len(ids) {
		if ctx.Err() != nil {
			break
		}
		id := ids[(start+dispatched)%len(ids)]
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- scanJob{id: id, offset: dispatched}:
			dispatched++
		}
	}
	close(jobs)
	workers.Wait()
	lastStarted := -1
	for i, ran := range started {
		if ran {
			result.Attempted++
			lastStarted = i
		}
	}
	result.Skipped = len(ids) - result.Attempted
	if result.Attempted == len(ids) {
		// Every project got a full budget opportunity. Rotate even after
		// source failures; a persistently bad project cannot pin first place.
		*after = ids[start]
	} else if lastStarted >= 0 {
		*after = ids[(start+lastStarted)%len(ids)]
	}
	if ctx.Err() != nil || result.Completed != len(ids) || result.Failed != 0 {
		return result, invalid
	}
	return result, nil
}

func v3HasProjectBudget(ctx context.Context, budget time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	deadline, ok := ctx.Deadline()
	return !ok || time.Until(deadline) >= budget
}

func scanApprovedV3Fair(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	ownerHash, governorHash common.Hash, after *uint64) (approvedV3ScanResult, error) {
	// One sharing namespace per exact source/tick. Never reused across ticks,
	// endpoints, chains, or processes. All unmarked reads remain independent.
	shared := newV3TickReads(ctx, rpc)
	defer shared.close()
	rpc = shared
	var ids []uint64
	var cursor uint64
	for {
		page, err := idov3whitelist.ListApprovedV3ProjectIDs(ctx, db, cursor, 128)
		if err != nil {
			return approvedV3ScanResult{}, err
		}
		ids = append(ids, page...)
		if len(page) < 128 {
			break
		}
		cursor = page[len(page)-1]
		if cursor == math.MaxUint64 {
			return approvedV3ScanResult{}, errors.New("V3 scanner enumeration exhausted")
		}
	}
	result, err := fairApprovedV3Scan(ctx, ids, after, approvedV3ProjectBudget,
		func(projectCtx context.Context, id uint64) error {
			return scanApprovedV3Project(projectCtx, db, rpc, id, ownerHash, governorHash)
		}, func(id uint64) error {
			haltCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return haltApprovedV3Projection(haltCtx, db, id)
		})
	result.SharedReads, result.SourceReads = shared.shared.Load(), shared.upstream.Load()
	return result, err
}
