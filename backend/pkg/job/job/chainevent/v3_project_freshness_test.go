package chainevent

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/idov3whitelist"
)

func expectFreshWriter(t *testing.T, m sqlmock.Sqlmock, s v3StreamCursor, scope v3StreamScope) {
	t.Helper()
	m.ExpectQuery("SELECT COALESCE").WillReturnRows(sqlmock.NewRows([]string{"marker"}).AddRow(0))
	m.ExpectExec("SET @jc_v3_stream_writer=1").WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectBegin()
	m.ExpectQuery("SELECT mode,state,anchor_block,cursor_block").WillReturnRows(sqlmock.NewRows([]string{"mode", "state", "anchor_block", "cursor_block", "cursor_hash", "revision", "epoch", "observed_head"}).AddRow(s.Mode, s.State, s.AnchorBlock, s.CursorBlock, s.CursorHash, s.Revision, s.Epoch, s.ObservedHead))
}
func finishFreshWriter(m sqlmock.Sqlmock, commit bool) {
	if commit {
		m.ExpectCommit()
	} else {
		m.ExpectRollback()
	}
	m.ExpectExec("SET @jc_v3_stream_writer=0").WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestProjectFreshnessPublishesOnlyAfterExactCursorAndOwnerFences(t *testing.T) {
	db, m := recoveryMockDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	expectAuditSnapshot(t, m, now)
	snapshot, err := readV3AuditSnapshot(context.Background(), db, 19)
	if err != nil {
		t.Fatal(err)
	}
	b := snapshot.candidate.Binding
	scope := productionV3StreamScope()
	s := v3StreamCursor{Mode: "stream", State: "running", AnchorBlock: 128805696, CursorBlock: snapshot.candidate.CursorBlock, CursorHash: strings.ToLower(snapshot.candidate.CursorHash.Hex()), Revision: 5, Epoch: 2, ObservedHead: snapshot.candidate.CursorBlock + 14}
	published := v3ConsumerCursor{CursorBlock: s.CursorBlock, CursorHash: s.CursorHash, Revision: snapshot.candidate.CursorRevision, State: "running"}
	h := &types.Header{Number: new(big.Int).SetUint64(s.CursorBlock)}
	s.CursorHash = h.Hash().Hex()
	// Pin target to fixture header bytes, independently of candidate cursor
	// hash. The exact current cursor/checkpoint join is validated separately.
	block := v3StreamBlock{Header: (*v3StreamHeader)(h)}
	raw, digest, err := v3StreamPayload(block)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), v3FreshnessKey{}, strings.Repeat("a", 64))
	ctx = context.WithValue(ctx, v3AuditGrantKey{}, v3AuditEntry{scope: v3AuditScope{epoch: 2, anchor: s.AnchorBlock}, snapshot: snapshot, completed: now.Add(-time.Minute), expires: now.Add(time.Minute)})
	ctx = context.WithValue(ctx, v3SharedConfirmedKey{}, v3SharedConfirmed{height: s.CursorBlock, hash: h.Hash()})
	expectFreshWriter(t, m, s, scope)
	m.ExpectQuery("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor").WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(published.CursorBlock, published.CursorHash, published.Revision, "running"))
	expectAuditSnapshot(t, m, now)
	m.ExpectQuery("SELECT p.wallets,p.state,project.status").WillReturnRows(sqlmock.NewRows([]string{"wallets", "state", "status"}).AddRow("{}", "pending", "Pending"))
	// Identity reader uses the same approved candidate; different argument
	// ordering of its final original-attempt read is intentional.
	expectAuditSnapshot(t, m, now)
	m.ExpectQuery("SELECT c.block_number,c.block_hash,c.parent_hash,c.digest,c.payload").WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "parent_hash", "digest", "payload"}).AddRow(s.CursorBlock, h.Hash().Hex(), h.ParentHash.Hex(), digest, raw))
	m.ExpectQuery("SELECT process_token,epoch,state FROM ido_v3_source_freshness").WillReturnRows(sqlmock.NewRows([]string{"process_token", "epoch", "state"}).AddRow(strings.Repeat("a", 64), 2, "healthy"))
	m.ExpectExec("INSERT INTO ido_v3_project_freshness").WillReturnResult(sqlmock.NewResult(1, 1))
	finishFreshWriter(m, true)
	if err := publishV3ProjectFreshness(ctx, db, scope, b, published, s); err != nil {
		t.Fatal(err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectFreshnessNoTokenOrBackfillCannotWriteCertificate(t *testing.T) {
	ctx := context.Background()
	if err := publishV3ProjectFreshness(ctx, nil, v3StreamScope{}, idov3whitelist.ExpectedProjectBinding{}, v3ConsumerCursor{}, v3StreamCursor{}); err != nil {
		t.Fatal(err)
	}
	ctx = context.WithValue(ctx, v3FreshnessKey{}, strings.Repeat("a", 64))
	ctx = context.WithValue(ctx, v3AuditGrantKey{}, v3AuditEntry{})
	ctx = context.WithValue(ctx, v3SharedConfirmedKey{}, v3SharedConfirmed{height: 100})
	if err := publishV3ProjectFreshness(ctx, nil, v3StreamScope{}, idov3whitelist.ExpectedProjectBinding{}, v3ConsumerCursor{CursorBlock: 99}, v3StreamCursor{CursorBlock: 100, CursorHash: common.HexToHash("0x11").Hex()}); err != nil {
		t.Fatal(err)
	}
}

func TestProjectFreshnessRestartReplacesProcessBeforeSourceHealthy(t *testing.T) {
	db, m := recoveryMockDB(t)
	j := &ApprovedV3ScannerJob{Database: db}
	old := viper.Get("idoV3ProjectFreshness.enabled")
	defer viper.Set("idoV3ProjectFreshness.enabled", old)
	viper.Set("idoV3ProjectFreshness.enabled", true)
	m.ExpectQuery("SELECT COUNT.*information_schema.TABLES").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	m.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	m.ExpectQuery("SELECT COUNT.*information_schema.TABLE_CONSTRAINTS").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(6))
	s := v3StreamCursor{Mode: "stream", State: "running", AnchorBlock: 100, CursorBlock: 200, CursorHash: common.HexToHash("0x22").Hex(), Revision: 5, Epoch: 2, ObservedHead: 214}
	expectFreshWriter(t, m, s, productionV3StreamScope())
	m.ExpectExec("INSERT INTO ido_v3_source_freshness.*'checking'").WillReturnResult(sqlmock.NewResult(1, 1))
	finishFreshWriter(m, true)
	ctx, err := j.prepareProjectFreshness(context.Background())
	if err != nil || len(j.freshness.token) != 64 || ctx.Value(v3FreshnessKey{}) != j.freshness.token {
		t.Fatal(err, j.freshness)
	}
	first := j.freshness.token
	ctx, err = j.prepareProjectFreshness(context.Background())
	if err != nil || j.freshness.token != first {
		t.Fatal("process token reset every tick")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectFreshnessMissingSchemaEnabledFailsClosed(t *testing.T) {
	db, m := recoveryMockDB(t)
	j := &ApprovedV3ScannerJob{Database: db}
	old := viper.Get("idoV3ProjectFreshness.enabled")
	defer viper.Set("idoV3ProjectFreshness.enabled", old)
	viper.Set("idoV3ProjectFreshness.enabled", true)
	m.ExpectQuery("SELECT COUNT.*information_schema.TABLES").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	if _, err := j.prepareProjectFreshness(context.Background()); err == nil || j.freshness.ready {
		t.Fatal("missing migration accepted")
	}
	viper.Set("idoV3ProjectFreshness.enabled", false)
	if _, err := j.prepareProjectFreshness(context.Background()); err != nil || j.freshness.token != "" {
		t.Fatal("disabled compatibility changed", err)
	}
}
