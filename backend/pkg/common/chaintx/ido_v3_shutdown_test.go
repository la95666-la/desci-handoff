package chaintx

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

func TestV3OutboxActionsRemainFencedBeforeMaterialization(t *testing.T) {
	for _, action := range []string{"v3_create", "v3_activate", "v3_end", "v3_cancel", "v3_policy", "ido_v3_future_action", "  V3_MIGRATION  "} {
		if err := rejectFairLaunchTransactionAction(action); !errors.Is(err, ErrIDOV3MaterializationUnavailable) {
			t.Fatalf("V3 action %q escaped fence: %v", action, err)
		}
	}
	for _, action := range []string{constant.TransactionTypeV2Create, constant.TransactionTypeV2Activate, constant.TransactionTypeProjectEnd} {
		if err := rejectFairLaunchTransactionAction(action); err != nil {
			t.Fatalf("V2 action %q incorrectly fenced: %v", action, err)
		}
	}
}

func TestV3OutboxProgressionStopsBeforeSignerOrReceipt(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*gorm.DB) error
	}{
		{"prepare", func(db *gorm.DB) error { _, err := Prepare(context.Background(), db, nil, 7); return err }},
		{"dispatch", func(db *gorm.DB) error { _, err := Dispatch(context.Background(), db, nil, 7); return err }},
		{"reconcile", func(db *gorm.DB) error { _, err := ReconcileReceipt(context.Background(), db, nil, 7, 1); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			expectSubmissionIdentity(mock, 7, "ido_v3_future_action", constant.ChainTxStateReady)
			if err := tc.call(db); !errors.Is(err, ErrIDOV3MaterializationUnavailable) {
				t.Fatalf("V3 progression escaped: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		call func(*gorm.DB, *countingGateway) error
	}{
		{"gateway signing", func(db *gorm.DB, gateway *countingGateway) error {
			_, err := RunGatewaySigning(context.Background(), db, gateway, SigningReservation{SubmissionID: 7}, []byte{1}, 1, time.Now())
			return err
		}},
		{"gateway recovery", func(db *gorm.DB, gateway *countingGateway) error {
			_, err := RecoverGatewaySigning(context.Background(), db, gateway, SigningReservation{SubmissionID: 7}, 1, time.Now())
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT `action` FROM `chain_tx_submission` WHERE id = ? LIMIT ?")).WithArgs(7, 1).
				WillReturnRows(sqlmock.NewRows([]string{"action"}).AddRow("ido_v3_future_action"))
			gateway := &countingGateway{}
			if err := tc.call(db, gateway); !errors.Is(err, ErrIDOV3MaterializationUnavailable) {
				t.Fatalf("V3 gateway escaped: %v", err)
			}
			if gateway.commit != 0 || gateway.journal != 0 {
				t.Fatalf("gateway used before V3 materialization: %+v", gateway)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
