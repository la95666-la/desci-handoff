package project

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
)

// The unknown outcome default is intentionally conservative. An HTTP failure
// or failed DB read MUST NOT become permission to sign/send again.
type V3OperationFailure struct {
	Reason       string `json:"reason_code"`
	Stage        string `json:"stage"`
	Message      string `json:"message"`
	NextAction   string `json:"next_action"`
	SubmissionID string `json:"submission_id,omitempty"`
	TxHash       string `json:"tx_hash,omitempty"`
	Outcome      string `json:"submission_outcome"`
	Check        string `json:"check,omitempty"`
}
type v3OperationError struct {
	failure V3OperationFailure
	cause   error
}

func (e *v3OperationError) Error() string { return "V3 operation unavailable" }
func (e *v3OperationError) Unwrap() error { return e.cause }
func V3Failure(err error) (V3OperationFailure, bool) {
	var operation *v3OperationError
	if errors.As(err, &operation) {
		return operation.failure, true
	}
	var diagnostic *v3diagnostic.Error
	if errors.As(err, &diagnostic) {
		r := v3diagnostic.Reason(err)
		return V3OperationFailure{Reason: r, Stage: v3diagnostic.Stage(err), Message: v3diagnostic.Message(r), NextAction: "refresh_and_check", Outcome: "unknown"}, true
	}
	if errors.Is(err, ErrV3PolicyReviewSchemaMigrationRequired) {
		return V3OperationFailure{Reason: "CONFIG_UNAVAILABLE", Stage: "request_gate", Message: "白名单审批数据库迁移尚未完成，请联系运维；不要重复提交。", NextAction: "contact_operator", Outcome: "unknown"}, true
	}
	var observation v3PolicyObservationStageError
	if errors.As(err, &observation) {
		stage := V3PolicyObservationFailureStage(err)
		reason := "UNAVAILABLE"
		if stage == "scanner_freshness" || stage == "final_scanner_freshness" {
			reason = "SCANNER_NOT_FRESH"
		}
		if stage == "canonical_policy_receipt" || stage == "repeat_receipt" {
			reason = "RECEIPT_UNVERIFIED"
		}
		if stage == "safe_execution" {
			reason = "SAFE_CHECK_FAILED"
		}
		return V3OperationFailure{Reason: reason, Stage: v3diagnostic.Stage(v3diagnostic.New(reason, stage, nil)), Message: v3diagnostic.Message(reason), NextAction: "reconcile_original_do_not_resend", Outcome: "unknown", Check: V3PolicyObservationFailureDetail(err)}, true
	}
	return V3OperationFailure{}, false
}
func v3ReadDiagnostic(err error, stage string) error {
	if err == nil {
		return nil
	}
	reason := "UNAVAILABLE"
	switch stage {
	case "request_gate", "mode_ready", "operator_identity", "safe_identity":
		reason = "CONFIG_UNAVAILABLE"
	}
	var n net.Error
	var h rpc.HTTPError
	if errors.Is(err, context.Canceled) {
		reason = "REQUEST_CANCELLED"
	} else if errors.Is(err, context.DeadlineExceeded) {
		reason = "CHECK_TIMEOUT"
	} else if errors.As(err, &n) && n.Timeout() {
		reason = "RPC_TIMEOUT"
	} else if errors.As(err, &h) {
		reason = "RPC_UNAVAILABLE"
	}
	return v3diagnostic.Annotate(err, reason, stage)
}

// Existing policy/create readers collapse internal details. Report a known
// stage without guessing whether it was a provider outage or an identity
// conflict. This never performs writes or changes any admission decision.
func v3PolicyScanDiagnostic(ctx context.Context, database *gorm.DB, id uint64) error {
	if ctx == nil || database == nil {
		return v3diagnostic.New("UNAVAILABLE", "project_admission", nil)
	}
	var states []struct{ State string }
	r := database.WithContext(ctx).Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=56 AND project_id=? AND contract_version='ido_v3_stablecoin' LIMIT 2`, id).Scan(&states)
	if r.Error == nil && len(states) == 1 && states[0].State == "reorg_halted" {
		return v3diagnostic.New("PROJECT_SCAN_HALTED", "project_admission", nil)
	}
	return nil
}
func wrapV3OperationFailure(ctx context.Context, database *gorm.DB, id uint64, phase string, err error) error {
	err = v3ReadDiagnostic(err, phase)
	f, _ := V3Failure(err)
	f.Outcome = "unknown"
	f.NextAction = "refresh_and_check"
	if id == 0 && f.Reason == "CREATE_RUNWAY_INSUFFICIENT" {
		f.Outcome = "not_reserved"
		f.NextAction = "prepare_future_set_and_review"
	}
	if id == 0 && (f.Reason == "SENDER_EVIDENCE_PENDING" || f.Reason == "SENDER_EVIDENCE_UNAVAILABLE" || f.Reason == "SENDER_RETIREMENT_REVIEW_REQUIRED" || f.Reason == "SENDER_SUSPENDED") {
		f.Outcome = "not_reserved"
		f.NextAction = "check_sender_maintenance"
	}
	if id > 0 {
		f.SubmissionID = strconv.FormatUint(id, 10)
		if database != nil {
			check, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var row entity.ChainTxSubmission
			r := database.WithContext(check).Select("id,state,nonce,tx_hash,raw_tx,broadcast_count").Where("id=?", id).Take(&row)
			if r.Error == nil && row.ID == id {
				if row.TxHash != nil && v3CanonicalHash(*row.TxHash) {
					f.TxHash = *row.TxHash
				}
				if row.State == "cancelled" && row.Nonce == nil && row.TxHash == nil && len(row.RawTx) == 0 && row.BroadcastCount == 0 {
					f.Outcome = "unsigned_cancelled"
				} else if row.Nonce != nil || row.TxHash != nil || len(row.RawTx) > 0 || row.BroadcastCount > 0 {
					f.Outcome = "signed_or_submitted"
				}
			}
		}
	}
	if f.Outcome != "unsigned_cancelled" && id > 0 {
		f.NextAction = "reconcile_original_do_not_resend"
		f.Message += " 已建立交易记录；请先核对原 attempt／交易哈希，勿重复提交。"
	}
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(map[string]any{"stage": f.Stage, "reason_code": f.Reason, "submission_id": f.SubmissionID, "submission_outcome": f.Outcome}).Warn("V3 operation failed")
	}
	return &v3OperationError{f, err}
}
