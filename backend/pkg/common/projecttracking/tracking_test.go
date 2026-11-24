package projecttracking

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

func TestTrackingReadUsesDatabaseUTCAndNoRowDefaultsNormal(t *testing.T) {
	conn, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m.ExpectQuery("SELECT t.*next_scan_at<=UTC_TIMESTAMP").WithArgs(uint64(19)).WillReturnRows(sqlmock.NewRows([]string{"project_id", "mode", "revision", "due"}))
	row, err := Read(context.Background(), db, 19, false)
	if err != nil || row.Mode != "normal" || !row.Due {
		t.Fatal(row, err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackingUTCWallClockSerialization(t *testing.T) {
	local := time.Date(2026, 10, 30, 12, 34, 56, 0, time.FixedZone("app", 8*3600))
	data, err := json.Marshal(Row{LastScanAt: &local})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"last_scan_at":"2026-10-30T12:34:56Z"`) || strings.Contains(string(data), "+08:00") {
		t.Fatal(string(data))
	}
}
