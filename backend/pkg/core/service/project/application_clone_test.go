package project

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	coredb "joincare_server/pkg/core/db"
)

func TestCloneApprovedApplicationRejectsInvalidActorAndSource(t *testing.T) {
	if _, err := CloneApprovedApplication(110, 0); err == nil {
		t.Fatal("missing admin accepted")
	}
	for _, status := range []int8{constant.ApplyStatusPending, constant.ApplyStatusReject} {
		conn, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		original := coredb.Client.Mysql
		coredb.Client.Mysql = database
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `apply_project` WHERE id = ? LIMIT ? FOR UPDATE")).
			WithArgs(uint64(110), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "wallet_address"}).
			AddRow(uint64(110), status, "0x6aaa882cce45ebf317a6342f317c6e92cba04f23"))
		mock.ExpectRollback()
		_, err = CloneApprovedApplication(110, 1)
		coredb.Client.Mysql = original
		if check := mock.ExpectationsWereMet(); err == nil || check != nil {
			t.Fatalf("status %d: err=%v expectations=%v", status, err, check)
		}
		conn.Close()
	}
}

func TestCloneApprovedApplicationRejectsDuplicatePendingRequest(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	original := coredb.Client.Mysql
	coredb.Client.Mysql = database
	defer func() { coredb.Client.Mysql = original }()
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `apply_project` WHERE id =").WithArgs(uint64(110), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "wallet_address", "project_name"}).
			AddRow(uint64(110), constant.ApplyStatusAgree, "0x6aaa882cce45ebf317a6342f317c6e92cba04f23", "TESTCAT"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `apply_project`").WithArgs(constant.ApplyStatusPending,
		"0x6aaa882cce45ebf317a6342f317c6e92cba04f23", "TESTCAT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()
	if _, err := CloneApprovedApplication(110, 17); err == nil || !strings.Contains(err.Error(), "already pending") {
		t.Fatalf("duplicate request accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCloneApprovedApplicationRecordsAdminProvenanceAndDoesNotApprove(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	original := coredb.Client.Mysql
	coredb.Client.Mysql = database
	defer func() { coredb.Client.Mysql = original }()
	columns := []string{"id", "status", "wallet_address", "project_name", "project_desc", "additional_info", "created_at", "updated_at"}
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows(columns).AddRow(uint64(110), constant.ApplyStatusAgree,
			"0x6aaa882cce45ebf317a6342f317c6e92cba04f23", "TESTCAT", "V2 source project description", "source review note", time.Now(), time.Now())
	}
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `apply_project` WHERE id =").WithArgs(uint64(110), 1).WillReturnRows(rows())
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `apply_project`").WithArgs(constant.ApplyStatusPending,
		"0x6aaa882cce45ebf317a6342f317c6e92cba04f23", "TESTCAT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO `apply_project`").WillReturnResult(sqlmock.NewResult(111, 1))
	mock.ExpectCommit()
	id, err := CloneApprovedApplication(110, 17)
	if err != nil || id != 111 {
		t.Fatalf("clone: id=%d err=%v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminSubmittedApplicationCopyPreservesApplicationOnly(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	source := entity.ApplyProject{ID: 110, Status: constant.ApplyStatusAgree,
		ProjectName: "TESTCAT", ProjectDesc: "Original review description",
		WalletAddress:  "0x6aaa882cce45ebf317a6342f317c6e92cba04f23",
		AdditionalInfo: "Internal review note", RecordID: "old_record"}
	clone := adminSubmittedApplicationCopy(source, 17, now)
	if clone.ID != 0 || clone.Status != constant.ApplyStatusPending ||
		clone.ProjectName != source.ProjectName || clone.ProjectDesc != source.ProjectDesc ||
		clone.WalletAddress != source.WalletAddress || clone.RecordID == source.RecordID ||
		len(clone.RecordID) > 50 || !strings.HasPrefix(clone.RecordID, "admin_clone_") ||
		!strings.Contains(clone.AdditionalInfo, "approved application #110 by admin #17") ||
		!strings.Contains(clone.AdditionalInfo, "applicant wallet has not submitted or signed this copy") ||
		!clone.CreatedAt.Equal(now) || !clone.UpdatedAt.Equal(now) {
		t.Fatalf("invalid admin-submitted application copy: %+v", clone)
	}
	if source.ID != 110 || source.Status != constant.ApplyStatusAgree || source.AdditionalInfo != "Internal review note" {
		t.Fatal("source mutated")
	}
}
