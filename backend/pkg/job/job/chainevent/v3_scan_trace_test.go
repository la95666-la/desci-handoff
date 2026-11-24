package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	drivermysql "github.com/go-sql-driver/mysql"
)

func TestV3TracePreservesInvestmentStageWithoutPayloadOrWallet(t *testing.T) {
	ctx := newV3ScanTrace(context.Background(), 43368658176704510)
	v3TraceWindow(ctx, 100, 7, 2, 101, 228)
	v3TraceBlock(ctx, 101, 1, 1, 1)
	v3TracePhase(ctx, "journal_projection_match")
	v3TraceSource(ctx, "eth_call", errors.New("https://private/SECRET calldata wallet"))
	v3TraceDB(ctx, &drivermysql.MySQLError{Number: 1213, Message: "SECRET SQL"})
	f := v3ScanLogFields(ctx, 43368658176704510, errors.New("SECRET"), "persistent_revoke")
	raw, e := json.Marshal(f)
	if e != nil {
		t.Fatal(e)
	}
	if f["phase"] != "journal_projection_match" || f["purchase_count"] != 1 || f["block_number"] != uint64(101) || f["project_id"] != "43368658176704510" || f["db_error_number"] != uint16(1213) {
		t.Fatal(f)
	}
	if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "private/") {
		t.Fatal(string(raw))
	}
}

func TestV3TraceConcurrentRPCFailuresAndProjectIsolation(t *testing.T) {
	ctx := newV3ScanTrace(context.Background(), 1)
	other := newV3ScanTrace(context.Background(), 2)
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			v3TraceSource(ctx, "eth_getBlockByNumber", context.DeadlineExceeded)
			_ = v3ScanLogFields(ctx, 1, context.DeadlineExceeded, "defer")
		}()
	}
	workers.Wait()
	if v3ScanLogFields(ctx, 1, context.DeadlineExceeded, "defer")["source_error_kind"] != "deadline" {
		t.Fatal("source kind missing")
	}
	if v3ScanLogFields(other, 2, nil, "complete")["rpc_method"] != "" {
		t.Fatal("cross-project diagnostic leak")
	}
}
