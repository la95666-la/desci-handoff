package project

import (
	"errors"
	"joincare_server/pkg/common/projectruntime"
	"time"
)

type FairLaunchActionEvidence struct {
	ReasonCode      string `json:"reason_code,omitempty"`
	EvidenceHash    string `json:"evidence_hash,omitempty"`
	OperationID     string `json:"operation_id,omitempty"`
	GovernanceClass string `json:"governance_class,omitempty"`
	RequestedUntil  string `json:"requested_until,omitempty"`
	Notes           string `json:"notes,omitempty"`
	CanonicalDigest string `json:"canonical_digest,omitempty"`
	SettlementBlock string `json:"settlement_block,omitempty"`
	AnchorBlock     string `json:"anchor_block,omitempty"`
	AnchorHash      string `json:"anchor_hash,omitempty"`
	ManifestDigest  string `json:"manifest_digest,omitempty"`
	ManifestVersion uint32 `json:"manifest_version,omitempty"`
	PolicyDigest    string `json:"policy_digest,omitempty"`
}
type FairLaunchPrecondition struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	OK      bool   `json:"ok"`
}
type FairLaunchActionPreview struct {
	Action                       string                    `json:"action"`
	ApprovalOperation            string                    `json:"approval_operation"`
	CanonicalDigest              string                    `json:"canonical_digest"`
	PreviewID                    uint64                    `json:"preview_id"`
	PreviewToken                 string                    `json:"preview_token"`
	ExecutionDomain              string                    `json:"execution_domain"`
	ExpiresAt                    string                    `json:"expires_at"`
	ImmutablePayload             map[string]any            `json:"immutable_payload"`
	Preconditions                []FairLaunchPrecondition  `json:"preconditions"`
	RequiresFourEyes             bool                      `json:"requires_four_eyes"`
	RequiresStepUpMFA            bool                      `json:"requires_step_up_mfa"`
	RequiresThreePartySeparation bool                      `json:"requires_three_party_separation"`
	Runtime                      FairLaunchRuntimeIdentity `json:"runtime"`
}

func PrepareFairLaunchActionPreview(projectID uint64, action string, evidence FairLaunchActionEvidence, r projectruntime.Runtime, runtimeEvidence FairLaunchRuntimeEvidence, runtimeEvidenceErr error, now time.Time) (FairLaunchActionPreview, error) {
	caps := BuildFairLaunchCapabilities(projectID, r, runtimeEvidence, runtimeEvidenceErr)
	d, ok := caps.Actions[action]
	if !ok {
		return FairLaunchActionPreview{}, errors.New("unknown fair launch action")
	}
	if !d.Enabled {
		return FairLaunchActionPreview{}, errors.New(d.ReasonCode)
	}
	if _, _, err := fairLaunchOperatorCalldata(projectID, action, evidence); err != nil {
		return FairLaunchActionPreview{}, err
	}
	payload := map[string]any{"project_id": projectID, "action": action, "reason_code": evidence.ReasonCode, "evidence_hash": evidence.EvidenceHash, "operation_id": evidence.OperationID, "governance_class": evidence.GovernanceClass, "requested_until": evidence.RequestedUntil, "notes": evidence.Notes, "settlement_block": evidence.SettlementBlock, "anchor_block": evidence.AnchorBlock, "anchor_hash": evidence.AnchorHash, "manifest_digest": evidence.ManifestDigest, "manifest_version": evidence.ManifestVersion, "policy_digest": evidence.PolicyDigest, "runtime_chain_id": r.ChainID, "runtime_proxy": r.ContractAddress, "implementation_code_hash": r.ImplementationCodeHash, "release_policy_digest": r.ReleasePolicyDigest, "release_policy_version": r.ReleasePolicyVersion, "source_registry_revision": runtimeEvidence.Runtime.SourceRegistryRevision, "source_registry_digest": runtimeEvidence.Runtime.SourceRegistryDigest, "observed_block_number": runtimeEvidence.Runtime.ObservedBlockNumber, "observed_block_hash": runtimeEvidence.Runtime.ObservedBlockHash, "runtime_generation": runtimeEvidence.Generations.Runtime, "policy_generation": runtimeEvidence.Generations.Policy, "source_registry_generation": runtimeEvidence.Generations.SourceRegistry, "config_generation": runtimeEvidence.Generations.Config}
	digest, err := canonicalDigest(payload)
	if err != nil {
		return FairLaunchActionPreview{}, err
	}
	return FairLaunchActionPreview{Action: action, ApprovalOperation: d.ApprovalOperation, CanonicalDigest: digest, ExecutionDomain: d.ExecutionDomain, ExpiresAt: now.Add(FairLaunchPreviewTTL).UTC().Format(time.RFC3339), ImmutablePayload: payload, Preconditions: []FairLaunchPrecondition{{Code: "runtime_enabled", Message: "trusted runtime is enabled", OK: true}, {Code: "runtime_verified", Message: "runtime identity is verified", OK: true}, {Code: "reorg_fence_clear", Message: "must be revalidated transactionally at execution", OK: true}}, RequiresFourEyes: d.RequiresFourEyes, RequiresStepUpMFA: d.RequiresStepUpMFA, RequiresThreePartySeparation: d.RequiresThreePartySeparation, Runtime: FairLaunchAdminRuntimeIdentity(runtimeEvidence.Runtime, r.Version)}, nil
}
