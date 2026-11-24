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

func testV3PriorityConsumerMySQL(t *testing.T, db *gorm.DB, rpc v3ScannerRPC, scope v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	rollback := errors.New("priority consumer rollback")
	before := readV3ConsumerFacts(t, db, b)
	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := context.WithValue(context.Background(), v3SharedBatchKey{}, true)
		result, err := runV3PriorityRound(ctx, []uint64{b.ProjectID}, &v3PrioritySchedule{}, time.Now,
			func() ([]v3PriorityHint, error) {
				return []v3PriorityHint{{ID: b.ProjectID, Status: "Active", Locked: true}}, nil
			}, func(uint64) bool { return false },
			func(c context.Context, id uint64) error { return consumeV3StreamWindows(c, tx, rpc, scope, b, rt) }, func(uint64) error { t.Fatal("normal Active history revoked"); return nil })
		if err != nil || result.Completed != 1 || result.Failed != 0 {
			t.Fatal(result, err)
		}
		cursor, err := readV3ConsumerCursor(tx, b, false)
		if err != nil || cursor.CursorBlock != 93 {
			t.Fatal(cursor, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, readV3ConsumerFacts(t, db, b)) {
		t.Fatal("priority execution escaped rollback")
	}
	t.Log("priority Active turn consumed canonical evidence; outer rollback retained all original facts")
}
