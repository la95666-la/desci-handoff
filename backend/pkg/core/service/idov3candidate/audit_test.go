package idov3candidate

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestAuditCandidateRequiresCompleteDurableRowsWithoutServingMembers(t *testing.T) {
	scope := NewScope(31337, 41376991395446785, common.HexToAddress("0x4444444444444444444444444444444444444444"))
	tree, err := idov3whitelist.Build(scope, []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		memberCount int
		state       string
		badLeaf     bool
		badVersion  bool
		dbFailure   bool
		want        bool
	}{
		{name: "complete inert candidate", memberCount: 2, state: "candidate", want: true},
		{name: "incomplete member set", memberCount: 1, state: "candidate"},
		{name: "altered member leaf", memberCount: 2, state: "candidate", badLeaf: true},
		{name: "nonsequential version", memberCount: 2, state: "candidate", badVersion: true},
		{name: "no candidate returned", memberCount: 0, state: "absent"},
		{name: "database read unavailable", state: "absent", dbFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			columns := []string{"expected_version", "target_version", "root", "members_digest", "snapshot_digest", "member_count"}
			candidateRows := sqlmock.NewRows(columns)
			if tc.state != "absent" {
				expected := uint64(0)
				if tc.badVersion {
					expected = 1
				}
				candidateRows.AddRow(expected, 1, strings.ToLower(commitment.Root.Hex()), strings.ToLower(commitment.MembersDigest.Hex()), strings.ToLower(commitment.Digest.Hex()), 2)
			}
			snapshotRead := mock.ExpectQuery("FROM ido_v3_policy_snapshot s").WithArgs(uint64(9), uint64(31337), uint64(41376991395446785), strings.ToLower(scope.ProxyAddress.Hex()))
			if tc.dbFailure {
				snapshotRead.WillReturnError(errors.New("sensitive database diagnostics"))
			} else {
				snapshotRead.WillReturnRows(candidateRows)
			}
			if tc.state != "absent" && !tc.badVersion {
				rows := sqlmock.NewRows([]string{"leaf_index", "wallet_address", "leaf_hash"})
				for i, member := range tree.Members()[:tc.memberCount] {
					leaf := strings.ToLower(member.Leaf.Hex())
					if tc.badLeaf && i == 1 {
						leaf = "0x" + strings.Repeat("f", 64)
					}
					rows.AddRow(i, strings.ToLower(member.Wallet.Hex()), leaf)
				}
				mock.ExpectQuery(regexp.QuoteMeta("SELECT leaf_index,wallet_address,leaf_hash FROM ido_v3_policy_member WHERE snapshot_id=? ORDER BY leaf_index")).WithArgs(uint64(9)).WillReturnRows(rows)
			}
			mock.ExpectRollback()
			got, err := AuditCandidate(context.Background(), database, scope, 9)
			if (err == nil) != tc.want {
				t.Fatalf("audit result=%+v err=%v", got, err)
			}
			if tc.dbFailure && strings.Contains(err.Error(), "sensitive database diagnostics") {
				t.Fatal("raw database failure escaped audit boundary")
			}
			if tc.want && got != commitment {
				t.Fatalf("commitment changed: %+v", got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAuditCandidateRejectsInvalidScopeBeforeDatabase(t *testing.T) {
	for _, scope := range []idov3whitelist.Scope{NewScope(0, 1, common.HexToAddress("0x1111111111111111111111111111111111111111")), NewScope(1, 0, common.HexToAddress("0x1111111111111111111111111111111111111111")), NewScope(1, 1, common.Address{})} {
		if _, err := AuditCandidate(context.Background(), nil, scope, 1); err == nil {
			t.Fatal("bad audit scope accepted")
		}
	}
}
