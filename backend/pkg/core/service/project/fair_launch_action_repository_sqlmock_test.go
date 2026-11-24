package project

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/projectruntime"
)

func unsupportedActionMock(t *testing.T, status, action, domain string, approver *uint64) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	columns := []string{"id", "project_id", "canonical_digest", "canonical_payload", "action", "execution_domain", "initiator_admin_id", "approver_admin_id", "status", "approval_expires_at", "execution_expires_at"}
	row := sqlmock.NewRows(columns).AddRow(1, 9, "digest", "{}", action, domain, 11, approver, status, now.Add(time.Hour), now.Add(time.Hour))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `fair_launch_action_request` WHERE id=? LIMIT ? FOR UPDATE")).WithArgs(uint64(1), 1).WillReturnRows(row)
	return gormDB, mock
}

func TestUnsupportedPersistedActionsStopBeforeMFAOrMutation(t *testing.T) {
	if !constant.FairLaunchMaterializationEnabled {
		t.Skip("global materialization gate now rejects before persisted request reads")
	}
	cases := []struct{ action, domain, reason string }{
		{"create", "operator_durable_outbox", "fair_launch_materialization_unavailable"},
		{"upgrade_proxy", "governance_timelock", "fair_launch_materialization_unavailable"},
		{"pause_claims", "pause_guardian_safe", "fair_launch_materialization_unavailable"},
		{"release", "external_safe_observation", "fair_launch_materialization_unavailable"},
		{"unpause_contributions", "unpause_timelock", "fair_launch_materialization_unavailable"},
		{"unknown_old_action", "operator_durable_outbox", "fair_launch_materialization_unavailable"},
		{"end", "governance_timelock", "fair_launch_materialization_unavailable"},
	}
	for _, tc := range cases {
		t.Run("approve_"+tc.action+"_"+tc.domain, func(t *testing.T) {
			db, mock := unsupportedActionMock(t, fairLaunchStatusPending, tc.action, tc.domain, nil)
			err := ApproveFairLaunchActionRequestTx(db, 1, 22, 33, "opaque-mfa-token-that-must-not-be-read", projectruntime.Runtime{Enabled: true}, time.Now().UTC())
			if err == nil || err.Error() != tc.reason {
				t.Fatalf("expected %s, got %v", tc.reason, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("execute_"+tc.action+"_"+tc.domain, func(t *testing.T) {
			approver := uint64(22)
			db, mock := unsupportedActionMock(t, fairLaunchStatusApproved, tc.action, tc.domain, &approver)
			_, err := ExecuteFairLaunchActionRequestTx(db, ExecuteFairLaunchActionRequestInput{RequestID: 1, ProjectID: 9, ExecutorAdminID: 33, MFAAssertionToken: "opaque-mfa-token-that-must-not-be-read", Runtime: projectruntime.Runtime{Enabled: true}, Now: time.Now().UTC()})
			if err == nil || err.Error() != tc.reason {
				t.Fatalf("expected %s, got %v", tc.reason, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
