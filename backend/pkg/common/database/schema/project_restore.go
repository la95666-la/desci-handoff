package schema

// Project restore schema gate.
//
// POST /admin/v1/projects/:id/restore revives an archived project to its
// recorded pre-archive lifecycle status (pre_archive_status). Without the
// 2026_08_31 migration the restore endpoint can never infer a safe target, so
// core must refuse to start until the migration is applied to the database.
//
// The migration file lives at migrations/2026_08_31_restore_archived_project.sql
// and must be applied with the same ledger conventions as every other
// joincare migration (filename + sha256 checksum in schema_migrations).

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const projectRestoreMigration = "2026_08_31_restore_archived_project.sql"
const projectRestoreChecksum = "875dc1bcc914badbb81b2019fd32f544ab5d77d8a6e2ddee3744dab79809b0b3"

// RequireProjectRestore blocks core/job startup until the migration that
// records the pre-archive lifecycle status of projects is applied.
func RequireProjectRestore(database *gorm.DB) error {
	if database == nil {
		return errors.New("project restore schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	var ledger int64
	if err := database.Raw(`SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename=? AND checksum=?
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`, projectRestoreMigration, projectRestoreChecksum).Scan(&ledger).Error; err != nil {
		return fmt.Errorf("check project restore migration ledger: %w", err)
	}
	if ledger != 1 {
		return fmt.Errorf("project restore migration ledger incomplete: got=%d want=1; apply 2026_08_31_restore_archived_project.sql before starting core/job", ledger)
	}
	var columns int64
	if err := database.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME='projects' AND COLUMN_NAME='pre_archive_status' AND COLUMN_TYPE='varchar(50)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT=''`).Scan(&columns).Error; err != nil {
		return fmt.Errorf("check project restore column: %w", err)
	}
	if columns != 1 {
		return fmt.Errorf("projects.pre_archive_status column incomplete: got=%d want=1", columns)
	}
	return nil
}
