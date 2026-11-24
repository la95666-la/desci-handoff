package chainevent

import (
	"context"
	"errors"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func auditFixture() (v3AuditSnapshot, v3AuditScope) {
	c := idov3whitelist.ApprovedLockedCandidate{Record: idov3whitelist.ApprovedSetRecord{Request: []byte(`{"review":1}`)}, SubmissionID: 42,
		CreationBlock: 100, CreationHash: common.HexToHash("0x01"), CreationTx: common.HexToHash("0x02"),
		CursorBlock: 110, CursorHash: common.HexToHash("0x03"), CursorRevision: 11}
	c.Binding.ProjectID = 19
	return v3AuditSnapshot{candidate: c, attempt: [32]byte{1}}, v3AuditScope{epoch: 1, anchor: 90, source: [32]byte{1}}
}

func TestPeriodicAuditFirstFullThenReuseDoesNotExtendExpiry(t *testing.T) {
	cache := &v3PeriodicAudit{}
	snapshot, scope := auditFixture()
	now := time.Now()
	clock := func() time.Time { return now }
	fulls, hots := 0, 0
	read := func() (v3AuditSnapshot, error) { return snapshot, nil }
	full := func() (v3AuditSnapshot, error) { fulls++; return snapshot, nil }
	hot := func(v3AuditSnapshot) error { hots++; return nil }
	_, mode, err := cache.check(context.Background(), 19, scope, clock, read, full, hot)
	if err != nil || mode != "full" || fulls != 1 || hots != 0 {
		t.Fatal(mode, err, fulls, hots)
	}
	expires := cache.entries[19].expires
	for i := 0; i < 10; i++ {
		now = now.Add(time.Second)
		snapshot.candidate.CursorBlock++
		snapshot.candidate.CursorRevision++
		snapshot.candidate.CursorHash = common.BigToHash(new(big.Int).SetUint64(snapshot.candidate.CursorBlock))
		_, mode, err = cache.check(context.Background(), 19, scope, clock, read, full, hot)
		if err != nil || mode != "reuse" || !cache.entries[19].expires.Equal(expires) {
			t.Fatal(mode, err)
		}
	}
	if fulls != 1 || hots != 10 {
		t.Fatal(fulls, hots)
	}
	now = expires
	_, mode, err = cache.check(context.Background(), 19, scope, clock, read, full, hot)
	if err != nil || mode != "full" || fulls != 2 {
		t.Fatal(mode, err, fulls)
	}
}

func TestPeriodicAuditChangeInvalidatesBeforeAnyHotReuse(t *testing.T) {
	for _, change := range []string{"epoch", "anchor", "source", "owner", "governor", "attempt", "approval", "receipt", "rewind", "revision", "hash", "runtime"} {
		t.Run(change, func(t *testing.T) {
			cache := &v3PeriodicAudit{}
			snapshot, scope := auditFixture()
			now := time.Now()
			read := func() (v3AuditSnapshot, error) { return snapshot, nil }
			_, _, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { t.Fatal("initial hit"); return nil })
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "epoch":
				scope.epoch++
			case "anchor":
				scope.anchor++
			case "source":
				scope.source[0]++
			case "owner":
				scope.owner = common.HexToHash("0xaa")
			case "governor":
				scope.governor = common.HexToHash("0xbb")
			case "attempt":
				snapshot.attempt[0]++
			case "approval":
				snapshot.candidate.Record.Request = []byte(`{"review":2}`)
			case "receipt":
				snapshot.candidate.CreationHash = common.HexToHash("0xab")
			case "rewind":
				snapshot.candidate.CursorBlock--
			case "revision":
				snapshot.candidate.CursorRevision++
			case "hash":
				snapshot.candidate.CursorHash = common.HexToHash("0xac")
			case "runtime":
				snapshot.candidate.Runtime.Implementation = common.HexToAddress("0xff")
			}
			_, mode, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read,
				func() (v3AuditSnapshot, error) { return v3AuditSnapshot{}, context.DeadlineExceeded },
				func(v3AuditSnapshot) error { t.Fatal("changed evidence reused"); return nil })
			if mode != "full" || err == nil || len(cache.entries) != 0 {
				t.Fatal(mode, err, cache.entries)
			}
		})
	}
}

