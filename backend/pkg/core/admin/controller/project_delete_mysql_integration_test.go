package controller

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openProjectDeleteVerificationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("JOINCARE_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("JOINCARE_MYSQL_TEST_DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || !strings.HasPrefix(name, "joincare_reorg_verify_") {
		t.Fatalf("refuse non-disposable project deletion database %q: %v", name, err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	for _, table := range []string{"project_updates", "project_subscribers", "project_watches", "project_whitelists", "project_files", "project_tge"} {
		var count int64
		if err := database.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name = ?", table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			if err := database.Exec("CREATE TEMPORARY TABLE " + table + " (project_id BIGINT NOT NULL)").Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	return database
}

func createDisposableDraft(t *testing.T, database *gorm.DB) int64 {
	t.Helper()
	projectID := time.Now().UnixNano()
	sql := "INSERT INTO projects (project_id,description,tags,team_info,roadmap,partners,start_time,end_time,tge_time) VALUES (?, '', '', '', '', '', '2026-01-01 00:00:00', '2026-01-02 00:00:00', '2026-01-02 00:00:00')"
	if err := database.Exec(sql, projectID).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Exec("DELETE FROM projects WHERE project_id = ?", projectID) })
	return projectID
}

func TestDeleteProjectDraftAgainstMySQL(t *testing.T) {
	database := openProjectDeleteVerificationMySQL(t)
	t.Run("empty unassigned draft is deleted", func(t *testing.T) {
		projectID := createDisposableDraft(t, database)
		if err := deleteProjectDraft(database, projectID); err != nil {
			t.Fatal(err)
		}
		var count int64
		if err := database.Table("projects").Where("project_id = ?", projectID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("project still exists count=%d err=%v", count, err)
		}
	})
	facts := []string{"chain_tx_submission", "transaction_record", "user_project", "user_claim_record", "user_refund_record", "fund_ledger", "ido_purchase_entitlement"}
	for _, table := range facts {
		t.Run("rejects immutable "+table, func(t *testing.T) {
			if err := database.Exec("DROP TEMPORARY TABLE IF EXISTS " + table).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Exec("CREATE TEMPORARY TABLE " + table + " (project_id BIGINT NOT NULL)").Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { database.Exec("DROP TEMPORARY TABLE IF EXISTS " + table) })
			projectID := createDisposableDraft(t, database)
			if err := database.Exec("INSERT INTO "+table+" (project_id) VALUES (?)", projectID).Error; err != nil {
				t.Fatal(err)
			}
			err := deleteProjectDraft(database, projectID)
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("immutable %s facts", table)) {
				t.Fatalf("fact-bearing project was not rejected: %v", err)
			}
			var count int64
			if queryErr := database.Table("projects").Where("project_id = ?", projectID).Count(&count).Error; queryErr != nil || count != 1 {
				t.Fatalf("project changed after rejection count=%d err=%v", count, queryErr)
			}
		})
	}
}
