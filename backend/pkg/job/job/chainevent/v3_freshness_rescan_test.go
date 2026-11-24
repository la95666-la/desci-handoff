package chainevent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestRescanInProgressSuccessAndTemporaryFailureDoNotTouchCertificate(t *testing.T) {
	for _, result := range []error{nil, context.DeadlineExceeded, context.Canceled, rpc.HTTPError{StatusCode: 429}, &v3ScanDeferred{stage: "consumer_budget_yield"}, &v3ScanDeferred{stage: "audit_ready_yield"}, &v3ScanDeferred{stage: "freshness_publication_unavailable", cause: errors.New("SQL unavailable")}} {
		db, m := recoveryMockDB(t)
		ctx := context.WithValue(context.Background(), v3FreshnessKey{}, strings.Repeat("a", 64))
		// No SQL expectation: retaining means no DELETE/UPDATE/expiry renewal.
		if got := finishV3ProjectRescan(ctx, db, productionV3StreamScope(), 19, result); !reflect.DeepEqual(got, result) {
			t.Fatal(got, result)
		}
		if err := m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRescanIntegrityFailureInvalidatesExactProcessEvenAfterCancellation(t *testing.T) {
	for _, result := range []error{errV3StreamConflict, errors.New("identity mismatch"), fmt.Errorf("wrapped: %w", errV3StreamConflict)} {
		db, m := recoveryMockDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		token := strings.Repeat("b", 64)
		ctx = context.WithValue(ctx, v3FreshnessKey{}, token)
		s := productionV3StreamScope()
		m.ExpectExec("DELETE FROM ido_v3_project_freshness WHERE chain_id=\\? AND proxy_address=\\? AND contract_version=\\? AND project_id=\\? AND process_token=\\?").WithArgs(s.Chain, v3Lower(s.Proxy), v3StreamVersion, uint64(19), token).WillReturnResult(sqlmock.NewResult(0, 1))
		if got := finishV3ProjectRescan(ctx, db, s, 19, result); !errors.Is(got, result) {
			t.Fatal(got)
		}
		if err := m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRescanInvalidationErrorNeverConvertsConflictToTransient(t *testing.T) {
	db, m := recoveryMockDB(t)
	ctx := context.WithValue(context.Background(), v3FreshnessKey{}, strings.Repeat("a", 64))
	m.ExpectExec("DELETE FROM ido_v3_project_freshness").WillReturnError(context.DeadlineExceeded)
	err := finishV3ProjectRescan(ctx, db, productionV3StreamScope(), 19, errV3StreamConflict)
	if !errors.Is(err, errV3StreamConflict) || v3Deferred(err) {
		t.Fatal("revocation bypassed", err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerBudgetYieldKeepsOldCertificateWithoutEagerDelete(t *testing.T) {
	db, m := recoveryMockDB(t)
	s, f := streamTestFixture()
	b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: 7, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, v3FreshnessKey{}, strings.Repeat("a", 64))
	ctx = context.WithValue(ctx, v3ScheduledConsumerBudget{}, true)
	m.ExpectQuery("SELECT mode,state,anchor_block,cursor_block").WillReturnRows(sqlmock.NewRows([]string{"mode", "state", "anchor_block", "cursor_block", "cursor_hash", "revision", "epoch"}).AddRow("stream", "running", 90, 93, f.blocks[93].head.Hash().Hex(), 4, 1))
	m.ExpectQuery("SELECT cursor_block,cursor_hash,revision,state").WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(90, f.blocks[90].head.Hash().Hex(), 1, "running"))
	err := consumeV3Stream(ctx, db, f, s, b, s.Runtime, 3)
	if !v3SchedulingYield(err) {
		t.Fatal("budget yield lost", err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRescanStartNoLongerInvalidatesAndPublicationStillAtomic(t *testing.T) {
	for _, file := range []string{"v3_proxy_runtime.go", "v3_proxy_prepared_consumer.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "invalidateV3ProjectFreshness(") {
			t.Fatal("eager scan deletion restored", file)
		}
		if !strings.Contains(string(raw), "finishV3ProjectRescan(") {
			t.Fatal("missing integrity invalidation", file)
		}
	}
	raw, err := os.ReadFile("v3_project_freshness.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "func publishV3ProjectFreshness(")
	if start < 0 {
		t.Fatal("publication missing")
	}
	publication := text[start:]
	if !strings.Contains(publication, "ON DUPLICATE KEY UPDATE") || strings.Contains(publication, "DELETE FROM") || !strings.Contains(publication, "v3StreamWriterTx(") {
		t.Fatal("publication not single transactional replacement")
	}
}
