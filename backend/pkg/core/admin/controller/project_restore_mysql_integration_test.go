package controller

// MySQL integration tests for archive/restore:
//   POST /admin/v1/projects/:id/disable (archives + records pre-archive status)
//   POST /admin/v1/projects/:id/restore (revives only archived non-V2 projects)
//
// These tests run only when JOINCARE_MYSQL_TEST_DSN points at a disposable
// database (joincare_reorg_verify_* prefix) whose schema includes the
// 2026_08_31_restore_archived_project.sql migration.

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

func openRestoreVerificationMySQL(t *testing.T) *gorm.DB {
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
		t.Fatalf("refuse non-disposable project restore database %q: %v", name, err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	var columns int64
	if err := database.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='pre_archive_status'").Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if columns != 1 {
		t.Fatal("projects.pre_archive_status is missing; apply 2026_08_31_restore_archived_project.sql")
	}
	return database
}

func createRestoreFixture(t *testing.T, database *gorm.DB, status, contract string, onchain bool) uint64 {
	t.Helper()
	projectID := uint64(time.Now().UnixNano())
	lock := 0
	if onchain {
		lock = 1
	}
	const insert = "INSERT INTO projects (project_id,status,contract_version,onchain_locked,description,tags,team_info,roadmap,partners,start_time,end_time,tge_time) VALUES (?,?,?,?, '', '', '', '', '','2026-01-01 00:00:00', '2026-01-02 00:00:00', '2026-01-02 00:00:00')"
	if err := database.Exec(insert, projectID, status, contract, lock).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Exec("DELETE FROM projects WHERE project_id = ?", projectID) })
	return projectID
}

type restoreProjectRow struct {
	Status    string
	PreStatus string
}

func readRestoreProjectRow(t *testing.T, database *gorm.DB, projectID uint64) restoreProjectRow {
	t.Helper()
	var row restoreProjectRow
	if err := database.Raw("SELECT status, pre_archive_status AS pre_status FROM projects WHERE project_id = ?", projectID).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestArchiveRestoreRoundTripAgainstMySQL(t *testing.T) {
	database := openRestoreVerificationMySQL(t)

	t.Run("setup draft archives and restores to Setup", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "Setup", "unassigned", false)
		if _, err := archiveProject(database, int64(projectID)); err != nil {
			t.Fatal(err)
		}
		row := readRestoreProjectRow(t, database, projectID)
		if row.Status != "archived" || row.PreStatus != "Setup" {
			t.Fatalf("after archive: status=%q pre_archive=%q", row.Status, row.PreStatus)
		}
		restored, err := restoreProject(database, int64(projectID))
		if err != nil {
			t.Fatal(err)
		}
		if restored != "Setup" {
			t.Fatalf("restored status=%q, want Setup", restored)
		}
		row = readRestoreProjectRow(t, database, projectID)
		if row.Status != "Setup" || row.PreStatus != "" {
			t.Fatalf("after restore: status=%q pre_archive=%q", row.Status, row.PreStatus)
		}
	})

	t.Run("pre_launch draft archives and restores to pre_launch", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "pre_launch", "unassigned", false)
		if _, err := archiveProject(database, int64(projectID)); err != nil {
			t.Fatal(err)
		}
		restored, err := restoreProject(database, int64(projectID))
		if err != nil || restored != "pre_launch" {
			t.Fatalf("restored=%q err=%v", restored, err)
		}
	})

	t.Run("double archive is idempotent and keeps pre-archive status", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "warmup", "unassigned", false)
		if _, err := archiveProject(database, int64(projectID)); err != nil {
			t.Fatal(err)
		}
		if _, err := archiveProject(database, int64(projectID)); err != nil {
			t.Fatal(err)
		}
		row := readRestoreProjectRow(t, database, projectID)
		if row.Status != "archived" || row.PreStatus != "warmup" {
			t.Fatalf("after double archive: status=%q pre_archive=%q", row.Status, row.PreStatus)
		}
	})
}