func TestPeriodicAuditHotFailureExpiryAndDBFailureNeverFallback(t *testing.T) {
	for _, failure := range []string{"rpc", "db", "expiry", "clock_back", "changed_during_hot", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			cache := &v3PeriodicAudit{}
			snapshot, scope := auditFixture()
			now := time.Now()
			read := func() (v3AuditSnapshot, error) { return snapshot, nil }
			_, _, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failRead := read
			full := func() (v3AuditSnapshot, error) { return v3AuditSnapshot{}, errors.New("full failed") }
			hot := func(v3AuditSnapshot) error {
				switch failure {
				case "rpc":
					return context.DeadlineExceeded
				case "expiry":
					now = cache.entries[19].expires
				case "changed_during_hot":
					snapshot.attempt[0]++
				}
				return nil
			}
			if failure == "db" {
				failRead = func() (v3AuditSnapshot, error) { return v3AuditSnapshot{}, errors.New("DB unavailable") }
			}
			if failure == "clock_back" {
				now = now.Add(-time.Second)
			}
			if failure == "canceled" {
				cancel()
			}
			_, _, err = cache.check(ctx, 19, scope, func() time.Time { return now }, failRead, full, hot)
			if err == nil || len(cache.entries) != 0 {
				t.Fatal(err, cache.entries)
			}
		})
	}
}

func TestPeriodicAuditStaggerBoundedPrunedAndRestartCold(t *testing.T) {
	cache := &v3PeriodicAudit{}
	snapshot, scope := auditFixture()
	now := time.Now()
	var wg sync.WaitGroup
	for id := uint64(1); id <= 20; id++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			s := snapshot
			s.candidate.Binding.ProjectID = id
			read := func() (v3AuditSnapshot, error) { return s, nil }
			_, _, err := cache.check(context.Background(), id, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { return nil })
			if err != nil {
				t.Error(err)
			}
		}(id)
		if d := v3AuditLifetime(id); d > 10*time.Minute || d < 8*time.Minute {
			t.Fatal(d)
		}
	}
	wg.Wait()
	cache.retain([]uint64{1, 2})
	if len(cache.entries) != 2 {
		t.Fatal(len(cache.entries))
	}
	cache.forget(1)
	if len(cache.entries) != 1 {
		t.Fatal(len(cache.entries))
	}
	cold := &v3PeriodicAudit{}
	read := func() (v3AuditSnapshot, error) { return snapshot, nil }
	_, mode, err := cold.check(context.Background(), 19, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { t.Fatal("restart reused"); return nil })
	if err != nil || mode != "full" {
		t.Fatal(mode, err)
	}
}

func TestPeriodicAuditOnlyWiredToStreamScanner(t *testing.T) {
	for _, p := range []string{"../../../common/chaintx/v3_approved_locked_admission.go", "../../../common/idov3whitelist/approved_scan_health.go"} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "v3PeriodicAudit") || strings.Contains(string(raw), "v3AuditGrantKey") {
			t.Fatal("cache leaked to financial/freshness boundary", p)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := admitV3ScannerProject(ctx, nil, nil, 19, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("no cache path bypassed full audit")
	}
}

func TestPeriodicAuditGrantExpiresAndRechecksEvidenceBeforeCommit(t *testing.T) {
	for _, change := range []string{"none", "expired", "epoch", "halted", "project", "attempt", "approval", "revision", "canceled"} {
		t.Run(change, func(t *testing.T) {
			snapshot, scope := auditFixture()
			now := time.Now()
			grant := v3AuditEntry{scope: scope, snapshot: snapshot, completed: now.Add(-time.Minute), expires: now.Add(time.Minute)}
			b := snapshot.candidate.Binding
			stream := v3StreamCursor{Mode: "stream", State: "running", Epoch: scope.epoch, AnchorBlock: scope.anchor}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch change {
			case "expired":
				grant.expires = now.Add(-time.Second)
			case "epoch":
				stream.Epoch++
			case "halted":
				stream.State = "halted"
			case "project":
				b.ProjectID++
			case "attempt":
				snapshot.attempt[0]++
			case "approval":
				snapshot.candidate.Record.Request = []byte("changed")
			case "revision":
				snapshot.candidate.CursorRevision++
			case "canceled":
				cancel()
			}
			err := validateV3ScannerAuditGrant(ctx, grant, b, stream, func() (v3AuditSnapshot, error) { return snapshot, nil })
			if (err == nil) != (change == "none") {
				t.Fatal(change, err)
			}
		})
	}
	cache := &v3PeriodicAudit{entries: map[uint64]v3AuditEntry{19: {expires: time.Now().Add(time.Second)}}}
	ctx := context.WithValue(context.Background(), v3AuditContextKey{}, &v3AuditTurn{cache: cache})
	bounded, cancel := v3ScannerAuditDeadline(ctx, 19)
	defer cancel()
	deadline, ok := bounded.Deadline()
	if !ok || !deadline.Equal(cache.entries[19].expires) {
		t.Fatal("transaction may outlive audit expiry")
	}
}

