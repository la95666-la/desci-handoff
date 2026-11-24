package chainevent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Run by the network-none MySQL 8.4 rehearsal. Unlike sqlmock, this proves
// the database actually finds an EIP-55 sender using an approved lowercase
// identity while preserving the other exact lookup predicates.
func TestApprovedV3CreateAttemptsDisposableMySQLSenderCase(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL DSN unavailable")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name, version string
	if database.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
		database.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable MySQL")
	}
	const projectID uint64 = 42950728196620289
	const proxy = "0x0d4040875742975269c55e16c8884d3ee9510b8e"
	const checksummed = "0xfd712061ee08E070A930eCD45E935Ae5d7f5445C"
	const approved = "0xfd712061ee08e070a930ecd45e935ae5d7f5445c"
	ctx := context.Background()
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Shadow only this connection's real outbox in a temporary table.
		if err := tx.Exec(`CREATE TEMPORARY TABLE chain_tx_submission (
			id BIGINT UNSIGNED PRIMARY KEY, chain_id BIGINT UNSIGNED, project_id BIGINT UNSIGNED,
			contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin,
			sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin,
			action VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin,
			active_slot TINYINT, state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO chain_tx_submission VALUES
			(41,56,?,?,?,'ido_v3_create',1,'broadcast_unknown')`, projectID, proxy, checksummed).Error; err != nil {
			return err
		}
		var oldCount int64
		if err := tx.Raw(`SELECT COUNT(*) FROM chain_tx_submission WHERE project_id=?
			AND BINARY sender_address=BINARY ?`, projectID, approved).Scan(&oldCount).Error; err != nil || oldCount != 0 {
			t.Fatalf("original BINARY mismatch not reproduced: count=%d err=%v", oldCount, err)
		}
		rows, err := approvedV3CreateAttempts(ctx, tx, projectID, proxy, approved)
		if err != nil || len(rows) != 1 || rows[0].ID != 41 || rows[0].State != "broadcast_unknown" {
			t.Fatalf("original attempt not selected: rows=%v err=%v", rows, err)
		}
		for _, other := range []struct {
			project uint64
			proxy   string
			sender  string
		}{
			{projectID + 1, proxy, approved},
			{projectID, "0x1111111111111111111111111111111111111111", approved},
			{projectID, proxy, "0x1111111111111111111111111111111111111111"},
		} {
			miss, err := approvedV3CreateAttempts(ctx, tx, other.project, other.proxy, other.sender)
			if err != nil || len(miss) != 0 {
				t.Fatalf("wrong identity selected: rows=%v err=%v", miss, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The immutable approval stores lowercase, while the signed outbox row uses
// the EIP-55 checksummed spelling. Selection must normalize only the sender
// address, retaining the chain/project/proxy/action/active-slot fences.
func TestApprovedV3CreateAttemptsSelectsChecksummedSender(t *testing.T) {
	const projectID uint64 = 42950728196620289
	const proxy = "0x0d4040875742975269c55e16c8884d3ee9510b8e"
	operator := strings.ToLower(common.HexToAddress("0xfd712061ee08E070A930eCD45E935Ae5d7f5445C").Hex())
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`(?s)SELECT id,state FROM chain_tx_submission WHERE chain_id=56 AND project_id=\?.*BINARY contract_address=BINARY \? AND BINARY LOWER\(sender_address\)=BINARY \?.*BINARY action=BINARY 'ido_v3_create' AND active_slot=1.*state IN \('signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success'\)`).
		WithArgs(projectID, proxy, operator).
		WillReturnRows(sqlmock.NewRows([]string{"id", "state"}).AddRow(41, "broadcast_unknown"))
	got, err := approvedV3CreateAttempts(context.Background(), database, projectID, proxy, operator)
	if err != nil || len(got) != 1 || got[0].ID != 41 || got[0].State != "broadcast_unknown" {
		t.Fatalf("original signed attempt not selected: %v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovedScannerPageNeverSkipsFailedProject(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ids       []uint64
		wantError bool
	}{
		{name: "empty page"},
		{name: "failed first project", ids: []uint64{21, 24}, wantError: true},
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
			rows := sqlmock.NewRows([]string{"project_id"})
			for _, id := range tc.ids {
				rows.AddRow(id)
			}
			mock.ExpectQuery("SELECT b.project_id FROM ido_v3_project_binding b").
				WithArgs(uint64(20), "0x0d4040875742975269c55e16c8884d3ee9510b8e",
					2).
				WillReturnRows(rows)
			if tc.wantError {
				mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
				mock.ExpectQuery("SELECT s.cursor_block,s.revision,s.state,p.token_address,p.payment_token_address FROM ido_v3_scan_cursor s").
					WithArgs(uint64(21), "0x0d4040875742975269c55e16c8884d3ee9510b8e").
					WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "revision", "state"}))
				mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
				mock.ExpectQuery("SELECT s.cursor_block,s.revision,s.state,p.token_address,p.payment_token_address FROM ido_v3_scan_cursor s").
					WithArgs(uint64(24), "0x0d4040875742975269c55e16c8884d3ee9510b8e").
					WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "revision", "state"}))
			}
			// A DB admission failure must not reach the scanner RPC or advance past
			// project 21 to 24; the next page invocation may retry both safely.
			var rpc v3ScannerRPC = struct{ v3ScannerRPC }{}
			next, count, err := scanApprovedV3Page(context.Background(), database, rpc, 20, 2,
				common.HexToHash("0x01"), common.HexToHash("0x02"))
			wantCount := len(tc.ids)
			if next != (func() uint64 {
				if len(tc.ids) == 0 {
					return 20
				}
				return tc.ids[len(tc.ids)-1]
			}()) || count != wantCount || (err != nil) != tc.wantError {
				t.Fatalf("next=%d count=%d err=%v", next, count, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, _, err := scanApprovedV3Page(nil, nil, nil, 0, 129, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("unbounded page accepted")
	}
}

func TestApprovedScannerAllRestartsFromZeroOnEveryTick(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		mock.ExpectQuery("SELECT b.project_id FROM ido_v3_project_binding b").
			WithArgs(uint64(0), "0x0d4040875742975269c55e16c8884d3ee9510b8e",
				32).
			WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	}
	var rpc v3ScannerRPC = struct{ v3ScannerRPC }{}
	for i := 0; i < 2; i++ {
		if err := scanApprovedV3All(context.Background(), database, rpc,
			common.HexToHash("0x01"), common.HexToHash("0x02")); err != nil {
			t.Fatal(err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
