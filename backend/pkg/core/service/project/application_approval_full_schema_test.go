package project

import (
	"errors"
	"os"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/database/schema"
	"joincare_server/pkg/common/entity"
)

// Optional local-only test, supplied by the disposable MySQL schema runner.
// No real application is approved: this inserts a synthetic draft directly.
func TestApplicationApprovalInsertOnFullMigratedMySQL(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	if dsn == "" {
		t.Skip("requires owned disposable MySQL schema; see scripts/tests/ido_outbox_schema_disposable_test.sh")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	var count int64
	if err := database.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&count).Error; err != nil || count < 103 {
		t.Fatalf("requires pinned migrated schema: tables=%d err=%v", count, err)
	}
	if err := database.Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND CONSTRAINT_NAME='chk_projects_funding_raw_rules' AND ENFORCED='YES'`).Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("funding raw constraint missing: count=%d err=%v", count, err)
	}
	const projectID = 1784512099 // uniquely synthetic, disposable DB only
	if err := database.Model(&entity.Project{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("synthetic project already exists: count=%d err=%v", count, err)
	}
	draft := newProjectFromApplication(entity.ApplyProject{
		ProjectName: "DISPOSABLE APPLICATION CHECK", ProjectDesc: "local schema-only test",
		WalletAddress: "0x6aaa882cce45ebf317a6342f317c6e92cba04f23",
	}, projectID, time.Now().UTC())
	// Reproduce the original production failure before exercising the corrected
	// INSERT. A rejected statement must leave no synthetic project behind.
	unfixed := draft
	var mysqlErr *drivermysql.MySQLError
	if err := database.Create(&unfixed).Error; !errors.As(err, &mysqlErr) || mysqlErr.Number != 3819 {
		t.Fatalf("expected original CHECK 3819 from uncorrected INSERT; got %v", err)
	}
	if err := database.Model(&entity.Project{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed uncorrected INSERT left a project: count=%d err=%v", count, err)
	}
	if err := createApplicationProject(database, &draft).Error; err != nil {
		t.Fatalf("corrected approval INSERT rejected by real schema: %v", err)
	}
	var actual entity.Project
	if err := database.Where("project_id = ?", projectID).Take(&actual).Error; err != nil {
		t.Fatal(err)
	}
	if actual.ID == 0 || actual.Status != draft.Status || actual.OwnerAddress != draft.OwnerAddress || actual.FundingMode != "capped_presale" {
		t.Fatalf("synthetic draft identity mismatch: id=%d status=%q funding=%q", actual.ID, actual.Status, actual.FundingMode)
	}
	var invalidColumns int64
	if err := database.Raw(`SELECT COUNT(*) FROM projects WHERE project_id=? AND (sale_token_amount_raw IS NOT NULL OR min_contribution_raw IS NOT NULL OR max_contribution_raw IS NOT NULL OR final_total_raised_raw IS NOT NULL OR final_sale_token_amount_raw IS NOT NULL)`, projectID).Scan(&invalidColumns).Error; err != nil || invalidColumns != 0 {
		t.Fatalf("capped draft wrote Fair Launch-only raw columns: count=%d err=%v", invalidColumns, err)
	}
}

func TestIDOV2CoreStartupSchemaGatesOnFullMigratedMySQL(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	if dsn == "" {
		t.Skip("requires owned disposable MySQL schema")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	for _, gate := range []struct {
		name  string
		check func(*gorm.DB) error
	}{
		{"collation", schema.RequireChainEventCollationCompatibility},
		{"signing metadata", schema.RequireChainTxSigningMetadata},
		{"purchase entitlement", schema.RequireIDOPurchaseEntitlement},
		{"reconciliation evidence", schema.RequireIDOReconciliationEvidence},
		{"reorg evidence", schema.RequireChainEventReorgEvidence},
		{"reorg recovery scan", schema.RequireChainEventReorgRecoveryScan},
		{"reorg materialization", schema.RequireChainEventReorgMaterialization},
		{"reorg assessment", schema.RequireChainEventReorgAssessment},
		{"reorg prerequisite", schema.RequireChainEventReorgPrerequisite},
		{"reorg generation", schema.RequireChainEventReorgGeneration},
		{"V2-only policy", schema.RequireV2OnlySourcePolicy},
		{"security control plane", schema.RequireSecurityControlPlane},
		{"project restore", schema.RequireProjectRestore},
	} {
		t.Run(gate.name, func(t *testing.T) {
			if err := gate.check(database); err != nil {
				t.Fatal(err)
			}
		})
	}
}
