package project

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedCreateExecutionRejectsMissingGuardAndUnsignedAdmission(t *testing.T) {
	if _, err := executeApprovedV3Create(19, nil); err == nil {
		t.Fatal("create accepted without administrator mutation guard")
	}
	if err := checkApprovedV3SignedContinuation(context.Background(), nil, 19, 1,
		idov3whitelist.ApprovedSetIdentity{}, idov3whitelist.ExpectedProjectBinding{}); err == nil {
		t.Fatal("signed row admitted without database")
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
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	if err := checkApprovedV3SignedContinuation(context.Background(), database, 19, 1,
		idov3whitelist.ApprovedSetIdentity{}, idov3whitelist.ExpectedProjectBinding{}); err == nil {
		t.Fatal("signed row admitted without executed four-eyes Set")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
