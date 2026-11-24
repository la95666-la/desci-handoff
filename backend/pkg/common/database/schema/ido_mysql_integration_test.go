package schema

import (
	"os"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openIDOVerificationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("JOINCARE_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("JOINCARE_MYSQL_TEST_DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect integration MySQL: %v", err)
	}
	var databaseName string
	if err := database.Raw("SELECT DATABASE()").Scan(&databaseName).Error; err != nil {
		t.Fatalf("read integration MySQL database name: %v", err)
	}
	if !strings.HasPrefix(databaseName, "joincare_reorg_verify_") {
		t.Fatalf("refuse non-disposable integration MySQL database %q", databaseName)
	}
	return database
}

// TestIDOSchemaGatesAgainstMySQL runs only when JOINCARE_MYSQL_TEST_DSN is
// explicitly provided. It validates the same fail-closed startup gates used by
// core and job against a real MySQL 8 schema without making this an implicit
// dependency of the normal unit-test suite.
func TestIDOSchemaGatesAgainstMySQL(t *testing.T) {
	database := openIDOVerificationMySQL(t)

	gates := []struct {
		name string
		run  func(*gorm.DB) error
	}{
		{name: "chain event collation compatibility", run: RequireChainEventCollationCompatibility},
		{name: "purchase entitlement", run: RequireIDOPurchaseEntitlement},
		{name: "reconciliation evidence", run: RequireIDOReconciliationEvidence},
		{name: "reorg evidence", run: RequireChainEventReorgEvidence},
		{name: "reorg recovery scan", run: RequireChainEventReorgRecoveryScan},
		{name: "reorg materialization", run: RequireChainEventReorgMaterialization},
		{name: "reorg assessment", run: RequireChainEventReorgAssessment},
		{name: "reorg prerequisite", run: RequireChainEventReorgPrerequisite},
		{name: "reorg generation", run: RequireChainEventReorgGeneration},
		{name: "V2-only source policy", run: RequireV2OnlySourcePolicy},
		{name: "security control plane", run: RequireSecurityControlPlane},
	}
	for _, gate := range gates {
		t.Run(gate.name, func(t *testing.T) {
			if err := gate.run(database); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestChainEventReorgGenerationGateAgainstMySQL isolates the generation gate
// so additive migrations that intentionally extend its protected tables can be
// validated without conflating unrelated historical attestation rows.
func TestChainEventReorgGenerationGateAgainstMySQL(t *testing.T) {
	database := openIDOVerificationMySQL(t)
	if err := RequireChainEventReorgGeneration(database); err != nil {
		t.Fatal(err)
	}
}

func TestIDOSchemaNegativeGuardsAgainstMySQL(t *testing.T) {
	database := openIDOVerificationMySQL(t)

	t.Run("assessment preparing slot cannot be null", func(t *testing.T) {
		if err := database.Exec("DROP TEMPORARY TABLE IF EXISTS tmp_reorg_assessment_job_guard").Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Exec("CREATE TEMPORARY TABLE tmp_reorg_assessment_job_guard LIKE chain_event_reorg_assessment_job").Error; err != nil {
			t.Fatal(err)
		}
		err := database.Exec(`INSERT INTO tmp_reorg_assessment_job_guard (
run_id,incident_id,state,active_slot,next_effect_ordinal,disposition_count,exact_count,conditional_count,blocked_count,noop_count,
blocker_count,disposition_digest,blocker_digest,lease_token,lease_expires_at,fence_epoch,attempt_count,last_error_code,
last_error_message,completed_at,updated_at
) VALUES (1,1,'preparing',NULL,1,0,0,0,0,0,0,REPEAT('0',64),REPEAT('0',64),'',NULL,0,0,'','',NULL,CURRENT_TIMESTAMP(6))`).Error
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "check constraint") {
			t.Fatalf("preparing assessment with NULL active slot was not rejected by CHECK: %v", err)
		}
	})

	t.Run("prerequisite preparing slot cannot be null", func(t *testing.T) {
		if err := database.Exec("DROP TEMPORARY TABLE IF EXISTS tmp_reorg_prerequisite_job_guard").Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Exec("CREATE TEMPORARY TABLE tmp_reorg_prerequisite_job_guard LIKE chain_event_reorg_prerequisite_job").Error; err != nil {
			t.Fatal(err)
		}
		err := database.Exec(`INSERT INTO tmp_reorg_prerequisite_job_guard (
run_id,incident_id,state,active_slot,next_requirement_ordinal,requirement_count,unmet_count,untrusted_count,deferred_count,
artifact_count,requirement_digest,artifact_digest,lease_token,lease_expires_at,fence_epoch,attempt_count,last_error_code,
last_error_message,completed_at,updated_at
) VALUES (1,1,'preparing',NULL,1,0,0,0,0,0,REPEAT('0',64),REPEAT('0',64),'',NULL,0,0,'','',NULL,CURRENT_TIMESTAMP(6))`).Error
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "check constraint") {
			t.Fatalf("preparing prerequisite with NULL active slot was not rejected by CHECK: %v", err)
		}
	})

	t.Run("recovery definition rows remain append only", func(t *testing.T) {
		var before int64
		if err := database.Model(&struct{}{}).Table("chain_event_reorg_recovery_definition").Count(&before).Error; err != nil {
			t.Fatal(err)
		}
		if before != 34 {
			t.Fatalf("definition row count=%d want=34", before)
		}
		tx := database.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		defer tx.Rollback()
		if err := tx.Exec("UPDATE chain_event_reorg_recovery_definition SET created_at=created_at WHERE id=(SELECT id FROM (SELECT MIN(id) id FROM chain_event_reorg_recovery_definition) x)").Error; err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("definition update was not rejected: %v", err)
		}
		if err := tx.Exec("DELETE FROM chain_event_reorg_recovery_definition WHERE id=(SELECT id FROM (SELECT MIN(id) id FROM chain_event_reorg_recovery_definition) x)").Error; err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("definition delete was not rejected: %v", err)
		}
		if err := tx.Rollback().Error; err != nil {
			t.Fatalf("rollback negative definition writes: %v", err)
		}
		var after int64
		if err := database.Model(&struct{}{}).Table("chain_event_reorg_recovery_definition").Count(&after).Error; err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("definition row count changed after rejected writes: before=%d after=%d", before, after)
		}
	})

	t.Run("assessment foreign keys are restrict restrict", func(t *testing.T) {
		var count int64
		if err := database.Raw(`SELECT COUNT(DISTINCT CONCAT(rc.TABLE_NAME,'|',rc.CONSTRAINT_NAME))
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
JOIN information_schema.KEY_COLUMN_USAGE k ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME AND k.TABLE_NAME=rc.TABLE_NAME
WHERE rc.CONSTRAINT_SCHEMA=DATABASE() AND rc.CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_assessment_'
AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE() AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
AND rc.UPDATE_RULE='RESTRICT' AND rc.DELETE_RULE='RESTRICT'`).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 10 {
			t.Fatalf("assessment RESTRICT/RESTRICT foreign key count=%d want=10", count)
		}
	})

	t.Run("prerequisite foreign keys are same schema restrict restrict", func(t *testing.T) {
		var count int64
		if err := database.Raw(`SELECT COUNT(DISTINCT CONCAT(rc.TABLE_NAME,'|',rc.CONSTRAINT_NAME))
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
JOIN information_schema.KEY_COLUMN_USAGE k ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME AND k.TABLE_NAME=rc.TABLE_NAME
WHERE rc.CONSTRAINT_SCHEMA=DATABASE() AND rc.CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_prerequisite_'
AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE() AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
AND rc.UPDATE_RULE='RESTRICT' AND rc.DELETE_RULE='RESTRICT'`).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 9 {
			t.Fatalf("prerequisite same-schema RESTRICT/RESTRICT foreign key count=%d want=9", count)
		}
	})
}