func TestPeriodicAuditRefreshesBeforeWholeRoundCouldOutliveCertificate(t *testing.T) {
	snapshot, scope := auditFixture()
	now := time.Now()
	cache := &v3PeriodicAudit{entries: map[uint64]v3AuditEntry{19: {scope: scope, snapshot: snapshot, completed: now.Add(-time.Minute), expires: now.Add(40 * time.Second)}}}
	read := func() (v3AuditSnapshot, error) { return snapshot, nil }
	_, mode, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { t.Fatal("round may outlive certificate"); return nil })
	if err != nil || mode != "full" {
		t.Fatal(mode, err)
	}
}

func TestPeriodicAuditCapacityNeverReturnsExpirylessSuccess(t *testing.T) {
	snapshot, scope := auditFixture()
	now := time.Now()
	cache := &v3PeriodicAudit{entries: make(map[uint64]v3AuditEntry)}
	for id := uint64(1); id <= v3AuditEntries; id++ {
		cache.entries[id] = v3AuditEntry{}
	}
	read := func() (v3AuditSnapshot, error) { return snapshot, nil }
	_, _, err := cache.check(context.Background(), v3AuditEntries+1, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { return nil })
	if err == nil || !v3Deferred(err) || len(cache.entries) != v3AuditEntries {
		t.Fatal(err, len(cache.entries))
	}
}

func TestPeriodicAuditFailedPeriodicRefreshCannotUseOldBaseline(t *testing.T) {
	snapshot, scope := auditFixture()
	now := time.Now()
	cache := &v3PeriodicAudit{}
	read := func() (v3AuditSnapshot, error) { return snapshot, nil }
	_, _, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read, read, func(v3AuditSnapshot) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	now = cache.entries[19].expires.Add(-20 * time.Second)
	_, mode, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read, func() (v3AuditSnapshot, error) { return v3AuditSnapshot{}, context.DeadlineExceeded }, func(v3AuditSnapshot) error { t.Fatal("expired baseline used"); return nil })
	if err == nil || mode != "full" || len(cache.entries) != 0 {
		t.Fatal(mode, err)
	}
	cache.clear()
	if len(cache.entries) != 0 {
		t.Fatal("mode change retained cache")
	}
}

func TestPeriodicAuditFullDoesNotPublishAfterMidAuditEvidenceChange(t *testing.T) {
	snapshot, scope := auditFixture()
	now := time.Now()
	cache := &v3PeriodicAudit{}
	read := func() (v3AuditSnapshot, error) { return snapshot, nil }
	full := func() (v3AuditSnapshot, error) { snapshot.attempt[0]++; return snapshot, nil }
	_, _, err := cache.check(context.Background(), 19, scope, func() time.Time { return now }, read, full, func(v3AuditSnapshot) error { return nil })
	if err == nil || len(cache.entries) != 0 {
		t.Fatal("full audit mixed original-attempt evidence", err)
	}
}

func TestPeriodicAuditAttemptDigestIncludesNonJSONSigningEvidence(t *testing.T) {
	now := time.Now()
	slot := uint8(1)
	base := entity.ChainTxSubmission{ID: 42, RawTx: []byte{1}, CallData: []byte{2}, V3SignedAt: &now, State: "applied"}
	want, err := v3AuditAttemptDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*entity.ChainTxSubmission){
		func(r *entity.ChainTxSubmission) { r.RawTx = []byte{3} }, func(r *entity.ChainTxSubmission) { r.CallData = []byte{4} },
		func(r *entity.ChainTxSubmission) { r.ActiveSlot = &slot }, func(r *entity.ChainTxSubmission) { t := now.Add(time.Second); r.V3SignedAt = &t },
		func(r *entity.ChainTxSubmission) { r.ReceiptBlockHash = "changed" }, func(r *entity.ChainTxSubmission) { r.State = "v3_reorg_suspended" },
	} {
		changed := base
		edit(&changed)
		got, err := v3AuditAttemptDigest(changed)
		if err != nil || got == want {
			t.Fatal("signing evidence not bound", err)
		}
	}
}
