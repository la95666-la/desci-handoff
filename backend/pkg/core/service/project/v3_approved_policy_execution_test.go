package project

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedPolicyReceiptRequiresExecutedSetBeforeRPC(t *testing.T) {
	txHash := "0x" + strings.Repeat("a", 64)
	safeHash := "0x" + strings.Repeat("b", 64)
	if err := recordApprovedV3PolicyExecution(nil, nil, nil, 19, 7,
		txHash, safeHash, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("missing scope admitted")
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	// No RPC or policy execution insert can occur before the exact applied
	// original creation and the two-admin Set join are authenticated.
	rpc := &ethclient.Client{}
	if err := recordApprovedV3PolicyExecution(context.Background(), database, rpc, 19, 7,
		txHash, safeHash, common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatal("unexecuted Set admitted a Safe receipt")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovedPolicyObservationErrorOnlyExposesFixedStage(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{v3PolicyObservationStageError("request_gate"), "request_gate"},
		{errors.New("sensitive RPC or database error"), "unavailable"},
		{nil, "unavailable"},
	} {
		if got := V3PolicyObservationFailureStage(tc.err); got != tc.want {
			t.Fatalf("stage = %q, want %q", got, tc.want)
		}
	}
	if err := RecordApprovedV3PolicyExecutionContext(context.Background(), 19, 7, "invalid", "invalid"); V3PolicyObservationFailureStage(err) != "request_gate" {
		t.Fatalf("invalid request stage: %v", err)
	}
	txHash, safeHash := "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64)
	if err := recordApprovedV3PolicyExecution(nil, nil, nil, 19, 7, txHash, safeHash, common.Hash{}, common.Hash{}); V3PolicyObservationFailureStage(err) != "input" {
		t.Fatalf("invalid internal stage: %v", err)
	}
}

func TestApprovedPolicyObservationBudgetBelowFrontendProxyAndCore(t *testing.T) {
	if v3PolicyObservationBudget <= 45*time.Second || v3PolicyObservationBudget >= 170*time.Second {
		t.Fatalf("invalid policy observation time budget: %v", v3PolicyObservationBudget)
	}
}

func TestApprovedPolicyObservationReceiptDetailIsFiniteAndScoped(t *testing.T) {
	for _, detail := range []v3PolicyObservationDetailError{
		"receipt_lookup", "receipt_lookup_deadline", "receipt_lookup_rpc", "receipt_lookup_missing", "receipt_lookup_malformed",
		"receipt_policy_read", "receipt_policy_match", "head_policy_match", "receipt_recheck", "receipt_logs",
	} {
		err := v3PolicyObservationError{v3PolicyObservationStageError("canonical_policy_receipt"), detail}
		if got := V3PolicyObservationFailureStage(err); got != "canonical_policy_receipt" {
			t.Fatalf("stage = %s", got)
		}
		if got := V3PolicyObservationFailureDetail(err); got != string(detail) {
			t.Fatalf("check = %s", got)
		}
		if strings.Contains(err.Error(), string(detail)) {
			t.Fatal("raw detail in error message")
		}
	}
	for _, err := range []error{errors.New("commercial URL token"),
		v3PolicyObservationDetailError("injected_secret"),
		v3PolicyObservationStageError("creation_admission"), nil} {
		if got := V3PolicyObservationFailureDetail(err); got != "" {
			t.Fatalf("unrecognized detail leaked: %s", got)
		}
	}
}
