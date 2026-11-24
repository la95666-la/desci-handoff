package chainevent

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
)

// Actual MySQL writer path must reject expired/changed scanner grants without
// advancing journal, financial projection, checkpoint, outbox or consumer tip.
// Positive production-chain evidence reuse is covered by the exact SQL reader
// test; this fixture intentionally cannot impersonate production chain 56.
func testPeriodicAuditGrantMySQL(t *testing.T, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	before := readV3ConsumerFacts(t, db, b)
	for _, kind := range []string{"expired", "epoch", "foreign_binding"} {
		now := time.Now()
		grant := v3AuditEntry{scope: v3AuditScope{epoch: 1, anchor: 90}, snapshot: v3AuditSnapshot{candidate: idov3whitelist.ApprovedLockedCandidate{Binding: b}}, completed: now.Add(-time.Minute), expires: now.Add(time.Minute)}
		switch kind {
		case "expired":
			grant.expires = now.Add(-time.Second)
		case "epoch":
			grant.scope.epoch = 999
		case "foreign_binding":
			grant.snapshot.candidate.Binding.ProjectID++
		}
		ctx := context.WithValue(context.Background(), v3AuditGrantKey{}, grant)
		if err := consumeV3Stream(ctx, db, rpc, s, b, rt, 3); err == nil {
			t.Fatal(kind, "grant accepted")
		}
		if after := readV3ConsumerFacts(t, db, b); !reflect.DeepEqual(before, after) {
			t.Fatal(kind, "invalid certificate changed business facts")
		}
	}
	rollback := errors.New("audit failed writer rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		c, e := v3StreamCursorRead(tx, s, true)
		if e != nil {
			return e
		}
		grant := v3AuditEntry{scope: v3AuditScope{epoch: c.Epoch, anchor: c.AnchorBlock}, snapshot: v3AuditSnapshot{candidate: idov3whitelist.ApprovedLockedCandidate{Binding: b}}, completed: time.Now().Add(-time.Minute), expires: time.Now().Add(-time.Second)}
		ctx := context.WithValue(context.Background(), v3AuditGrantKey{}, grant)
		if err := recheckV3ScannerAuditGrant(ctx, tx, b, c); err == nil {
			t.Fatal("expired grant passed transaction-side fence")
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if after := readV3ConsumerFacts(t, db, b); !reflect.DeepEqual(before, after) {
		t.Fatal("transaction fence changed facts")
	}
	t.Log("periodic audit: expired/changed grant refused before consumer and under writer lock; all business facts unchanged")
}
