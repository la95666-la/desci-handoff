package project

import (
	"strings"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/projectruntime"
)

type FairLaunchActionDescriptor struct {
	Action                       string   `json:"action"`
	ExecuteRoute                 string   `json:"execute_route,omitempty"`
	ApprovalOperation            string   `json:"approval_operation"`
	Enabled                      bool     `json:"enabled"`
	ExecutionDomain              string   `json:"execution_domain"`
	ReasonCode                   string   `json:"reason_code,omitempty"`
	RequiresFourEyes             bool     `json:"requires_four_eyes"`
	RequiresPreview              bool     `json:"requires_preview"`
	RequiresStepUpMFA            bool     `json:"requires_step_up_mfa"`
	StepUpMFAVerified            bool     `json:"step_up_mfa_verified"`
	StepUpMFAFreshUntil          string   `json:"step_up_mfa_fresh_until,omitempty"`
	RequiresThreePartySeparation bool     `json:"requires_three_party_separation"`
	ForbiddenAdminIDs            []uint64 `json:"forbidden_admin_ids,omitempty"`
}
type FairLaunchPauseState struct {
	ClaimsPauseUntil    string `json:"claims_pause_until,omitempty"`
	ClaimsPaused        bool   `json:"claims_paused"`
	ContributionsPaused bool   `json:"contributions_paused"`
	PayoutsPaused       bool   `json:"payouts_paused"`
}
type FairLaunchCapabilityEnvelope struct {
	ProjectID       uint64                                `json:"project_id"`
	FundingMode     string                                `json:"funding_mode"`
	ContractVersion string                                `json:"contract_version"`
	Available       bool                                  `json:"available"`
	RuntimeEnabled  bool                                  `json:"runtime_enabled"`
	ReasonCode      string                                `json:"reason_code,omitempty"`
	Runtime         FairLaunchRuntimeIdentity             `json:"runtime"`
	PauseState      FairLaunchPauseState                  `json:"pause_state"`
	Actions         map[string]FairLaunchActionDescriptor `json:"actions"`
}

var fairLaunchActions = map[string]string{
	"create": "operator_durable_outbox", "activate": "operator_durable_outbox", "end": "operator_durable_outbox", "cancel": "operator_durable_outbox", "release": "operator_durable_outbox",
	"withdraw_project_proceeds": "external_safe_observation", "withdraw_platform_fee": "external_safe_observation", "withdraw_failed_project_tokens": "external_safe_observation",
	"pause_claims": "pause_guardian_safe", "pause_contributions": "pause_guardian_safe", "pause_payouts": "pause_guardian_safe", "unpause_claims": "unpause_timelock", "unpause_contributions": "unpause_timelock", "unpause_payouts": "unpause_timelock",
	"configure_beneficiary_safe": "governance_timelock", "configure_project_token": "governance_timelock", "configure_release_policy": "governance_timelock", "invalidate_payment_token": "governance_timelock", "invalidate_project": "governance_timelock", "recover_excess_tokens": "governance_timelock", "register_payment_token_policy": "governance_timelock", "set_operator": "governance_timelock", "set_payment_token_admission": "governance_timelock", "set_payment_token_emergency_blocked": "governance_timelock", "set_platform_fee": "governance_timelock", "set_project_token_emergency_blocked": "governance_timelock", "upgrade_proxy": "governance_timelock",
}

func fairLaunchActionDisabledReason(action, persistedDomain string) string {
	domain, exists := fairLaunchActions[action]
	if !exists || (persistedDomain != "" && persistedDomain != domain) {
		return constant.FairLaunchMaterializationUnavailable
	}
	// No Fair Launch mutation may outpace canonical CE-V2 materialization.
	// This gate is deliberately independent of runtime verification so a
	// verified runtime cannot make preview/submission/approval executable.
	return constant.FairLaunchMaterializationUnavailable
}

