package project

import (
	"fmt"
	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPolicyHandoffDisposableMySQLApproval(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL DSN not set")
	}
	db, e := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable DB")
	}
	// MySQL cannot reopen a TEMPORARY table under the supersession self-join.
	// Use a separately named disposable schema, never production tables.
	isolated := fmt.Sprintf("jc_v3_audit_handoff_%d", time.Now().UnixNano())
	if e := db.Exec("CREATE DATABASE " + isolated).Error; e != nil {
		t.Fatal(e)
	}
	defer db.Exec("DROP DATABASE " + isolated)
	cfg, e := drivermysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal("DSN parse failed")
	}
	cfg.DBName = isolated
	local, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("isolated schema unavailable")
	}
	sqlDB, _ := local.DB()
	defer sqlDB.Close()
	e = local.Transaction(func(tx *gorm.DB) error {
		for _, sql := range []string{
			`CREATE TEMPORARY TABLE ido_v3_policy_review(id BIGINT UNSIGNED,project_id BIGINT UNSIGNED,chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42),contract_version VARCHAR(32),preview_digest VARCHAR(66),initiator_admin_id BIGINT UNSIGNED,target_version BIGINT UNSIGNED,expires_at DATETIME(6))`,
			`CREATE TEMPORARY TABLE ido_v3_policy_review_decision(review_id BIGINT UNSIGNED,chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42),project_id BIGINT UNSIGNED,target_version BIGINT UNSIGNED,decision VARCHAR(16),approver_admin_id BIGINT UNSIGNED,contract_version VARCHAR(32) DEFAULT 'ido_v3_stablecoin')`,
			`CREATE TEMPORARY TABLE ido_v3_policy_review_execution(review_id BIGINT UNSIGNED)`,
		} {
			sql = strings.Replace(sql, "CREATE TEMPORARY TABLE", "CREATE TABLE", 1)
			if e := tx.Exec(sql).Error; e != nil {
				return e
			}
		}
		// CREATE TABLE implicitly committed MySQL's original transaction.
		// Start the DML rehearsal after all isolated-schema DDL is installed.
		if e := tx.Exec("START TRANSACTION").Error; e != nil {
			return e
		}
		p, _ := lightHandoffFixture(t)
		if e := tx.Exec(`INSERT INTO ido_v3_policy_review VALUES(12,7,56,?,'ido_v3_stablecoin',?,8,1,UTC_TIMESTAMP(6)+INTERVAL 1 HOUR)`, p.Proxy, p.PreviewDigest).Error; e != nil {
			return e
		}
		if e := tx.Exec(`INSERT INTO ido_v3_policy_review_decision(review_id,chain_id,proxy_address,project_id,target_version,decision,approver_admin_id) VALUES(12,56,?,7,1,'approved',9)`, p.Proxy).Error; e != nil {
			return e
		}
		if e := validateV3HandoffDecisionTx(tx, p, 12, 7); e != nil {
			return fmt.Errorf("valid approval refused: %w", e)
		}
		for _, sql := range []string{
			`UPDATE ido_v3_policy_review SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND`,
			`UPDATE ido_v3_policy_review_decision SET approver_admin_id=8`,
			`UPDATE ido_v3_policy_review_decision SET target_version=2`,
			`UPDATE ido_v3_policy_review SET preview_digest=CONCAT('0x',REPEAT('f',64))`,
			`INSERT INTO ido_v3_policy_review_execution VALUES(12)`,
			`INSERT INTO ido_v3_policy_review SELECT 13,project_id,chain_id,proxy_address,contract_version,preview_digest,initiator_admin_id,2,expires_at FROM ido_v3_policy_review WHERE id=12`,
		} {
			if e := tx.SavePoint("invalid_variant").Error; e != nil {
				return e
			}
			if e := tx.Exec(sql).Error; e != nil {
				return e
			}
			if strings.Contains(sql, "SELECT 13") {
				if e := tx.Exec(`INSERT INTO ido_v3_policy_review_decision(review_id,chain_id,proxy_address,project_id,target_version,decision,approver_admin_id) VALUES(13,56,?,7,2,'approved',9)`, p.Proxy).Error; e != nil {
					return e
				}
			}
			if e := validateV3HandoffDecisionTx(tx, p, 12, 7); e == nil {
				return fmt.Errorf("invalid approval exported: %s", sql)
			}
			if e := tx.RollbackTo("invalid_variant").Error; e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
