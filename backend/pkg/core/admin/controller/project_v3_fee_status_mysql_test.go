package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Execute the actual read query in a separate disposable schema. MySQL cannot
// reopen a TEMPORARY checkpoint table twice, so use ordinary fixture tables.
// Refuse production DSNs and never touch the shared fixture schema/data.
func TestV3FeeStatusDisposableMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL only")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable MySQL")
	}
	err = db.Connection(func(tx *gorm.DB) error {
		exec := func(q string, args ...any) {
			t.Helper()
			if e := tx.Exec(q, args...).Error; e != nil {
				t.Fatal(e)
			}
		}
		fixture := fmt.Sprintf("jc_v3_audit_fee_%d", time.Now().UnixNano())
		exec("CREATE DATABASE " + fixture)
		defer tx.Exec("DROP DATABASE " + fixture) // only our new disposable schema
		exec("USE " + fixture)
		defer tx.Exec("USE " + name)
		definitions := map[string]string{
			"projects":                  "project_id BIGINT UNSIGNED,chain_id BIGINT,contract_address VARCHAR(42),contract_version VARCHAR(32),funding_mode VARCHAR(32),onchain_locked INT,payment_token_address VARCHAR(42),fee_receiver VARCHAR(42)",
			"ido_v3_project_binding":    "project_id BIGINT UNSIGNED,chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32)",
			"ido_v3_scan_cursor":        "project_id BIGINT UNSIGNED,chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),cursor_block BIGINT,cursor_hash VARCHAR(66),revision BIGINT,state VARCHAR(16)",
			"ido_v3_project_projection": "project_id BIGINT UNSIGNED,chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),cursor_block BIGINT,cursor_hash VARCHAR(66),revision BIGINT,state VARCHAR(16),fee_paid_raw VARCHAR(78)",
			"ido_v3_block_checkpoint":   "project_id BIGINT UNSIGNED,chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),block_number BIGINT,block_hash VARCHAR(66),status VARCHAR(16)",
			"ido_v3_event_journal":      "id BIGINT,project_id BIGINT UNSIGNED,chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),block_number BIGINT,block_hash VARCHAR(66),tx_hash VARCHAR(66),log_index INT,event_kind VARCHAR(40),status VARCHAR(16)",
			"ido_v3_financial_effect":   "journal_id BIGINT,project_id BIGINT UNSIGNED,chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),block_number BIGINT,block_hash VARCHAR(66),tx_hash VARCHAR(66),log_index INT,entry_kind VARCHAR(40),asset_role VARCHAR(16),status VARCHAR(16),token_address VARCHAR(42),wallet_address VARCHAR(42),amount_raw VARCHAR(78)",
		}
		for table, columns := range definitions {
			exec("CREATE TABLE " + table + " (" + columns + ")")
		}
		const projectID uint64 = 43368660559134718
		proxy := "0x0d4040875742975269c55e16c8884d3ee9510b8e"
		version := "ido_v3_stablecoin"
		token, receiver := "0x"+strings.Repeat("1", 40), "0x"+strings.Repeat("2", 40)
		blockHash, txHash := "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64)
		exec("INSERT INTO projects VALUES(?,56,?,?,'capped_presale',1,?,?)", projectID, proxy, version, token, receiver)
		exec("INSERT INTO ido_v3_project_binding VALUES(?,56,?,?)", projectID, proxy, version)
		exec("INSERT INTO ido_v3_scan_cursor VALUES(?,56,?,?,100,?,7,'running')", projectID, proxy, version, blockHash)
		exec("INSERT INTO ido_v3_project_projection VALUES(?,56,?,?,100,?,7,'successful','10800000000000000')", projectID, proxy, version, blockHash)
		exec("INSERT INTO ido_v3_block_checkpoint VALUES(?,56,?,?,100,?,'canonical')", projectID, proxy, version, blockHash)
		exec("INSERT INTO ido_v3_event_journal VALUES(237,?,56,?,?,100,?,?,1,'PlatformFeeWithdrawn','canonical')", projectID, proxy, version, blockHash, txHash)
		exec("INSERT INTO ido_v3_financial_effect VALUES(237,?,56,?,?,100,?,?,1,'PlatformFeeWithdrawn','payment','canonical',?,?,'10800000000000000')", projectID, proxy, version, blockHash, txHash, token, receiver)
		check := func(want bool) {
			t.Helper()
			result, e := readV3PlatformFeeWithdrawn(context.Background(), tx, []uint64{projectID})
			if e != nil || result[projectID] != want {
				t.Fatal("withdrawal display", result, e, want)
			}
		}
		check(true)
		exec("UPDATE ido_v3_project_projection SET fee_paid_raw='0'")
		exec("UPDATE ido_v3_financial_effect SET amount_raw='0'")
		check(true) // zero-fee execution still hides button
		for _, tc := range []struct{ mutate, restore string }{
			{"UPDATE ido_v3_event_journal SET status='orphaned'", "UPDATE ido_v3_event_journal SET status='canonical'"},
			{"UPDATE ido_v3_financial_effect SET status='orphaned'", "UPDATE ido_v3_financial_effect SET status='canonical'"},
			{"UPDATE ido_v3_block_checkpoint SET status='orphaned'", "UPDATE ido_v3_block_checkpoint SET status='canonical'"},
			{"UPDATE ido_v3_scan_cursor SET state='reorg_halted'", "UPDATE ido_v3_scan_cursor SET state='running'"},
			{"UPDATE ido_v3_project_projection SET revision=8", "UPDATE ido_v3_project_projection SET revision=7"},
			{"UPDATE ido_v3_financial_effect SET wallet_address='0x3333333333333333333333333333333333333333'", "UPDATE ido_v3_financial_effect SET wallet_address='" + receiver + "'"},
			{"UPDATE ido_v3_financial_effect SET amount_raw='1'", "UPDATE ido_v3_financial_effect SET amount_raw='0'"},
		} {
			exec(tc.mutate)
			check(false)
			exec(tc.restore)
			check(true)
		}
		result, e := readV3PlatformFeeWithdrawn(context.Background(), tx, []uint64{projectID + 1})
		if e != nil || len(result) != 0 {
			t.Fatal("foreign project", result, e)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
