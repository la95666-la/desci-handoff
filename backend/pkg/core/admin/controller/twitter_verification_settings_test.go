package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
)

func TestUpdateTwitterVerificationSettingsRequiresExplicitBoolean(t *testing.T) {
	database, mock := twitterVerificationSettingsDB(t)
	old := db.Client.Mysql
	db.Client.Mysql = database
	t.Cleanup(func() { db.Client.Mysql = old })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/v1/twitter-verification-settings", strings.NewReader(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateTwitterVerificationSettings(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTwitterVerificationSettingsPersistsDisabledPolicy(t *testing.T) {
	database, mock := twitterVerificationSettingsDB(t)
	old := db.Client.Mysql
	db.Client.Mysql = database
	t.Cleanup(func() { db.Client.Mysql = old })

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `settings` WHERE `key` = .* LIMIT .* FOR UPDATE").
		WithArgs(constant.TwitterVerificationDisabledSettingKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{"key", "value", "type"}).AddRow(constant.TwitterVerificationDisabledSettingKey, "0", "int"))
	mock.ExpectExec("UPDATE `settings` SET .* WHERE `key` = .*").
		WithArgs("int", "1", constant.TwitterVerificationDisabledSettingKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/v1/twitter-verification-settings", strings.NewReader(`{"verificationDisabled":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateTwitterVerificationSettings(ctx)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"verificationDisabled":true`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func twitterVerificationSettingsDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}
