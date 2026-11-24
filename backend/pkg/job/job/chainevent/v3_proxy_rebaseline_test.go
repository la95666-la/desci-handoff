package chainevent

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
	"testing"
)

func testV3TestRetirementBaselineMySQL(t *testing.T, db *gorm.DB, rpc *v3EmptyWindowRPC, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) {
	t.Helper()
	ctx := context.Background()
	s := v3StreamScope{Chain: b.ChainID, Proxy: b.ProxyAddress, Runtime: rt, Confirmations: 3}
	rollback := errors.New("baseline rehearsal rollback")
	latest := rpc.latest
	rpc.latest = 95
	defer func() { rpc.latest = latest }()
	e := db.Transaction(func(tx *gorm.DB) error {
		if e := bootstrapV3Stream(ctx, tx, rpc, s, "shadow", 90, rpc.blocks[90].head.Hash()); e != nil {
			t.Fatal(e)
		}
		if e := rebaselineV3Tests(ctx, tx, rpc, s, 1, []uint64{b.ProjectID + 1}, 93, rpc.blocks[93].head.Hash()); e == nil {
			t.Fatal("wrong retirement list accepted")
		}
		if e := rebaselineV3Tests(ctx, tx, rpc, s, 1, []uint64{b.ProjectID}, 93, rpc.blocks[93].head.Hash()); e != nil {
			t.Fatalf("rebaseline %v", e)
		}
		c, e := v3StreamCursorRead(tx, s, false)
		if e != nil || c.Mode != "stream" || c.Epoch != 2 || c.AnchorBlock != 93 || c.CursorBlock != 93 {
			t.Fatal(c, e)
		}
		blocks, e := readV3Coverage(tx, s, 93, 93)
		if e != nil || len(blocks[93].RetiredTests) != 1 || blocks[93].RetiredTests[0] != b.ProjectID {
			t.Fatal("retirement receipt missing", e)
		}
		if _, e = readV3Coverage(tx, s, 90, 93); e == nil {
			t.Fatal("skipped history claimed covered")
		}
		var state string
		tx.Raw(`SELECT status FROM projects WHERE project_id=?`, b.ProjectID).Scan(&state)
		if state != "archived" {
			t.Fatal(state)
		}
		var count int64
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_project_projection WHERE project_id=?`, b.ProjectID).Scan(&count)
		if count != 0 {
			t.Fatal("retired authority remains")
		}
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE project_id=?`, b.ProjectID).Scan(&count)
		if count != 1 {
			t.Fatal("old facts deleted")
		}
		if e = rebaselineV3Tests(ctx, tx, rpc, s, 1, []uint64{b.ProjectID}, 93, rpc.blocks[93].head.Hash()); e == nil {
			t.Fatal("retirement replayed")
		}
		return rollback
	})
	if !errors.Is(e, rollback) {
		t.Fatal(e)
	}
}

func TestRetiredV3DigestExactOrderingAndScope(t *testing.T) {
	a := RetiredV3TestDigest([]uint64{1, 2})
	if len(a) != 64 || a == RetiredV3TestDigest([]uint64{2, 1}) || a == RetiredV3TestDigest([]uint64{1}) {
		t.Fatal("unbound list")
	}
	if e := OperatorV3TestRebaseline(context.Background(), nil, nil, 1, []uint64{1}, 1, common.Hash{}, "wrong"); e == nil {
		t.Fatal("wrong confirmation accepted")
	}
}
