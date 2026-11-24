package approval

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
)

func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }

const DefaultTTL = 72 * time.Hour

// Reserve V3 operation names before introducing any handler or migration.
// Candidate rows and a caller-supplied second Admin ID are not an approval.
// Delimiters prevent unrelated operation names from being captured.
func isUnmaterializedIDOV3Approval(op string) bool {
	op = strings.ToLower(strings.TrimSpace(op))
	return strings.HasPrefix(op, "ido_v3.") || strings.HasPrefix(op, "ido_v3_") ||
		strings.HasPrefix(op, "v3.") || strings.HasPrefix(op, "v3_")
}

type SubmitReq struct {
	Operation     string
	TargetKind    string
	TargetID      string
	Payload       string
	Reason        string
	InitiatorID   uint64
	InitiatorName string
	TTL           time.Duration
}

func Submit(req SubmitReq) (*adminEntity.AdminApproval, error) {
	return SubmitTx(db.Mysql(), req)
}

func SubmitTx(tx *gorm.DB, req SubmitReq) (*adminEntity.AdminApproval, error) {
	if isUnmaterializedIDOV3Approval(req.Operation) {
		return nil, errors.New("IDO V3 approval workflow unavailable")
	}
	if len(req.Operation) >= len("fair_launch.") && req.Operation[:len("fair_launch.")] == "fair_launch." && !constant.FairLaunchMaterializationEnabled {
		return nil, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if req.Operation == "" {
		return nil, errors.New("operation is required")
	}
	if _, err := Get(req.Operation); err != nil {
		return nil, err
	}
	if req.InitiatorID == 0 {
		return nil, errors.New("initiator_id is required")
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	expires := time.Now().Add(ttl)
	row := adminEntity.AdminApproval{Operation: req.Operation, TargetKind: req.TargetKind, TargetID: req.TargetID, Payload: req.Payload, Reason: req.Reason, InitiatorID: req.InitiatorID, InitiatorName: req.InitiatorName, Status: adminEntity.ApprovalPending, ExpiresAt: &expires}
	if err := tx.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func Approve(id, approverID uint64, approverName string) (*adminEntity.AdminApproval, error) {
	return ApproveWithMFA(id, approverID, approverName, "")
}

func ApproveWithMFA(id, approverID uint64, approverName, mfaAssertionToken string) (*adminEntity.AdminApproval, error) {
	var committed adminEntity.AdminApproval
	var afterCommit func()
	var executionErr error
	var decisionErr error
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		row, err := lockPendingApproval(tx, id)
		if err != nil {
			return err
		}
		if isUnmaterializedIDOV3Approval(row.Operation) {
			return errors.New("IDO V3 approval workflow unavailable")
		}
		if len(row.Operation) >= len("fair_launch.") && row.Operation[:len("fair_launch.")] == "fair_launch." && !constant.FairLaunchMaterializationEnabled {
			return errors.New(constant.FairLaunchMaterializationUnavailable)
		}
		if row.ExpiresAt != nil && time.Now().After(*row.ExpiresAt) {
			row.Status = adminEntity.ApprovalExpired
			if err := tx.Save(row).Error; err != nil {
				return err
			}
			committed = *row
			decisionErr = errors.New("approval expired")
			return nil
		}
		if row.InitiatorID == approverID {
			return errors.New("initiator cannot approve own request")
		}
		spec, err := Get(row.Operation)
		if err != nil {
			return err
		}
		if _, err := middleware.HasLivePermissionTx(tx, int64(row.InitiatorID), spec.module, spec.initiatorAction); err != nil {
			return errors.New("approval initiator is no longer authorized for the operation")
		}
		liveAdmin, err := middleware.HasLivePermissionTx(tx, int64(approverID), spec.module, spec.action)
		if err != nil {
			return err
		}
		if liveAdmin.Username != approverName {
			approverName = liveAdmin.Username
		}
		if err := tx.SavePoint("approval_business").Error; err != nil {
			return err
		}
		payload := row.Payload
		if len(row.Operation) > 12 && row.Operation[:12] == "fair_launch." {
			if mfaAssertionToken == "" {
				return errors.New("mfa_assertion_required")
			}
			if row.Operation == OpFairLaunchSet {
				var setPayload FairLaunchSetApprovalPayload
				if err := json.Unmarshal([]byte(payload), &setPayload); err != nil {
					return err
				}
				setPayload.AdminApprovalID = row.ID
				setPayload.MFAAssertionToken = mfaAssertionToken
				buf, err := json.Marshal(setPayload)
				if err != nil {
					return err
				}
				payload = string(buf)
			} else {
				var actionPayload FairLaunchActionApprovalPayload
				if err := json.Unmarshal([]byte(payload), &actionPayload); err != nil {
					return err
				}
				actionPayload.AdminApprovalID = row.ID
				actionPayload.MFAAssertionToken = mfaAssertionToken
				buf, err := json.Marshal(actionPayload)
				if err != nil {
					return err
				}
				payload = string(buf)
			}
		}
		var result string
		var hook func()
		var handlerErr error
		if row.Operation == OpProjectSetOnchainParams {
			// Parse the complete typed payload before dispatch. An ambiguous or
			// malformed V3 request cannot fall through to the legacy V2 handler.
			version, parseErr := projectSetContractVersion(payload)
			if parseErr != nil {
				handlerErr = parseErr
			} else if version == constant.ContractVersionIDOV3Stablecoin && !projectService.ApprovedV3ModeReady() {
				handlerErr = errors.New("V3 approved Set prerequisites unavailable")
			} else if version == constant.ContractVersionIDOV3Stablecoin {
				result, hook, handlerErr = handleApprovedProjectSetOnchainParams(tx, row, approverID)
			} else {
				result, hook, handlerErr = spec.handler(tx, payload, approverID)
			}
		} else {
			result, hook, handlerErr = spec.handler(tx, payload, approverID)
		}
		now := time.Now()
		row.ReviewedAt, row.ExecutedAt, row.ApproverID, row.ApproverName = &now, &now, &approverID, approverName
		if handlerErr != nil {
			if err := tx.RollbackTo("approval_business").Error; err != nil {
				return fmt.Errorf("rollback failed approval handler: %w", err)
			}
			message := handlerErr.Error()
			row.Result, row.Status = &message, adminEntity.ApprovalFailed
			executionErr = handlerErr
		} else {
			row.Result, row.Status, afterCommit = &result, adminEntity.ApprovalExecuted, hook
		}
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		committed = *row
		return nil
	})
	if err != nil {
		return nil, err
	}
	if decisionErr != nil {
		return &committed, decisionErr
	}
	if afterCommit != nil {
		afterCommit()
	}
	if executionErr != nil {
		return &committed, executionErr
	}
	return &committed, nil
}

func Reject(id, approverID uint64, approverName, reason string) (*adminEntity.AdminApproval, error) {
	var committed adminEntity.AdminApproval
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		row, err := lockPendingApproval(tx, id)
		if err != nil {
			return err
		}
		if row.InitiatorID == approverID {
			return errors.New("initiator cannot reject own request")
		}
		spec, err := Get(row.Operation)
		if err != nil {
			return err
		}
		liveAdmin, err := middleware.HasLivePermissionTx(tx, int64(approverID), spec.module, spec.action)
		if err != nil {
			return err
		}
		now := time.Now()
		row.ReviewedAt, row.ApproverID, row.ApproverName = &now, &approverID, liveAdmin.Username
		row.Status, row.RejectReason = adminEntity.ApprovalRejected, reason
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		committed = *row
		return nil
	})
	return &committed, err
}

