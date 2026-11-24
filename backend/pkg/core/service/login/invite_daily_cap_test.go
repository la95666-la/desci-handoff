package login

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
)

func inviteCapTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}

func TestUTCDayWindow(t *testing.T) {
	start, end := utcDayWindow(time.Date(2026, 9, 25, 7, 8, 9, 0, time.FixedZone("UTC+8", 8*60*60)))
	if got := start.Format(time.RFC3339); got != "2026-09-24T00:00:00Z" {
		t.Fatalf("start=%s", got)
	}
	if got := end.Format(time.RFC3339); got != "2026-09-25T00:00:00Z" {
		t.Fatalf("end=%s", got)
	}
}

func TestInviteDailyCapDefaultsWhenMissingOrInvalid(t *testing.T) {
	for _, value := range []string{"", "0", "100001", "not-a-number"} {
		database, mock := inviteCapTestDB(t)
		rows := sqlmock.NewRows([]string{"value"})
		if value != "" {
			rows.AddRow(value)
		}
		mock.ExpectQuery("SELECT .*value.* FROM `settings` WHERE `key` = .* LIMIT .* FOR SHARE").
			WithArgs(constant.InviteDailyCapSettingKey, 1).
			WillReturnRows(rows)
		cap, err := inviteDailyCap(database)
		if err != nil || cap != constant.InviteDailyCapDefault {
			t.Fatalf("value=%q cap=%d err=%v", value, cap, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInviteDailyCapReadsConfiguredValue(t *testing.T) {
	database, mock := inviteCapTestDB(t)
	mock.ExpectQuery("SELECT .*value.* FROM `settings` WHERE `key` = .* LIMIT .* FOR SHARE").
		WithArgs(constant.InviteDailyCapSettingKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("25"))
	cap, err := inviteDailyCap(database)
	if err != nil || cap != 25 {
		t.Fatalf("cap=%d err=%v", cap, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindInviteRelationSkipsAtDailyCap(t *testing.T) {
	database, mock := inviteCapTestDB(t)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT `uid` FROM `invite_user_code` WHERE code = .* LIMIT").
		WithArgs("ABC123", 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid"}).AddRow(42))
	mock.ExpectQuery("SELECT `id` FROM `user` WHERE id = .* LIMIT .* FOR UPDATE").
		WithArgs(42, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery("SELECT `id` FROM `invite_record` WHERE uid = .* LIMIT .* FOR UPDATE").
		WithArgs(99, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .*value.* FROM `settings` WHERE `key` = .* LIMIT .* FOR SHARE").
		WithArgs(constant.InviteDailyCapSettingKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("3"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM invite_record WHERE inviter_uid = .* AND created_at >= .* AND created_at < .* FOR UPDATE").
		WithArgs(42, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(3))
	if err := bindInviteRelation(database, 99, "ABC123", now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindInviteRelationCreatesBelowDailyCap(t *testing.T) {
	database, mock := inviteCapTestDB(t)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `uid` FROM `invite_user_code` WHERE code = .* LIMIT").
		WithArgs("ABC123", 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid"}).AddRow(42))
	mock.ExpectQuery("SELECT `id` FROM `user` WHERE id = .* LIMIT .* FOR UPDATE").
		WithArgs(42, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery("SELECT `id` FROM `invite_record` WHERE uid = .* LIMIT .* FOR UPDATE").
		WithArgs(99, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .*value.* FROM `settings` WHERE `key` = .* LIMIT .* FOR SHARE").
		WithArgs(constant.InviteDailyCapSettingKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("3"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM invite_record WHERE inviter_uid = .* AND created_at >= .* AND created_at < .* FOR UPDATE").
		WithArgs(42, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(2))
	mock.ExpectExec("INSERT INTO `invite_record`").
		WithArgs(99, 42, 0, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := database.Transaction(func(tx *gorm.DB) error {
		return bindInviteRelation(tx, 99, "ABC123", now)
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
