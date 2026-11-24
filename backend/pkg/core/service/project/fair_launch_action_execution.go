package project

import (
	"encoding/json/v2"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
	"time"
)

type FairLaunchActionExecution struct {
	ApprovalID      uint64
	ProjectID       uint64
	Action          string
	InitiatorID     uint64
	ApproverID      uint64
	ExecutorID      uint64
	CanonicalDigest string
	Evidence        FairLaunchActionEvidence
	StepUpMFA       bool
}

func ExecuteFairLaunchApprovedActionTx(tx *gorm.DB, in FairLaunchActionExecution, r projectruntime.Runtime, now time.Time) (string, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return "", errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if in.ApprovalID == 0 || in.ProjectID == 0 || in.InitiatorID == 0 || in.ApproverID == 0 || in.ExecutorID == 0 || in.InitiatorID == in.ApproverID || in.InitiatorID == in.ExecutorID || in.ApproverID == in.ExecutorID {
		return "", errors.New("initiator approver and executor must be distinct")
	}
	if !in.StepUpMFA {
		return "", errors.New("step-up MFA required")
	}
	runtimeEvidence, runtimeEvidenceErr := ReadFairLaunchRuntimeEvidence(tx, r, now)
	preview, err := PrepareFairLaunchActionPreview(in.ProjectID, in.Action, in.Evidence, r, runtimeEvidence, runtimeEvidenceErr, now)
	if err != nil {
		return "", err
	}
	if preview.CanonicalDigest != in.CanonicalDigest {
		return "", errors.New("canonical action digest mismatch")
	}
	var row entity.Project
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id=? AND funding_mode='fair_launch' AND contract_version='fair_launch_v1'", in.ProjectID).Take(&row).Error; err != nil {
		return "", err
	}
	payload, _ := json.Marshal(preview.ImmutablePayload)
	switch preview.ExecutionDomain {
	case "operator_durable_outbox":
		return "operator_dispatch:" + string(payload), nil
	case "external_safe_observation":
		return "external_safe_observation:" + string(payload), nil
	case "pause_guardian_safe":
		return "pause_guardian_safe:" + string(payload), nil
	case "unpause_timelock", "governance_timelock":
		return "governance_timelock:" + string(payload), nil
	default:
		return "", errors.New("unsupported execution domain")
	}
}