func BuildFairLaunchCapabilities(projectID uint64, r projectruntime.Runtime, evidence FairLaunchRuntimeEvidence, evidenceErr error) FairLaunchCapabilityEnvelope {
	runtimeReason := FairLaunchRuntimeReason(r, evidence, evidenceErr)
	available := runtimeReason == ""
	actions := make(map[string]FairLaunchActionDescriptor, len(fairLaunchActions))
	for action, domain := range fairLaunchActions {
		actionEnabled, actionReason := available, runtimeReason
		if disabledReason := fairLaunchActionDisabledReason(action, domain); disabledReason != "" {
			actionEnabled, actionReason = false, disabledReason
		}
		actions[action] = FairLaunchActionDescriptor{Action: action, ExecuteRoute: "/v1/projects/:id/fair-launch/actions/:requestID/execute", ApprovalOperation: "fair_launch.action." + action, Enabled: actionEnabled, ExecutionDomain: domain, ReasonCode: actionReason, RequiresFourEyes: true, RequiresPreview: true, RequiresStepUpMFA: true, RequiresThreePartySeparation: true}
	}
	identity := FairLaunchRuntimeIdentity{ChainID: r.ChainID, ContractVersion: r.Version, ProxyAddress: r.ContractAddress, ImplementationAddress: r.ImplementationAddress, ImplementationCodeHash: r.ImplementationCodeHash}
	if available {
		identity = FairLaunchAdminRuntimeIdentity(evidence.Runtime, r.Version)
	}
	// Available describes the read-only runtime evidence. Individual mutation
	// descriptors remain disabled until materialization is admitted.
	return FairLaunchCapabilityEnvelope{ProjectID: projectID, FundingMode: constant.FundingModeFairLaunch, ContractVersion: constant.ContractVersionFairLaunchV1, Available: available, RuntimeEnabled: r.Enabled, ReasonCode: runtimeReason, Runtime: identity, Actions: actions}
}

func FairLaunchRuntimeReason(r projectruntime.Runtime, evidence FairLaunchRuntimeEvidence, err error) string {
	if !r.Enabled {
		return "runtime_disabled"
	}
	if err != nil {
		if strings.HasPrefix(err.Error(), "runtime_stale:") {
			return "runtime_stale"
		}
		return "runtime_unverified"
	}
	if !evidence.Runtime.RuntimeVerified {
		return "runtime_unverified"
	}
	if evidence.Runtime.ChainID != r.ChainID || !strings.EqualFold(evidence.Runtime.ProxyAddress, r.ContractAddress) || !strings.EqualFold(evidence.Runtime.ImplementationAddress, r.ImplementationAddress) || !strings.EqualFold(evidence.Runtime.ImplementationCodeHash, r.ImplementationCodeHash) {
		return "runtime_stale"
	}
	if evidence.Runtime.SourceRegistryRevision == 0 || evidence.Runtime.SourceRegistryDigest == "" || evidence.Runtime.ObservedBlockNumber == "" || evidence.Runtime.ObservedBlockHash == "" || evidence.Runtime.ObservedAt == "" {
		return "runtime_stale"
	}
	return ""
}

func FairLaunchAdminRuntimeIdentity(identity FairLaunchPublicRuntimeIdentity, version string) FairLaunchRuntimeIdentity {
	return FairLaunchRuntimeIdentity{
		ChainID: identity.ChainID, ContractVersion: version, ProxyAddress: identity.ProxyAddress,
		ImplementationAddress: identity.ImplementationAddress, ImplementationCodeHash: identity.ImplementationCodeHash,
		RuntimeVerified: identity.RuntimeVerified, SourceRegistryRevision: identity.SourceRegistryRevision,
		SourceRegistryDigest: identity.SourceRegistryDigest, ObservedBlockNumber: identity.ObservedBlockNumber,
		ObservedBlockHash: identity.ObservedBlockHash, ObservedAt: identity.ObservedAt,
	}
}
