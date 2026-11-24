package idov3whitelist

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
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedLockedCandidateRequiresCanonicalAppliedCreation(t *testing.T) {
	address := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	start := time.Unix(1800000000, 0).UTC().Format(time.RFC3339)
	request := struct {
		StartTime string `json:"start_time"`
	}{start}
	requestJSON, _ := json.Marshal(request)
	id := ApprovedSetIdentity{ProjectID: 19, ChainID: 56,
		Proxy:        "0x0d4040875742975269c55e16c8884d3ee9510b8e",
		ProjectToken: address("2"), TokenDecimals: 18,
		PaymentToken: "0x55d398326f99059ff775485246999027b3197955", PaymentDecimals: 18,
		ProjectOwner: address("4"), FeeReceiver: address("5"), FeeRate: 100,
		Operator:       "0xfd712061ee08e070a930ecd45e935ae5d7f5445c",
		RequestDigest:  fmt.Sprintf("0x%x", sha256.Sum256(requestJSON)),
		CreateCallHash: hash("b"), CreateDeadline: 1800000000,
		ApprovalID: 7, InitiatorID: 8, ApproverID: 9}
	digest, err := DigestApprovedSetIdentity(id)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(id)
	approvalJSON := fmt.Sprintf(`{"project_id":19,"params":%s}`, requestJSON)
	firstColumns := []string{"chain_id", "proxy_address", "token_address", "payment_token_address",
		"owner_address", "fee_receiver", "success_fee_rate", "token_decimals", "payment_token_decimals",
		"request_digest", "manifest_digest", "create_calldata_hash", "operator_address", "identity_json",
		"identity_digest", "approval_id", "initiator_id", "approver_id", "payload"}
	first := []driver.Value{uint64(56), id.Proxy, id.ProjectToken, id.PaymentToken, id.ProjectOwner,
		id.FeeReceiver, id.FeeRate, id.TokenDecimals, id.PaymentDecimals, id.RequestDigest,
		digest.Hex(), id.CreateCallHash, id.Operator, string(encoded), digest.Hex(),
		id.ApprovalID, id.InitiatorID, id.ApproverID, approvalJSON}
	secondColumns := []string{"block_number", "block_hash", "tx_hash", "log_index", "payload", "cursor_block", "cursor_hash",
		"revision", "state", "checkpoint_hash", "token_address", "payment_token_address",
		"owner_address", "fee_receiver", "success_fee_rate", "token_decimals", "payment_token_decimals",
		"create_calldata_hash", "sender_address", "submission_id", "receipt_block_number", "receipt_block_hash", "receipt_status"}
	second := []driver.Value{uint64(128805700), hash("c"), hash("d"), uint64(3), `{"Created":{}}`, uint64(128805710), hash("e"),
		uint64(2), "running", hash("e"), id.ProjectToken, id.PaymentToken, id.ProjectOwner,
		id.FeeReceiver, id.FeeRate, id.TokenDecimals, id.PaymentDecimals, id.CreateCallHash,
		id.Operator, uint64(99), uint64(128805700), hash("c"), uint64(1)}
	for _, tc := range []struct {
		name  string
		alter func([]driver.Value)
		count int
		want  bool
	}{
		{name: "exact bridged project", count: 1, want: true},
		{name: "checksummed original sender", count: 1, alter: func(v []driver.Value) {
			v[18] = common.HexToAddress(id.Operator).Hex()
		}, want: true},
		{name: "no canonical creation", count: 0},
		{name: "ambiguous creation", count: 2},
		{name: "empty event payload", count: 1, alter: func(v []driver.Value) { v[4] = "{}" }},
		{name: "halted cursor", count: 1, alter: func(v []driver.Value) { v[8] = "reorg_halted" }},
		{name: "orphaned cursor checkpoint", count: 1, alter: func(v []driver.Value) { v[9] = hash("f") }},
		{name: "foreign owner", count: 1, alter: func(v []driver.Value) { v[12] = address("a") }},
		{name: "changed calldata", count: 1, alter: func(v []driver.Value) { v[17] = hash("f") }},
		{name: "foreign sender", count: 1, alter: func(v []driver.Value) { v[18] = address("a") }},
		{name: "missing original submission", count: 1, alter: func(v []driver.Value) { v[19] = uint64(0) }},
		{name: "wrong receipt block", count: 1, alter: func(v []driver.Value) { v[20] = uint64(128805701) }},
		{name: "reverted receipt", count: 1, alter: func(v []driver.Value) { v[22] = uint64(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			database, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows(firstColumns).AddRow(first...))
			rows := sqlmock.NewRows(secondColumns)
			for i := 0; i < tc.count; i++ {
				v := append([]driver.Value(nil), second...)
				if tc.alter != nil {
					tc.alter(v)
				}
				rows.AddRow(v...)
			}
			mock.ExpectQuery("SELECT j.block_number,j.block_hash,j.tx_hash").WithArgs("applied", uint64(19), id.Proxy, "running").WillReturnRows(rows)
			result, e := ReadApprovedLockedCandidate(context.Background(), database, 19)
			if (e == nil) != tc.want {
				t.Fatalf("candidate=%+v err=%v", result, e)
			}
			if tc.want && (result.CreationBlock != 128805700 || result.Binding.ProjectToken != common.HexToAddress(id.ProjectToken)) {
				t.Fatalf("wrong candidate: %+v", result)
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
	// A halted cursor is recovery evidence only. The ordinary locked-project
	// reader cannot turn it into signer or wallet eligibility.
	t.Run("halted recovery is not ordinary admission", func(t *testing.T) {
		conn, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		for _, state := range []string{"reorg_halted", "running"} {
			mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows(firstColumns).AddRow(first...))
			values := append([]driver.Value(nil), second...)
			values[8] = "reorg_halted"
			attempt := "applied"
			if state == "reorg_halted" {
				attempt = "v3_reorg_suspended"
			}
			mock.ExpectQuery("SELECT j.block_number,j.block_hash,j.tx_hash").
				WithArgs(attempt, uint64(19), id.Proxy, state).WillReturnRows(sqlmock.NewRows(secondColumns).AddRow(values...))
		}
		if candidate, err := ReadApprovedHaltedCandidate(context.Background(), database, 19); err != nil || candidate.CursorBlock != 128805710 {
			t.Fatalf("halted recovery evidence rejected: %+v %v", candidate, err)
		}
		if _, err := ReadApprovedLockedCandidate(context.Background(), database, 19); err == nil {
			t.Fatal("halted cursor admitted to ordinary reader")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}
