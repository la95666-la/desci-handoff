package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
)

func TestEngagementProjectIDBodyPreservesSonyflakeID(t *testing.T) {
	const want int64 = 41065961574367233
	for _, body := range []string{
		`{"project_id":"41065961574367233"}`,
		`{"project_id":41065961574367233}`,
	} {
		var req projectIDBody
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		got, err := req.projectID()
		if err != nil || got != want {
			t.Fatalf("%s parsed as %d: %v", body, got, err)
		}
	}
	for _, body := range []string{`{"project_id":0}`, `{"project_id":-1}`, `{"project_id":"9223372036854775808"}`} {
		var req projectIDBody
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		if _, err := req.projectID(); err == nil {
			t.Fatalf("accepted invalid ID: %s", body)
		}
	}
}

func TestV3WarmupSignupNeverAutoApprovesMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := db.Client.Mysql
	db.Client.Mysql = database
	t.Cleanup(func() { db.Client.Mysql = old })
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `contract_version` FROM `projects` WHERE project_id = ?")).WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows([]string{"contract_version"}).AddRow(constant.ContractVersionIDOV3Stablecoin))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(constant.HEADER_JOINCARE_UID, 12)
	c.Request = httptest.NewRequest(http.MethodPost, "/project/v1/whitelist", strings.NewReader(`{"project_id":"77","wallet_address":"0x1111111111111111111111111111111111111111"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	JoinWhitelist(c)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "V3_WHITELIST_SIGNUP_UNAVAILABLE") {
		t.Fatalf("V3 signup response=%d %s", recorder.Code, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3UserStatusNeverReportsWarmupRowAsEligibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := db.Client.Mysql
	db.Client.Mysql = database
	t.Cleanup(func() { db.Client.Mysql = old })
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `contract_version` FROM `projects` WHERE project_id = ?")).WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows([]string{"contract_version"}).AddRow(constant.ContractVersionIDOV3Stablecoin))
	mock.ExpectQuery("SELECT .* FROM `project_subscribers` WHERE project_id = \\? AND uid = \\?.*LIMIT \\?").WithArgs(int64(77), 12, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `project_watches` WHERE project_id = \\? AND uid = \\?.*LIMIT \\?").WithArgs(int64(77), 12, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(constant.HEADER_JOINCARE_UID, 12)
	c.Request = httptest.NewRequest(http.MethodGet, "/project/v1/userStatus?id=77", nil)
	UserStatus(c)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"whitelist_eligibility":"unavailable"`) || strings.Contains(recorder.Body.String(), `"whitelist_approved":true`) {
		t.Fatalf("V3 status=%d %s", recorder.Code, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEngagementGinBindsStringProjectID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"project_id":"41065961574367233"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	// This is the exact Gin binding path used by Watch and Subscribe, without
	// invoking either database write.
	var req projectIDBody
	if err := c.ShouldBindJSON(&req); err != nil {
		t.Fatalf("Gin rejected string project ID: %v", err)
	}
	if got, err := req.projectID(); err != nil || got != 41065961574367233 {
		t.Fatalf("Gin project ID = %d: %v", got, err)
	}
}
