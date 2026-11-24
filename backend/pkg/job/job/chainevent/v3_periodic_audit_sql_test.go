package chainevent

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/idov3whitelist"
)

func expectAuditSnapshot(t *testing.T, mock sqlmock.Sqlmock, signedAt time.Time) {
	t.Helper()
	address := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	request, _ := json.Marshal(struct {
		StartTime string `json:"start_time"`
	}{time.Unix(1800000000, 0).UTC().Format(time.RFC3339)})
	id := idov3whitelist.ApprovedSetIdentity{ProjectID: 19, ChainID: 56, Proxy: "0x0d4040875742975269c55e16c8884d3ee9510b8e", ProjectToken: address("2"), TokenDecimals: 18,
		PaymentToken: "0x55d398326f99059ff775485246999027b3197955", PaymentDecimals: 18, ProjectOwner: address("4"), FeeReceiver: address("5"), FeeRate: 100,
		Operator: "0xfd712061ee08e070a930ecd45e935ae5d7f5445c", RequestDigest: fmt.Sprintf("0x%x", sha256.Sum256(request)), CreateCallHash: hash("b"), CreateDeadline: 1800000000, ApprovalID: 7, InitiatorID: 8, ApproverID: 9}
	digest, err := idov3whitelist.DigestApprovedSetIdentity(id)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(id)
	columns := []string{"chain_id", "proxy_address", "token_address", "payment_token_address", "owner_address", "fee_receiver", "success_fee_rate", "token_decimals", "payment_token_decimals", "request_digest", "manifest_digest", "create_calldata_hash", "operator_address", "identity_json", "identity_digest", "approval_id", "initiator_id", "approver_id", "payload"}
	values := []driver.Value{56, id.Proxy, id.ProjectToken, id.PaymentToken, id.ProjectOwner, id.FeeReceiver, id.FeeRate, 18, 18, id.RequestDigest, digest.Hex(), id.CreateCallHash, id.Operator, string(encoded), digest.Hex(), 7, 8, 9, fmt.Sprintf(`{"project_id":19,"params":%s}`, request)}
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
	columns = []string{"block_number", "block_hash", "tx_hash", "log_index", "payload", "cursor_block", "cursor_hash", "revision", "state", "checkpoint_hash", "token_address", "payment_token_address", "owner_address", "fee_receiver", "success_fee_rate", "token_decimals", "payment_token_decimals", "create_calldata_hash", "sender_address", "submission_id", "receipt_block_number", "receipt_block_hash", "receipt_status"}
	values = []driver.Value{128805700, hash("c"), hash("d"), 3, `{"Created":{}}`, 128805710, hash("e"), 11, "running", hash("e"), id.ProjectToken, id.PaymentToken, id.ProjectOwner, id.FeeReceiver, id.FeeRate, 18, 18, id.CreateCallHash, id.Operator, 99, 128805700, hash("c"), 1}
	mock.ExpectQuery("SELECT j.block_number,j.block_hash,j.tx_hash").WithArgs("applied", uint64(19), id.Proxy, "running").WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
	mock.ExpectQuery("SELECT \\* FROM chain_tx_submission WHERE id=\\? AND project_id=\\? AND chain_id=\\?").WithArgs(uint64(99), uint64(19), uint64(56), id.Proxy).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "chain_id", "action", "state", "raw_tx", "call_data", "v3_signed_at"}).AddRow(99, 19, 56, "ido_v3_create", "applied", []byte{1, 2}, []byte{3, 4}, signedAt))
}

func TestPeriodicAuditReadsFreshExecutedApprovalAndOriginalAttempt(t *testing.T) {
	database, mock := recoveryMockDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	expectAuditSnapshot(t, mock, now)
	first, err := readV3AuditSnapshot(context.Background(), database, 19)
	if err != nil || first.candidate.SubmissionID != 99 || first.candidate.Binding.ProjectID != 19 {
		t.Fatal(first, err)
	}
	expectAuditSnapshot(t, mock, now.Add(time.Second))
	changed, err := readV3AuditSnapshot(context.Background(), database, 19)
	if err != nil || sameV3AuditEvidence(first, changed) {
		t.Fatal("changed signing time reused", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	// Missing executed Set is a failure before original attempt or RPC reuse.
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	if _, err := readV3AuditSnapshot(context.Background(), database, 19); err == nil {
		t.Fatal("missing approval admitted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPeriodicAuditTransactionGrantBindsActualFreshSQLSnapshot(t *testing.T) {
	database, mock := recoveryMockDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	expectAuditSnapshot(t, mock, now)
	snapshot, err := readV3AuditSnapshot(context.Background(), database, 19)
	if err != nil {
		t.Fatal(err)
	}
	grant := v3AuditEntry{scope: v3AuditScope{epoch: 2, anchor: 128805696, proxy: common.HexToAddress(snapshot.candidate.Record.Identity.Proxy)}, snapshot: snapshot, completed: time.Now().Add(-time.Minute), expires: time.Now().Add(time.Minute)}
	ctx := context.WithValue(context.Background(), v3AuditGrantKey{}, grant)
	stream := v3StreamCursor{Mode: "stream", State: "running", Epoch: 2, AnchorBlock: 128805696}
	expectAuditSnapshot(t, mock, now)
	if err := recheckV3ScannerAuditGrant(ctx, database, snapshot.candidate.Binding, stream); err != nil {
		t.Fatal("valid exact grant refused", err)
	}
	expectAuditSnapshot(t, mock, now.Add(time.Second))
	if err := recheckV3ScannerAuditGrant(ctx, database, snapshot.candidate.Binding, stream); err == nil {
		t.Fatal("changed signed evidence passed writer grant")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
