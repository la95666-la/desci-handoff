package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestPolicyObservationNoCertificateWait(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT c.project_id FROM ido_v3_scan_cursor").WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(19))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := requireV3PolicyEvidenceStore(ctx, database, 19); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyEvidenceStoreRejectsRecoveryAndCrossProject(t *testing.T) {
	for _, id := range []uint64{0, 20} {
		conn, m, e := sqlmock.New()
		if e != nil {
			t.Fatal(e)
		}
		db, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
		if e != nil {
			t.Fatal(e)
		}
		rows := sqlmock.NewRows([]string{"project_id"})
		if id != 0 {
			rows.AddRow(id)
		}
		m.ExpectQuery("SELECT c.project_id FROM ido_v3_scan_cursor.*c.state='running'.*chain_event_reorg_incident").WillReturnRows(rows)
		if requireV3PolicyEvidenceStore(context.Background(), db, 19) == nil {
			t.Fatal("missing/halted/reorg/foreign evidence admitted")
		}
		if e := m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
		conn.Close()
	}
}

func TestPolicyObservationScanDeadlineDoesNotBypassGate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := requireV3PolicyEvidenceStore(ctx, &gorm.DB{}, 19); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan returned %v", err)
	}
	if err := requireV3PolicyEvidenceStore(context.Background(), nil, 19); err == nil {
		t.Fatal("missing database passed scan gate")
	}
}
