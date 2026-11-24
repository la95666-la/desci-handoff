package project

import (
	"context"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
	"strings"
	"testing"
	"time"
)

func TestV3DiagnosticOutcomeRequiresDurableEvidence(t *testing.T) {
	hash := "0x" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, state string
		nonce       any
		hash        any
		raw         any
		broadcast   int
		readFail    bool
		outcome     string
	}{
		{"unsigned cancelled", "cancelled", nil, nil, nil, 0, false, "unsigned_cancelled"},
		{"signed", "signed", uint64(51), hash, []byte{1}, 0, false, "signed_or_submitted"},
		{"submitted", "submitted", uint64(51), hash, []byte{1}, 1, false, "signed_or_submitted"},
		{"ready not cancelled", "ready", nil, nil, nil, 0, false, "unknown"},
		{"DB unknown", "", nil, nil, nil, 0, true, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			q := mock.ExpectQuery("SELECT id,state,nonce,tx_hash,raw_tx,broadcast_count").WithArgs(uint64(65), 1)
			if tc.readFail {
				q.WillReturnError(errors.New("SECRET SQL"))
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"id", "state", "nonce", "tx_hash", "raw_tx", "broadcast_count"}).AddRow(65, tc.state, tc.nonce, tc.hash, tc.raw, tc.broadcast))
			}
			e := wrapV3OperationFailure(context.Background(), database, 65, "prepare_signature", v3diagnostic.New("CREATE_DEADLINE_EXPIRED", "signature_persist", errors.New("SECRET RPC")))
			f, ok := V3Failure(e)
			if !ok || f.Outcome != tc.outcome || f.Reason != "CREATE_DEADLINE_EXPIRED" || f.Stage != "signature_persist" {
				t.Fatal(f)
			}
			if strings.Contains(f.Message, "SECRET") {
				t.Fatal("cause leaked")
			}
			if tc.outcome != "unsigned_cancelled" && f.NextAction != "reconcile_original_do_not_resend" {
				t.Fatal("unknown allowed retry", f)
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestV3ExpiredUnsignedIntentHasSpecificReason(t *testing.T) {
	e := checkApprovedV3UnsignedIntent(context.Background(), nil, 19, 65, idov3whitelist.ApprovedSetIdentity{CreateDeadline: time.Now().Unix() - 1}, idov3whitelist.ExpectedProjectBinding{})
	if v3diagnostic.Reason(e) != "CREATE_DEADLINE_EXPIRED" {
		t.Fatal(e)
	}
}

func TestSenderReadinessBeforeIntentDoesNotPretendSigned(t *testing.T) {
	for _, reason := range []string{"SENDER_EVIDENCE_PENDING", "SENDER_EVIDENCE_UNAVAILABLE", "SENDER_RETIREMENT_REVIEW_REQUIRED"} {
		e := wrapV3OperationFailure(context.Background(), nil, 0, "sender_readiness", v3diagnostic.New(reason, "sender_readiness", nil))
		f, ok := V3Failure(e)
		if !ok || f.Outcome != "not_reserved" || f.SubmissionID != "" || f.NextAction != "check_sender_maintenance" || f.Reason != reason {
			t.Fatal(f)
		}
	}
}
func TestV3UnknownPhaseNeverReturned(t *testing.T) {
	e := wrapV3OperationFailure(context.Background(), nil, 0, "private-RPC-secret", fmt.Errorf("SECRET URL"))
	f, _ := V3Failure(e)
	if f.Stage != "unavailable" || strings.Contains(f.Message, "SECRET") {
		t.Fatal(f)
	}
}

func TestCreateRunwayRejectionBeforeIntentIsExplicitlyNotReserved(t *testing.T) {
	err := wrapV3OperationFailure(context.Background(), nil, 0, "approved_set", v3diagnostic.New("CREATE_RUNWAY_INSUFFICIENT", "approved_set", nil))
	f, ok := V3Failure(err)
	if !ok || f.Outcome != "not_reserved" || f.NextAction != "prepare_future_set_and_review" || f.Reason != "CREATE_RUNWAY_INSUFFICIENT" {
		t.Fatal(f)
	}
}

func TestSenderEvidenceFailureAfterSignatureDoesNotClaimUnsigned(t *testing.T) {
	for _, reason := range []string{"SENDER_EVIDENCE_PENDING", "SENDER_EVIDENCE_UNAVAILABLE"} {
		database, mock := v2AdmissionMock(t)
		mock.ExpectQuery("SELECT id,state,nonce,tx_hash,raw_tx,broadcast_count").WithArgs(uint64(65), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "state", "nonce", "tx_hash", "raw_tx", "broadcast_count"}).AddRow(65, "signed", 51, "0x"+strings.Repeat("a", 64), []byte{1}, 0))
		f, _ := V3Failure(wrapV3OperationFailure(context.Background(), database, 65, "signed_admission", v3diagnostic.New(reason, "sender_readiness", nil)))
		if f.Outcome != "signed_or_submitted" || f.NextAction != "reconcile_original_do_not_resend" || strings.Contains(f.Message, "尚未建立新的签名") || strings.Contains(f.Message, "未获得签名授权") {
			t.Fatal(f)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSetDiagnosticReportsActualFailedStageWithoutRawCause(t *testing.T) {
	_, err := resolveV3SetTerms(0, OnchainParamsRequest{})
	f, ok := V3Failure(err)
	if !ok || f.Stage != "project_admission" || f.Reason != "UNAVAILABLE" || f.Outcome != "unknown" {
		t.Fatal(f, err)
	}
	if strings.Contains(f.Message, "identity unavailable") {
		t.Fatal(f)
	}
}
