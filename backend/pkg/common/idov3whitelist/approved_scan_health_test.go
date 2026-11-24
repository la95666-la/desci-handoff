package idov3whitelist

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedScanHealthRequiresCompletedCurrentGeneration(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT chain_id FROM ido_v3_runtime").WithArgs(approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"chain_id"}).AddRow(56))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM ido_v3_project_binding").WithArgs(approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
	mock.ExpectExec("INSERT INTO ido_v3_approved_scan_health").WithArgs(approvedScanProxy, int64(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(7))
	mock.ExpectCommit()
	generation, err := BeginApprovedV3Scan(ctx, database)
	if err != nil || generation != 7 {
		t.Fatalf("begin: %d %v", generation, err)
	}
	// A crashed or partial scan stays checking, never fresh enough for proof.
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy, approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}))
	if RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("checking scan admitted")
	}
	// The old generation cannot mark a newer scan complete.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT chain_id FROM ido_v3_runtime").WithArgs(approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"chain_id"}).AddRow(56))
	mock.ExpectExec("UPDATE ido_v3_approved_scan_health SET state='healthy'").
		WithArgs(approvedScanProxy, uint64(6), approvedScanProxy).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if FinishApprovedV3Scan(ctx, database, 6) == nil {
		t.Fatal("old generation completed")
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT chain_id FROM ido_v3_runtime").WithArgs(approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"chain_id"}).AddRow(56))
	mock.ExpectExec("UPDATE ido_v3_approved_scan_health SET state='healthy'").
		WithArgs(approvedScanProxy, uint64(7), approvedScanProxy).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := FinishApprovedV3Scan(ctx, database, 7); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy, approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(7))
	if err := RequireFreshApprovedV3Scan(ctx, database); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy, approvedScanProxy).
		WillReturnError(errors.New("sensitive database diagnostic"))
	if err := RequireFreshApprovedV3Scan(ctx, database); err == nil || err.Error() == "sensitive database diagnostic" {
		t.Fatalf("DB failure not masked: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecentApprovedReviewScanDoesNotBecomeProofFreshness(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// SQL mock represents a checking tick with a recent completed full tick.
	// The review-only query admits it, but strict proof freshness still closes.
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy, approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(19))
	if err := RequireRecentApprovedV3ReviewScan(ctx, database); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy, approvedScanProxy).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}))
	if RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("review-only tick became proof freshness")
	}
	mock.ExpectQuery("SELECT generation FROM ido_v3_approved_scan_health").WithArgs(approvedScanProxy, approvedScanProxy).
		WillReturnError(errors.New("sensitive database diagnostic"))
	if e := RequireRecentApprovedV3ReviewScan(ctx, database); e == nil || e.Error() == "sensitive database diagnostic" {
		t.Fatalf("review query leaked DB details: %v", e)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
