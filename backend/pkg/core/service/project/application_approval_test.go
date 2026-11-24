package project

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestNewProjectFromApplicationInsertsValidJSON(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	application := entity.ApplyProject{ProjectName: "TESTCAT", ProjectDesc: "Draft application", WalletAddress: "0x6aaa882cce45ebf317a6342f317c6e92cba04f23"}
	project := newProjectFromApplication(application, 12345, now)
	if project.Status != constant.ProjectStatusSetup || project.OwnerAddress != application.WalletAddress || project.OnchainLocked {
		t.Fatalf("unexpected draft identity: %+v", project)
	}
	for label, value := range map[string]string{
		"team_json": project.TeamJSON, "tokenomics_json": project.TokenomicsJSON,
		"roadmap_json": project.RoadmapJSON, "documents_json": project.DocumentsJSON,
		"faq_json": project.FAQJSON, "onchain_json": project.OnchainJSON,
	} {
		if !json.Valid([]byte(value)) {
			t.Errorf("%s has invalid JSON: %q", label, value)
		}
	}
	// Use DryRun to inspect every generated bound value without a live database.
	stmt := db.Session(&gorm.Session{DryRun: true, SkipDefaultTransaction: true}).Create(&project).Statement
	for _, column := range []string{"team_json", "tokenomics_json", "roadmap_json", "documents_json", "faq_json", "onchain_json"} {
		if !strings.Contains(stmt.SQL.String(), "`"+column+"`") {
			t.Errorf("GORM did not include %s in INSERT", column)
		}
	}
	arrays, objects := 0, 0
	for _, value := range stmt.Vars {
		if value == "[]" {
			arrays++
		} else if value == "{}" {
			objects++
		}
	}
	if arrays < 5 || objects < 1 {
		t.Errorf("GORM JSON bound values: arrays=%d objects=%d", arrays, objects)
	}
	// The approval INSERT must not bind empty strings into nullable Fair Launch
	// raw columns: MySQL requires these to be NULL for capped_presale drafts.
	approval := createApplicationProject(db.Session(&gorm.Session{DryRun: true, SkipDefaultTransaction: true}), &project)
	if approval.Error != nil {
		t.Fatal(approval.Error)
	}
	if !strings.Contains(approval.Statement.SQL.String(), "INSERT INTO `projects`") {
		t.Fatalf("approval INSERT was not captured: %q", approval.Statement.SQL.String())
	}
	for _, column := range []string{"sale_token_amount_raw", "min_contribution_raw", "max_contribution_raw", "final_total_raised_raw", "final_sale_token_amount_raw"} {
		if strings.Contains(approval.Statement.SQL.String(), "`"+column+"`") {
			t.Errorf("approval INSERT must leave %s NULL: %s", column, approval.Statement.SQL.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNewProjectFromApplicationPreservesPublicSummaryOnly(t *testing.T) {
	application := entity.ApplyProject{
		ProjectName: "TESTCAT", ProjectDesc: "A <vaccine> & clinical study",
		ProjectCategory: "生物科技", AdditionalInfo: "internal reviewer note",
		WalletAddress: "0x6aaa882cce45ebf317a6342f317c6e92cba04f23",
	}
	project := newProjectFromApplication(application, 12345, time.Now())
	if project.Category != application.ProjectCategory || project.Description != application.ProjectDesc || project.Tagline != application.ProjectDesc {
		t.Fatalf("application summary not preserved in public project: category=%q description=%q tagline=%q", project.Category, project.Description, project.Tagline)
	}
	if project.StoryHTML != "<p>A &lt;vaccine&gt; &amp; clinical study</p>" {
		t.Fatalf("application story must be escaped HTML: %q", project.StoryHTML)
	}
	if strings.Contains(project.StoryHTML, application.AdditionalInfo) || project.ResearchHTML != "" {
		t.Fatal("internal notes or unsupported research claims leaked to public project")
	}
	if project.IsListed != 1 {
		t.Fatalf("newly approved projects must be publicly listed by default, got %d", project.IsListed)
	}
}
