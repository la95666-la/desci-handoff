package settings

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
)

func TestTwitterVerificationDisabled(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   *string
		want    bool
		wantErr bool
	}{
		{name: "missing is strict", want: false},
		{name: "zero is strict", value: stringSettingPtr("0"), want: false},
		{name: "false is strict", value: stringSettingPtr("false"), want: false},
		{name: "one disables verification", value: stringSettingPtr("1"), want: true},
		{name: "true disables verification", value: stringSettingPtr("true"), want: true},
		{name: "invalid fails closed", value: stringSettingPtr("unexpected"), wantErr: true},
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
				WithArgs(constant.TwitterVerificationDisabledSettingKey, 1).
				WillReturnRows(rows)

			got, err := TwitterVerificationDisabled(database)
			if got != test.want || (err != nil) != test.wantErr {
				t.Fatalf("got=%v want=%v err=%v wantErr=%v", got, test.want, err, test.wantErr)
			}
			if test.wantErr && !errors.Is(err, ErrInvalidTwitterVerificationSetting) {
				t.Fatalf("err=%v, want ErrInvalidTwitterVerificationSetting", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func stringSettingPtr(value string) *string { return &value }
