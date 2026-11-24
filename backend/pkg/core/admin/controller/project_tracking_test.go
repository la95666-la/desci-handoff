package controller

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrackingWriteRechecksRevokedAdminBeforeProjectLocks(t *testing.T) {
	conn, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(context.Background())
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM `admin_users`").WillReturnRows(sqlmock.NewRows([]string{"id", "is_enable", "role_code"}))
	m.ExpectRollback()
	if err := changeProjectTracking(c, db, 19, 7, 1, "archive", "test reason"); err == nil {
		t.Fatal("revoked admin archived project")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestTrackingHTTPRejectsBadIDAndMissingAuditReason(t *testing.T) {
	for _, tc := range []struct{ id, body string }{{"0", `{}`}, {"19", `{"action":"archive","reason":"x","revision":1}`}} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: tc.id}}
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		ChangeProjectTracking(c)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestTrackingAdminTransitionsAreRevisionAndObligationFenced(t *testing.T) {
	for _, tc := range []struct {
		name, action, status string
		finished, current    bool
		// Nonzero wake with stale evidence is historical queued work, not
		// permission to lose it when changing cadence.
		blocked  int
		revision uint64
		allow    bool
	}{
		{"maintenance terminal", "maintenance", "Failed", false, true, 0, 5, true},
		{"active forbidden", "maintenance", "Active", false, true, 0, 5, false},
		{"pending work forbidden", "maintenance", "Failed", false, true, 1, 5, false},
		{"stale evidence allowed for cadence only", "maintenance", "Failed", false, false, 0, 5, true},
		{"archive unfinished forbidden", "archive", "Successful", false, true, 0, 5, false},
		{"archive finished", "archive", "Successful", true, true, 0, 5, true},
		{"archive stale still forbidden", "archive", "Failed", true, false, 0, 5, false},
		{"maintenance repeated does not slide deadline", "maintenance", "Failed", false, false, 0, 5, false},
		{"sync deferred wake allowed", "sync", "Failed", false, false, 0, 5, true},
		{"sync due wake already queued", "sync", "Failed", false, false, 0, 5, false},
		{"concurrent mode change", "normal", "Failed", false, true, 0, 4, false},
		{"immediate sync", "sync", "Failed", false, true, 0, 5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, m, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/", nil)
			m.ExpectBegin()
			m.ExpectQuery("SELECT .* FROM `admin_users`").WillReturnRows(sqlmock.NewRows([]string{"id", "is_enable", "role_code"}).AddRow(7, 1, "super_admin"))
			m.ExpectQuery("SELECT .* FROM `admin_user_roles`").WillReturnRows(sqlmock.NewRows([]string{"id", "role_code"}))
			m.ExpectQuery("SELECT .* FROM `chain_event_sync_state`").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			m.ExpectQuery("SELECT count.* FROM `chain_event_reorg_incident`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectQuery("SELECT chain_id,epoch,state,mode FROM ido_v3_proxy_stream").WillReturnRows(sqlmock.NewRows([]string{"chain_id", "epoch", "state", "mode"}).AddRow(56, 1, "running", "stream"))
			m.ExpectQuery("SELECT p.status,p.onchain_locked FROM projects").WillReturnRows(sqlmock.NewRows([]string{"status", "onchain_locked"}).AddRow(tc.status, 1))
			m.ExpectQuery("SELECT cursor_block,state FROM ido_v3_scan_cursor").WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "state"}).AddRow(100, "running"))
			m.ExpectExec("INSERT INTO ido_v3_project_tracking").WillReturnResult(sqlmock.NewResult(0, 0))
			wake := uint64(0)
			mode, due := "normal", true
			if tc.action == "maintenance" && !tc.current {
				wake = 250
			}
			if strings.Contains(tc.name, "repeated") {
				mode = "maintenance"
			}
			if strings.Contains(tc.name, "sync deferred") {
				mode = "maintenance"
				due = false
				wake = 250
			}
			if strings.Contains(tc.name, "sync due") {
				mode = "maintenance"
				wake = 250
			}
			m.ExpectQuery("SELECT t.*FROM ido_v3_project_tracking").WillReturnRows(sqlmock.NewRows([]string{"mode", "revision", "finished", "evidence_current", "evidence_block", "wake_block", "due"}).AddRow(mode, 5, tc.finished, tc.current, 100, wake, due))
			if tc.revision == 5 {
				m.ExpectQuery("SELECT .*SELECT COUNT.*FROM chain_tx_submission").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.blocked))
				if tc.action == "archive" {
					m.ExpectQuery("SELECT COUNT.*FROM ido_v3_proxy_stream").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				}
			}
			if tc.allow {
				if tc.action == "maintenance" {
					m.ExpectExec("UPDATE ido_v3_project_tracking SET mode=\\?,next_scan_at=TIMESTAMPADD\\(HOUR,12,UTC_TIMESTAMP\\(6\\)\\),wake_block=\\?,revision=revision\\+1").WithArgs("maintenance", wake, uint64(19), uint64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
				} else {
					if strings.Contains(tc.name, "sync deferred") {
						m.ExpectExec("UPDATE ido_v3_project_tracking SET mode=").WithArgs("maintenance", uint64(250), uint64(19), uint64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
					} else {
						m.ExpectExec("UPDATE ido_v3_project_tracking SET mode=").WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
				m.ExpectExec("INSERT INTO ido_v3_tracking_audit").WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			err = changeProjectTracking(c, db, 19, 7, tc.revision, tc.action, "audited test")
			if (err == nil) != tc.allow {
				t.Fatal(err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
