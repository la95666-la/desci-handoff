package approval

import (
	"encoding/json/v2"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/projectruntime"
	projectService "joincare_server/pkg/core/service/project"
)

type FairLaunchActionApprovalPayload struct {
	ActionRequestID   uint64 `json:"action_request_id"`
	AdminApprovalID   uint64 `json:"admin_approval_id,omitempty"`
	MFAAssertionToken string `json:"mfa_assertion_token,omitempty"`
}

func FairLaunchActionOperation(action string) string {
	return "fair_launch.action." + strings.ReplaceAll(action, "-", "_")
}

var fairLaunchApprovalActions = []string{
	"create", "activate", "end", "cancel", "release",
	"withdraw_project_proceeds", "withdraw_platform_fee", "withdraw_failed_project_tokens",
	"pause_claims", "pause_contributions", "pause_payouts",
	"unpause_claims", "unpause_contributions", "unpause_payouts",
	"configure_beneficiary_safe", "configure_project_token", "configure_release_policy",
	"invalidate_payment_token", "invalidate_project", "recover_excess_tokens",
	"register_payment_token_policy", "set_operator", "set_payment_token_admission",
	"set_payment_token_emergency_blocked", "set_platform_fee",
	"set_project_token_emergency_blocked", "upgrade_proxy",
}

func registerFairLaunchActionHandlers() {
	for _, action := range fairLaunchApprovalActions {
		operation := FairLaunchActionOperation(action)
		Register(operation, "project", "approve", handleFairLaunchActionApproval)
	}
}

// Approval only advances the durable action request to approved. Dispatch is
// deliberately deferred to the explicit third executor endpoint.
func handleFairLaunchActionApproval(tx *gorm.DB, payload string, approverID uint64) (string, func(), error) {
	if !constant.FairLaunchMaterializationEnabled {
		return "", nil, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	var p FairLaunchActionApprovalPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if p.ActionRequestID == 0 || p.AdminApprovalID == 0 {
		return "", nil, errors.New("fair launch action request and approval IDs are required")
	}
	runtime, err := projectruntime.FairLaunchV1()
	if err != nil || !runtime.Enabled {
		return "", nil, errors.New("fair launch runtime disabled")
	}
	if err := projectService.ApproveFairLaunchActionRequestTx(tx, p.ActionRequestID, p.AdminApprovalID, approverID, p.MFAAssertionToken, runtime, time.Now().UTC()); err != nil {
		return "", nil, err
	}
	return "fair launch action approved; explicit distinct executor required", nil, nil
}
