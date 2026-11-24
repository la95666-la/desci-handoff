package project

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

// ApplyFairLaunchSetTx revalidates the canonical preview under a project row
// lock and writes the 09_03 schema atomically. The caller is the approved
// four-eyes handler; runtime-disabled environments cannot enter this path.
func ApplyFairLaunchSetTx(tx *gorm.DB, projectID uint64, req FairLaunchSetRequest, runtime projectruntime.Runtime, inspector FairLaunchSetInspector, now time.Time) (string, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return "", errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	evidence, evidenceErr := ReadFairLaunchRuntimeEvidenceTx(tx, runtime, now, true)
	preview, err := VerifyFairLaunchSetDigest(projectID, req, runtime, evidence, evidenceErr, inspector, now)
	if err != nil {
		return "", err
	}
	project := entity.Project{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
		return "", err
	}
	if project.OnchainLocked {
		return "", errors.New("project is already locked")
	}
	switch project.Status {
	case constant.ProjectStatusPreLaunch, constant.ProjectStatusSet, constant.ProjectStatusSetup, constant.ProjectStatusWarmup:
	default:
		return "", fmt.Errorf("project status %s cannot be set", project.Status)
	}
	metadata := map[string]any{}
	if strings.TrimSpace(project.OnchainJSON) != "" {
		if err := json.Unmarshal([]byte(project.OnchainJSON), &metadata); err != nil {
			return "", errors.New("project onchain metadata is invalid")
		}
	}
	metadata["fairLaunch"] = map[string]any{
		"saleTokenAmountRaw": preview.SaleTokenAmount.Raw, "softCapRaw": preview.SoftCap.Raw, "minContributionRaw": preview.MinContribution.Raw, "maxContributionRaw": preview.MaxContribution.Raw,
		"projectProceedsReceiver": req.ProjectProceedsReceiver, "projectProceedsReceiverAttestationDigest": strip0x(fmt.Sprint(preview.ProjectProceedsReceiver["attestation_digest"])), "projectProceedsReceiverAttestationVersion": preview.ProjectProceedsReceiver["attestation_version"],
		"platformFeeReceiverAttestationDigest": strip0x(fmt.Sprint(preview.PlatformFee["receiver_attestation_digest"])), "platformFeeReceiverAttestationVersion": preview.PlatformFee["receiver_attestation_version"],
		"projectTokenPolicyDigest": strip0x(fmt.Sprint(preview.ProjectToken["policy_digest"])), "projectTokenPolicyVersion": preview.ProjectToken["policy_version"],
		"paymentTokenPolicyDigest": strip0x(fmt.Sprint(preview.PaymentToken["policy_digest"])), "paymentTokenPolicyVersion": preview.PaymentToken["policy_version"],
		"releasePolicyDigest": strip0x(fmt.Sprint(preview.ReleasePolicy["digest"])), "releasePolicyVersion": preview.ReleasePolicy["version"], "canonicalDigest": preview.CanonicalDigest,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	start, _ := time.Parse(time.RFC3339, preview.Window["start_utc"])
	end, _ := time.Parse(time.RFC3339, preview.Window["end_utc"])
	soft, _ := decimal.NewFromString(preview.SoftCap.Display)
	max, _ := decimal.NewFromString(preview.MaxContribution.Display)
	updates := map[string]any{"status": constant.ProjectStatusSet, "funding_mode": constant.FundingModeFairLaunch, "contract_version": constant.ContractVersionFairLaunchV1, "chain_id": runtime.ChainID, "contract_address": runtime.ContractAddress, "token_address": req.ProjectToken, "token_symbol": preview.SaleTokenAmount.Symbol, "token_decimals": preview.SaleTokenAmount.Decimals, "payment_token": req.PaymentToken, "payment_token_address": preview.PaymentToken["address"], "payment_token_decimals": preview.PaymentToken["decimals"], "payment_token_policy_revision": preview.PaymentToken["policy_version"], "project_token_policy_digest": strip0x(fmt.Sprint(preview.ProjectToken["policy_digest"])), "project_token_runtime_code_hash": strip0x(fmt.Sprint(preview.ProjectToken["runtime_code_hash"])), "owner_address": req.ProjectOwner, "fee_receiver": runtime.PlatformFeeReceiver, "success_fee_rate": runtime.PlatformFeeBps, "token_price": decimal.Zero, "token_price_raw": "0", "hard_cap": decimal.Zero, "hard_cap_raw": "0", "sale_token_amount_raw": preview.SaleTokenAmount.Raw, "soft_cap": soft, "soft_cap_raw": preview.SoftCap.Raw, "min_contribution_raw": preview.MinContribution.Raw, "max_contribution_raw": preview.MaxContribution.Raw, "max_per_user": max, "max_per_user_raw": preview.MaxContribution.Raw, "start_time": start, "end_time": end, "onchain_json": string(metadataJSON), "tge_time": end}
	res := tx.Model(&entity.Project{}).Where("project_id = ? AND onchain_locked = 0", projectID).Updates(updates)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected != 1 {
		return "", errors.New("project changed while applying Fair Launch set")
	}
	if err := tx.Where("project_id = ?", projectID).Delete(&entity.ProjectTge{}).Error; err != nil {
		return "", err
	}
	for i, row := range req.Vesting {
		unlock, _ := time.Parse(time.RFC3339, row.UnlockTime)
		percent := decimal.NewFromInt(int64(row.BPS)).Div(decimal.NewFromInt(100))
		tge := entity.ProjectTge{ProjectID: int(projectID), Index: i + 1, TgePercent: percent, VestingPercent: percent, VestingTime: unlock, VestingInterval: 0, VestingDuration: 0, IsNotification: 1}
		if err := tx.Create(&tge).Error; err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("fair launch project %d set with digest %s", projectID, preview.CanonicalDigest), nil
}
func strip0x(v string) string {
	if len(v) == 66 && v[:2] == "0x" {
		return v[2:]
	}
	return v
}