func TestRestoreRejectionsAgainstMySQL(t *testing.T) {
	database := openRestoreVerificationMySQL(t)

	t.Run("legacy archive without recorded status is rejected", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "archived", "unassigned", false)
		// Simulate a project archived before the migration: wipe the recorded status.
		if err := database.Exec("UPDATE projects SET pre_archive_status = '' WHERE project_id = ?", projectID).Error; err != nil {
			t.Fatal(err)
		}
		_, err := restoreProject(database, int64(projectID))
		if !errors.Is(err, errProjectRestoreUnknownState) {
			t.Fatalf("expected unknown-state rejection, got %v", err)
		}
		if row := readRestoreProjectRow(t, database, projectID); row.Status != "archived" {
			t.Fatalf("rejected restore must not change the row: status=%q", row.Status)
		}
	})

	t.Run("non-archived project is rejected", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "pre_launch", "unassigned", false)
		if _, err := restoreProject(database, int64(projectID)); !errors.Is(err, errProjectRestoreNotArchived) {
			t.Fatalf("expected not-archived rejection, got %v", err)
		}
	})

	t.Run("recorded status outside allowlist is rejected", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "archived", "unassigned", false)
		if err := database.Exec("UPDATE projects SET pre_archive_status = 'Active' WHERE project_id = ?", projectID).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := restoreProject(database, int64(projectID)); !errors.Is(err, errProjectRestoreUnknownState) {
			t.Fatalf("expected allowlist rejection, got %v", err)
		}
	})

	t.Run("on-chain V2 locked project is rejected", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "archived", "v2_stablecoin", true)
		if err := database.Exec("UPDATE projects SET pre_archive_status = 'pre_launch' WHERE project_id = ?", projectID).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := restoreProject(database, int64(projectID)); !errors.Is(err, errProjectRestoreLifecycleLocked) {
			t.Fatalf("expected lifecycle-locked rejection, got %v", err)
		}
	})

	t.Run("unlocked Fair Launch cannot use generic archive or restore", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "Set", constant.ContractVersionFairLaunchV1, false)
		if _, err := archiveProject(database, int64(projectID)); !errors.Is(err, errProjectArchiveLifecycleLocked) {
			t.Fatalf("expected archive lifecycle-locked rejection, got %v", err)
		}
		if err := database.Exec("UPDATE projects SET status = 'archived', pre_archive_status = 'Set' WHERE project_id = ?", projectID).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := restoreProject(database, int64(projectID)); !errors.Is(err, errProjectRestoreLifecycleLocked) {
			t.Fatalf("expected restore lifecycle-locked rejection, got %v", err)
		}
	})

	t.Run("unlocked V3 cannot use generic archive or restore", func(t *testing.T) {
		projectID := createRestoreFixture(t, database, "Set", constant.ContractVersionIDOV3Stablecoin, false)
		if _, err := archiveProject(database, int64(projectID)); !errors.Is(err, errProjectArchiveLifecycleLocked) {
			t.Fatalf("expected V3 archive lifecycle rejection, got %v", err)
		}
		if err := database.Exec("UPDATE projects SET status = 'archived', pre_archive_status = 'Set' WHERE project_id = ?", projectID).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := restoreProject(database, int64(projectID)); !errors.Is(err, errProjectRestoreLifecycleLocked) {
			t.Fatalf("expected V3 restore lifecycle rejection, got %v", err)
		}
	})

	t.Run("immutable chain facts block restore", func(t *testing.T) {
		for _, table := range []string{"chain_tx_submission", "fund_ledger"} {
			t.Run(table, func(t *testing.T) {
				if err := database.Exec("DROP TEMPORARY TABLE IF EXISTS " + table).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Exec("CREATE TEMPORARY TABLE " + table + " (project_id BIGINT NOT NULL)").Error; err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { database.Exec("DROP TEMPORARY TABLE IF EXISTS " + table) })
				projectID := createRestoreFixture(t, database, "archived", "unassigned", false)
				if err := database.Exec("UPDATE projects SET pre_archive_status = 'Setup' WHERE project_id = ?", projectID).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Exec("INSERT INTO "+table+" (project_id) VALUES (?)", projectID).Error; err != nil {
					t.Fatal(err)
				}
				_, err := restoreProject(database, int64(projectID))
				if !errors.Is(err, errProjectRestoreImmutableFacts) {
					t.Fatalf("expected immutable-fact rejection, got %v", err)
				}
				if row := readRestoreProjectRow(t, database, projectID); row.Status != "archived" {
					t.Fatalf("rejected restore must not change the row: status=%q", row.Status)
				}
			})
		}
	})
}

func TestRestoreNotFoundAgainstMySQL(t *testing.T) {
	database := openRestoreVerificationMySQL(t)
	_, err := restoreProject(database, int64(time.Now().UnixNano()))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record-not-found, got %v", err)
	}
}
