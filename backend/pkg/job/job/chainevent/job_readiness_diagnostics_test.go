package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"joincare_server/pkg/job/db"
)

func TestJobBlockingCountAndDetailsUseSameApplicationTimezone(t *testing.T) {
	for _, offset := range []int{0, 8 * 3600} {
		database, mock := recoveryMockDB(t)
		now := time.Date(2026, 10, 28, 2, 26, 0, 0, time.FixedZone("application", offset))
		cutoff := now.Add(-3 * time.Minute)
		q := blockedSubmissionQuery(database.Session(&gorm.Session{DryRun: true}), cutoff).Count(new(int64))
		if !strings.Contains(q.Statement.SQL.String(), "updated_at < ?") || strings.Contains(q.Statement.SQL.String(), "UTC_TIMESTAMP") {
			t.Fatal(q.Statement.SQL.String())
		}
		var times []time.Time
		for _, v := range q.Statement.Vars {
			if d, ok := v.(time.Time); ok {
				times = append(times, d)
			}
		}
		if len(times) != 2 || times[0] != cutoff || times[1] != cutoff {
			t.Fatal(times)
		}
		mock.ExpectQuery("SELECT id,project_id,chain_id,action,state,contract_address,tx_hash,receipt_block_number,created_at,updated_at FROM `chain_tx_submission` WHERE .* ORDER BY updated_at ASC,id ASC LIMIT ").WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "chain_id", "action", "state", "contract_address", "receipt_block_number", "updated_at"}).AddRow(68, 43368658176704510, 56, "ido_v3_end", "final_success", "0xproxy", 100, now.Add(-5*time.Minute)))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block FROM ido_v3_scan_cursor")).WillReturnRows(sqlmock.NewRows([]string{"cursor_block"}).AddRow(80))
		details, ok := jobBlockedSubmissionDetails(context.Background(), database, now)
		if !ok || len(details) != 1 || details[0].WaitSeconds != 300 || details[0].ProjectID != "43368658176704510" || details[0].Reason != "awaiting_canonical_consumption" || *details[0].BlocksToReceipt != 20 {
			t.Fatal(details, ok)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJobBlockingDiagnosticsSeparateConsumptionFromApplication(t *testing.T) {
	for _, events := range []int{0, 1} {
		database, mock := recoveryMockDB(t)
		now := time.Now()
		mock.ExpectQuery("SELECT id,project_id,chain_id,action,state,contract_address,tx_hash,receipt_block_number,created_at,updated_at FROM").WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "chain_id", "action", "state", "contract_address", "tx_hash", "receipt_block_number", "updated_at"}).AddRow(68, 99, 56, "ido_v3_end", "final_success", "0xproxy", "0xhash", 100, now.Add(-5*time.Minute)))
		mock.ExpectQuery("SELECT cursor_block FROM ido_v3_scan_cursor").WillReturnRows(sqlmock.NewRows([]string{"cursor_block"}).AddRow(110))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM ido_v3_event_journal").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(events))
		details, ok := jobBlockedSubmissionDetails(context.Background(), database, now)
		want := "awaiting_application"
		if events == 0 {
			want = "canonical_event_not_observed"
		}
		if !ok || len(details) != 1 || details[0].Reason != want {
			t.Fatal(details, ok)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJobBlockingDiagnosticsNeverExposeRawPayloadOrDBError(t *testing.T) {
	database, mock := recoveryMockDB(t)
	mock.ExpectQuery("SELECT id,project_id,chain_id,action,state,contract_address,tx_hash,receipt_block_number,created_at,updated_at FROM").WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "action", "state", "updated_at"}).AddRow(1, 2, "SECRET calldata", "SECRET provider", time.Now()))
	details, ok := jobBlockedSubmissionDetails(context.Background(), database, time.Now())
	raw, _ := json.Marshal(details)
	if !ok || strings.Contains(string(raw), "SECRET") || details[0].Action != "other" {
		t.Fatal(string(raw), ok)
	}
}

func TestReadinessKeepsBlockedTransactionFailClosedEvenWhenDetailsUnavailable(t *testing.T) {
	for _, blocked := range []int{1, 0} {
		database, mock := recoveryMockDB(t)
		prior := db.Client.Mysql
		db.Client.Mysql = database
		func() {
			defer func() { db.Client.Mysql = prior; viper.Reset() }()
			now := time.Now()
			mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state` WHERE id = ").WillReturnRows(sqlmock.NewRows([]string{"id", "last_started_at", "last_success_at", "updated_at", "last_error", "lag_blocks"}).AddRow(1, now, now, now, "", 3))
			mock.ExpectQuery("SELECT count.*FROM `chain_event_failure`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery("SELECT count.*FROM `chain_tx_submission`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(blocked))
			mock.ExpectQuery("SELECT count.*FROM `chain_event_reorg_incident`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			if blocked > 0 {
				mock.ExpectQuery("SELECT id,project_id,chain_id,action,state,contract_address,tx_hash,receipt_block_number,created_at,updated_at FROM").WillReturnError(errors.New("SECRET SQL provider URL"))
			}
			router := gin.New()
			router.GET("/health/job-ready", chainEventHealth)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/job-ready", nil))
			want := http.StatusOK
			if blocked > 0 {
				want = http.StatusServiceUnavailable
			}
			if response.Code != want || strings.Contains(response.Body.String(), "SECRET") {
				t.Fatal(response.Code, response.Body.String())
			}
			var body struct {
				Status bool `json:"status"`
				Data   struct {
					Blocked   int  `json:"blocked_chain_transactions"`
					Available bool `json:"blocking_diagnostics_available"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Status != (blocked == 0) || body.Data.Blocked != blocked || body.Data.Available != (blocked == 0) {
				t.Fatal(body, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		}()
	}
}