func Cancel(id, initiatorID uint64) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		row, err := lockPendingApproval(tx, id)
		if err != nil {
			return err
		}
		if row.InitiatorID != initiatorID {
			return errors.New("only initiator can cancel own request")
		}
		var initiator adminEntity.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,is_enable").Where("id = ? AND is_enable = 1", initiatorID).Take(&initiator).Error; err != nil {
			return errors.New("approval initiator is no longer active")
		}
		now := time.Now()
		row.Status, row.RejectReason, row.ReviewedAt = adminEntity.ApprovalRejected, "cancelled by initiator", &now
		return tx.Save(row).Error
	})
}

func lockPendingApproval(tx *gorm.DB, id uint64) (*adminEntity.AdminApproval, error) {
	var row adminEntity.AdminApproval
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("approval not found")
		}
		return nil, err
	}
	if row.Status != adminEntity.ApprovalPending {
		return nil, errors.New("approval is not pending (current: " + row.Status + ")")
	}
	return &row, nil
}

func ExpireStale() (int64, error) {
	res := db.Mysql().Model(&adminEntity.AdminApproval{}).Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", adminEntity.ApprovalPending, time.Now()).Update("status", adminEntity.ApprovalExpired)
	return res.RowsAffected, res.Error
}

func RunHandlerDirect(op string, payload any, actorID uint64) (string, error) {
	spec, err := Get(op)
	if err != nil {
		return "", err
	}
	buf, err := marshalJSON(payload)
	if err != nil {
		return "", err
	}
	// Keep the V2 direct path intact, but never let it manufacture the
	// second administrator required for a V3 immutable Set identity.
	if op == OpProjectSetOnchainParams {
		version, parseErr := projectSetContractVersion(string(buf))
		if parseErr != nil || version == constant.ContractVersionIDOV3Stablecoin {
			return "", errors.New("V3 project Set requires four-eyes approval")
		}
	}
	var result string
	var afterCommit func()
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		var handlerErr error
		result, afterCommit, handlerErr = spec.handler(tx, string(buf), actorID)
		return handlerErr
	})
	if err == nil && afterCommit != nil {
		afterCommit()
	}
	return result, err
}
