package chaintx

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestLockV3RunningCursorRequiresCanonicalCheckpoint(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy}
	hash := "0x" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, state, checkpoint string
		present, want           bool
	}{
		{"missing cursor", "", "", false, false},
		{"halted", "reorg_halted", "", true, false},
		{"missing checkpoint", "running", "", true, false},
		{"wrong checkpoint", "running", "0x" + strings.Repeat("b", 64), true, false},
		{"canonical", "running", hash, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"})
			if tc.present {
				rows.AddRow(90, hash, 1, tc.state)
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), strings.ToLower(proxy.Hex()), uint64(7)).WillReturnRows(rows)
			if tc.state == "running" {
				checkpoint := sqlmock.NewRows([]string{"block_hash"})
				if tc.checkpoint != "" {
					checkpoint.AddRow(tc.checkpoint)
				}
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND block_hash=? AND status='canonical' LIMIT 2 FOR SHARE`)).WithArgs(uint64(31337), strings.ToLower(proxy.Hex()), uint64(7), uint64(90), hash).WillReturnRows(checkpoint)
			}
			if tc.want {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err := db.Transaction(func(tx *gorm.DB) error { return LockV3RunningCursorTx(tx, binding) })
			if (err == nil) != tc.want {
				t.Fatalf("cursor result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
