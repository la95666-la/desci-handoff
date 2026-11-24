package idov3whitelist

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func freshMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	conn, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db, m
}

func TestProjectFreshnessExactSnapshotCannotBorrowGlobalOrAnotherProject(t *testing.T) {
	for _, kind := range []string{"valid", "missing", "foreign", "db_error"} {
		t.Run(kind, func(t *testing.T) {
			db, m := freshMock(t)
			id := uint64(9007199254740999)
			c := ApprovedLockedCandidate{CursorBlock: 100, CursorHash: common.HexToHash("0x11"), CursorRevision: 7}
			digest := strings.Repeat("a", 64)
			expect := m.ExpectQuery("SELECT f.project_id FROM ido_v3_project_freshness f").WithArgs(approvedScanProxy, id, digest, uint64(100), strings.ToLower(c.CursorHash.Hex()), uint64(7), int64(-300), int64(-90), int64(-90), uint64(660), uint64(135))
			rows := sqlmock.NewRows([]string{"project_id"})
			if kind == "valid" {
				rows.AddRow(id)
			}
			if kind == "foreign" {
				rows.AddRow(id + 1)
			}
			if kind == "db_error" {
				expect.WillReturnError(errors.New("SECRET provider"))
			} else {
				expect.WillReturnRows(rows)
			}
			err := requireProjectFreshnessSnapshot(context.Background(), db, id, c, digest)
			if (err == nil) != (kind == "valid") || err != nil && strings.Contains(err.Error(), "SECRET") {
				t.Fatal(err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectFreshnessEnabledHasNoLegacyFallback(t *testing.T) {
	old := viper.Get("idoV3ProjectFreshness.enabled")
	defer viper.Set("idoV3ProjectFreshness.enabled", old)
	viper.Set("idoV3ProjectFreshness.enabled", true)
	db, m := freshMock(t)
	m.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	if err := RequireApprovedV3ProjectScan(context.Background(), db, 19, false); err == nil {
		t.Fatal("missing project proof admitted")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := RequireApprovedV3ProjectScan(context.Background(), db, 0, true); err == nil {
		t.Fatal("missing project ID fell back")
	}
	viper.Set("idoV3ProjectFreshness.enabled", false)
	m.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(1))
	if err := RequireApprovedV3ProjectScan(context.Background(), db, 19, false); err != nil {
		t.Fatal("disabled compatibility changed", err)
	}
}

func TestFiveMinuteProjectAllowanceAndTTLKeepIndependentSourceLimits(t *testing.T) {
	if ProjectFreshnessMaxLag != 660 || SourceFreshnessMaxHeadLag != 135 || SourceFreshnessMaxConfirmedLag != 120 || ProjectFreshnessMaxAge != 300*time.Second || SourceFreshnessMaxAge != 90*time.Second {
		t.Fatal("independent source/age limits changed")
	}
}

func TestTrackingOperationRequestsCatchupButNeverAdmitsStaleEvidence(t *testing.T) {
	old := viper.Get("idoV3ProjectTracking.enabled")
	defer viper.Set("idoV3ProjectTracking.enabled", old)
	viper.Set("idoV3ProjectTracking.enabled", true)
	db, m := freshMock(t)
	m.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	m.ExpectExec("UPDATE ido_v3_project_tracking SET wake_block=GREATEST").WithArgs(uint64(19)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := RequireFreshApprovedV3Project(context.Background(), db, 19); err == nil {
		t.Fatal("queued sync admitted original request")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
