package project

import (
	jsonv1 "encoding/json"
	"errors"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

func DispatchFairLaunchActionTx(tx *gorm.DB, row *entity.FairLaunchActionRequest, r projectruntime.Runtime, now time.Time) error {
	if !constant.FairLaunchMaterializationEnabled {
		return errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if row == nil || row.Status != "executing" || row.ExecutorAdminID == nil || row.ExecutorMFAEvidenceRef == nil {
		return errors.New("action is not ready for dispatch")
	}
	if !r.Enabled || !strings.EqualFold(r.ContractAddress, projectAddress(tx, row.ProjectID)) {
		return errors.New("trusted runtime mismatch")
	}
	evidence, err := immutableActionEvidence(row)
	if err != nil {
		return err
	}
	mfa := FairLaunchActionMFAEvidence{Reference: *row.ExecutorMFAEvidenceRef, VerifiedAt: *row.ExecutorMFAVerifiedAt}
	switch row.ExecutionDomain {
	case "operator_durable_outbox":
		calldata, action, err := fairLaunchOperatorCalldata(row.ProjectID, row.Action, evidence)
		if err != nil {
			return err
		}
		submission := entity.ChainTxSubmission{ProjectID: row.ProjectID, Action: action, AttemptNo: 1, ChainID: r.ChainID, SenderAddress: strings.ToLower(r.OperatorAddress), ContractAddress: strings.ToLower(r.ContractAddress), CallData: calldata, ValueRaw: "0", State: constant.ChainTxStateIntent, GasPriceRaw: "0", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&submission).Error; err != nil {
			return err
		}
		if err := appendActionTransitionTx(tx, row, "executing", "dispatched", *row.ExecutorAdminID, mfa, now); err != nil {
			return err
		}
		row.SubmissionID = &submission.ID
		row.Status = "dispatched"
		row.UpdatedAt = now
		return tx.Save(row).Error
	case "external_safe_observation", "pause_guardian_safe", "unpause_timelock", "governance_timelock":
		return errors.New("external_dispatch_unavailable")
	default:
		return errors.New("unsupported execution domain")
	}
}

func immutableActionEvidence(row *entity.FairLaunchActionRequest) (FairLaunchActionEvidence, error) {
	var payload map[string]any
	decoder := jsonv1.NewDecoder(strings.NewReader(row.CanonicalPayload))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return FairLaunchActionEvidence{}, errors.New("immutable action payload malformed")
	}
	digest, err := canonicalDigest(payload)
	if err != nil || !strings.EqualFold(stripActionDigest(digest), row.CanonicalDigest) {
		return FairLaunchActionEvidence{}, errors.New("immutable action payload digest mismatch")
	}
	if action, ok := payload["action"].(string); !ok || action != row.Action {
		return FairLaunchActionEvidence{}, errors.New("immutable action identity mismatch")
	}
	str := func(k string) string { v, _ := payload[k].(string); return v }
	manifest := uint32(0)
	switch v := payload["manifest_version"].(type) {
	case jsonv1.Number:
		n, _ := strconv.ParseUint(v.String(), 10, 32)
		manifest = uint32(n)
	case float64:
		manifest = uint32(v)
	case string:
		n, _ := strconv.ParseUint(v, 10, 32)
		manifest = uint32(n)
	}
	return FairLaunchActionEvidence{ReasonCode: str("reason_code"), EvidenceHash: str("evidence_hash"), OperationID: str("operation_id"), GovernanceClass: str("governance_class"), RequestedUntil: str("requested_until"), Notes: str("notes"), SettlementBlock: str("settlement_block"), AnchorBlock: str("anchor_block"), AnchorHash: str("anchor_hash"), ManifestDigest: str("manifest_digest"), ManifestVersion: manifest, PolicyDigest: str("policy_digest")}, nil
}
func projectAddress(tx *gorm.DB, projectID uint64) string {
	var row entity.Project
	if tx.Where("project_id=?", projectID).Take(&row).Error != nil {
		return ""
	}
	return row.ContractAddress
}

var canonicalBytes32Pattern = regexp.MustCompile(`^0x[0-9a-f]{64}$`)

func requireCanonicalNonZeroBytes32(name, value string) (common.Hash, error) {
	if !canonicalBytes32Pattern.MatchString(value) || value == "0x"+strings.Repeat("0", 64) {
		return common.Hash{}, errors.New(name + " must be canonical nonzero bytes32")
	}
	return common.HexToHash(value), nil
}

func fairLaunchOperatorCalldata(projectID uint64, action string, e FairLaunchActionEvidence) ([]byte, string, error) {
	id := new(big.Int).SetUint64(projectID)
	switch action {
	case "activate":
		data, err := chain.PackFairLaunchActivate(id)
		return data, constant.TransactionTypeFairLaunchActivate, err
	case "end":
		data, err := chain.PackFairLaunchEnd(id)
		return data, constant.TransactionTypeFairLaunchEnd, err
	case "cancel":
		reason, err := requireCanonicalNonZeroBytes32("cancel reason_code", e.ReasonCode)
		if err != nil {
			return nil, "", err
		}
		evidence, err := requireCanonicalNonZeroBytes32("cancel evidence_hash", e.EvidenceHash)
		if err != nil {
			return nil, "", err
		}
		data, err := chain.PackFairLaunchCancel(id, reason, evidence)
		return data, constant.TransactionTypeFairLaunchCancel, err
	case "release":
		settlement, ok := new(big.Int).SetString(e.SettlementBlock, 10)
		if !ok {
			return nil, "", errors.New("release settlement block is invalid")
		}
		anchor, ok := new(big.Int).SetString(e.AnchorBlock, 10)
		if !ok {
			return nil, "", errors.New("release anchor block is invalid")
		}
		anchorHash, err := requireCanonicalNonZeroBytes32("release anchor_hash", e.AnchorHash)
		if err != nil {
			return nil, "", err
		}
		manifestDigest, err := requireCanonicalNonZeroBytes32("release manifest_digest", e.ManifestDigest)
		if err != nil {
			return nil, "", err
		}
		policyDigest, err := requireCanonicalNonZeroBytes32("release policy_digest", e.PolicyDigest)
		if err != nil {
			return nil, "", err
		}
		if e.ManifestVersion == 0 {
			return nil, "", errors.New("release manifest_version must be nonzero")
		}
		data, err := chain.PackFairLaunchRelease(id, settlement, anchor, anchorHash, manifestDigest, e.ManifestVersion, policyDigest)
		return data, constant.TransactionTypeFairLaunchRelease, err
	case "create":
		return nil, "", errors.New("create_dispatch_unavailable")
	default:
		return nil, "", errors.New("action is not an operator durable action")
	}
}
