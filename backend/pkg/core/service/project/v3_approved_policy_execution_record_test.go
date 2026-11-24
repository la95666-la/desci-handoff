package project

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedPolicyExecutionRecordRequiresProjectSetBeforeReview(t *testing.T) {
	if _, err := readApprovedV3PolicyExecutionTx(nil, 19, 7); err == nil {
		t.Fatal("nil transaction accepted")
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	// No policy review or Safe hash read may precede the exact executed Set
	// join; a legacy offline attestation cannot be laundered through this path.
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	mock.ExpectRollback()
	tx := database.Begin()
	if _, err := readApprovedV3PolicyExecutionTx(tx, 19, 7); err == nil {
		t.Fatal("policy execution without four-eyes Set admitted")
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
