package project

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestPolicyOperationsUseSpecificEvidenceNotProjectFreshness(t *testing.T) {
	for _, name := range []string{"v3_policy_review.go", "v3_policy_preview.go", "v3_approved_policy_status.go", "v3_policy_observation_scan.go", "v3_approved_policy_execution.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "idov3whitelist.RequireFreshApprovedV3Scan(") || strings.Contains(string(raw), "idov3whitelist.RequireRecentApprovedV3ReviewScan(") {
			t.Fatal("business gate still directly coupled to unrelated full enumeration", name)
		}
		if strings.Contains(string(raw), "RequireApprovedV3ProjectScan(") || !strings.Contains(string(raw), "requireV3PolicyEvidenceStore(") {
			t.Fatal("policy evidence boundary incorrect", name)
		}
	}
}

func TestEnabledProjectGateDoesNotFallbackAtBackendHelpers(t *testing.T) {
	old := viper.Get("idoV3ProjectFreshness.enabled")
	defer viper.Set("idoV3ProjectFreshness.enabled", old)
	viper.Set("idoV3ProjectFreshness.enabled", true)
	conn, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{func() error { return requireV3PolicyPreviewScan(context.Background(), db, true, 19) }, func() error { return requireV3ReviewOrStrictScan(context.Background(), db, true, 19) }, func() error { return requireV3ReviewOrStrictScan(context.Background(), db, false, 19) }} {
		m.ExpectQuery("SELECT c.project_id FROM ido_v3_scan_cursor").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
		if err := call(); err == nil {
			t.Fatal("missing target project evidence fell back")
		}
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
