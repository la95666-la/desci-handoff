package controller

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
)

func TestReadInviteDailyCap(t *testing.T) {
	for _, test := range []struct {
		name  string
		value *string
		want  int
	}{
		{name: "missing", want: constant.InviteDailyCapDefault},
		{name: "configured", value: stringPtr("17"), want: 17},
		{name: "invalid", value: stringPtr("0"), want: constant.InviteDailyCapDefault},
	} {
		t.Run(test.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			rows := sqlmock.NewRows([]string{"value"})
			if test.value != nil {
				rows.AddRow(*test.value)
			}
			mock.ExpectQuery("SELECT .*value.* FROM `settings` WHERE `key` = .* LIMIT").
				WithArgs(constant.InviteDailyCapSettingKey, 1).
				WillReturnRows(rows)
			got, err := readInviteDailyCap(database)
			if err != nil || got != test.want {
				t.Fatalf("got=%d want=%d err=%v", got, test.want, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }
