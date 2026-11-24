package project

import (
	"errors"
	"gorm.io/gorm"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
	"strings"
	"time"
)

type FairLaunchPublicRuntimeIdentity struct {
	ChainID                uint64 `json:"chainId"`
	ProxyAddress           string `json:"proxyAddress"`
	ImplementationAddress  string `json:"implementationAddress"`
	ImplementationCodeHash string `json:"implementationCodeHash"`
	SourceRegistryRevision uint64 `json:"sourceRegistryRevision"`
	SourceRegistryDigest   string `json:"sourceRegistryDigest"`
	ObservedBlockNumber    string `json:"observedBlockNumber"`
	ObservedBlockHash      string `json:"observedBlockHash"`
	ObservedAt             string `json:"observedAt"`
	RuntimeVerified        bool   `json:"runtimeVerified"`
}

type FairLaunchPublicAssetIdentity struct {
	Address  string `json:"address"`
	Decimals uint8  `json:"decimals"`
}

type FairLaunchPublicAssets struct {
	PaymentToken FairLaunchPublicAssetIdentity `json:"paymentToken"`
	ProjectToken FairLaunchPublicAssetIdentity `json:"projectToken"`
}

type FairLaunchPositionResponse struct {
	ProjectID       string                          `json:"projectId"`
	Wallet          string                          `json:"wallet"`
	FundingMode     string                          `json:"fundingMode"`
	ContractVersion string                          `json:"contractVersion"`
	ChainStatus     string                          `json:"chainStatus"`
	Assets          FairLaunchPublicAssets          `json:"assets"`
	Projection      FairLaunchPositionProjection    `json:"projection"`
	Actions         map[string]map[string]any       `json:"actions"`
	Runtime         FairLaunchPublicRuntimeIdentity `json:"runtime"`
	Source          map[string]any                  `json:"source"`
}

func ReadFairLaunchPosition(db *gorm.DB, projectID uint64, wallet string, r projectruntime.Runtime) (FairLaunchPositionResponse, error) {
	if !r.Enabled {
		return FairLaunchPositionResponse{}, errors.New("fair launch runtime disabled")
	}
	project := entity.Project{}
	if err := db.Where("project_id=? AND funding_mode=? AND contract_version=?", projectID, constant.FundingModeFairLaunch, constant.ContractVersionFairLaunchV1).Take(&project).Error; err != nil {
		return FairLaunchPositionResponse{}, err
	}
	if project.ChainID != r.ChainID || !strings.EqualFold(project.ContractAddress, r.ContractAddress) {
		return FairLaunchPositionResponse{}, errors.New("project runtime mismatch")
	}
	position := entity.FairLaunchPosition{}
	paymentAddress := strings.ToLower(prefixedHex(project.PaymentTokenAddress))
	projectAddress := strings.ToLower(prefixedHex(project.TokenAddress))
	err := db.Where("chain_id=? AND contract_address=? AND contract_version=? AND project_id=? AND wallet_address=? AND payment_token_address=? AND payment_token_decimals=? AND project_token_address=? AND project_token_decimals=? AND status='canonical'", project.ChainID, strings.ToLower(project.ContractAddress), project.ContractVersion, projectID, strings.ToLower(wallet), paymentAddress, uint8(project.PaymentTokenDecimals), projectAddress, uint8(project.TokenDecimals)).Take(&position).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var mismatched int64
		if countErr := db.Model(&entity.FairLaunchPosition{}).Where("chain_id=? AND contract_address=? AND contract_version=? AND project_id=? AND wallet_address=? AND status='canonical'", project.ChainID, strings.ToLower(project.ContractAddress), project.ContractVersion, projectID, strings.ToLower(wallet)).Count(&mismatched).Error; countErr != nil {
			return FairLaunchPositionResponse{}, countErr
		}
		if mismatched != 0 {
			return FairLaunchPositionResponse{}, errors.New("fair launch position token identity mismatch")
		}
		position = entity.FairLaunchPosition{PaymentTokenAddress: paymentAddress, ProjectTokenAddress: projectAddress, ContributionAmountRaw: "0", ClaimedTokensRaw: "0", Refunded: false, PaymentTokenDecimals: uint8(project.PaymentTokenDecimals), ProjectTokenDecimals: uint8(project.TokenDecimals)}
	} else if err != nil {
		return FairLaunchPositionResponse{}, err
	}
	status := normalizeFairChainStatus(project.Status)
	projection, err := BuildFairLaunchPositionProjection(FairLaunchPositionInput{ChainStatus: status, ContributionRaw: position.ContributionAmountRaw, ClaimedRaw: position.ClaimedTokensRaw, TotalRaisedRaw: project.TotalRaisedRaw, FinalTotalRaisedRaw: project.FinalTotalRaisedRaw, SaleTokenAmountRaw: project.SaleTokenAmountRaw, PaymentDecimals: uint8(project.PaymentTokenDecimals), ProjectDecimals: uint8(project.TokenDecimals), Refunded: position.Refunded})
	if err != nil {
		return FairLaunchPositionResponse{}, err
	}
	reconciliationState, err := fairLaunchReconciliationState(db, projectID)
	if err != nil {
		return FairLaunchPositionResponse{}, err
	}
	evidence, err := ReadFairLaunchRuntimeEvidence(db, r, time.Now().UTC())
	if err != nil {
		return FairLaunchPositionResponse{}, err
	}
	runtime := evidence.Runtime
	// Wallet writes require a separately reviewed durable submission authority.
	// It is not implemented in this release, so every public wallet-write
	// capability remains explicitly fail closed instead of trusting browser state.
	actions := fairLaunchPublicWalletActions()
	source := map[string]any{}
	for key, value := range evidence.Source {
		source[key] = value
	}
	source["reconciliationState"] = reconciliationState
	assets := FairLaunchPublicAssets{
		PaymentToken: FairLaunchPublicAssetIdentity{Address: prefixedHex(project.PaymentTokenAddress), Decimals: uint8(project.PaymentTokenDecimals)},
		ProjectToken: FairLaunchPublicAssetIdentity{Address: prefixedHex(project.TokenAddress), Decimals: uint8(project.TokenDecimals)},
	}
	return FairLaunchPositionResponse{ProjectID: projectIDString(projectID), Wallet: strings.ToLower(wallet), FundingMode: constant.FundingModeFairLaunch, ContractVersion: constant.ContractVersionFairLaunchV1, ChainStatus: status, Assets: assets, Projection: projection, Actions: actions, Runtime: runtime, Source: source}, nil
}
func fairLaunchPublicWalletActions() map[string]map[string]any {
	const reason = "durable_wallet_submission_unavailable"
	actions := make(map[string]map[string]any, 8)
	for _, action := range []string{"contribute", "claim", "refund", "depositProjectTokens", "withdrawFailedProjectTokens", "withdrawProjectProceeds", "withdrawPlatformFee"} {
		actions[action] = map[string]any{"available": false, "reason": reason}
	}
	actions["release"] = map[string]any{"available": false, "reason": "operator_only"}
	return actions
}

func ReadFairLaunchPortfolio(db *gorm.DB, wallet string, r projectruntime.Runtime) ([]FairLaunchPositionResponse, error) {
	if !r.Enabled {
		return nil, errors.New("fair launch runtime disabled")
	}
	var positions []entity.FairLaunchPosition
	if err := db.Where("chain_id=? AND contract_address=? AND contract_version=? AND wallet_address=? AND status='canonical'", r.ChainID, strings.ToLower(r.ContractAddress), constant.ContractVersionFairLaunchV1, strings.ToLower(wallet)).Order("project_id ASC, id ASC").Find(&positions).Error; err != nil {
		return nil, err
	}
	out := make([]FairLaunchPositionResponse, 0, len(positions))
	for _, p := range positions {
		item, err := ReadFairLaunchPosition(db, p.ProjectID, wallet, r)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
func ReadFairLaunchRuntimeEvidence(db *gorm.DB, r projectruntime.Runtime, now time.Time) (FairLaunchRuntimeEvidence, error) {
	if !r.Enabled {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_disabled")
	}
	if db == nil {
		return FairLaunchRuntimeEvidence{}, errors.New("runtime_unverified")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return ReadFairLaunchRuntimeEvidenceTx(db, r, now.UTC(), false)
}

func fairLaunchPositionTokenIdentityMatches(position entity.FairLaunchPosition, project entity.Project) bool {
	return strings.EqualFold(prefixedHex(position.PaymentTokenAddress), prefixedHex(project.PaymentTokenAddress)) &&
		position.PaymentTokenDecimals == uint8(project.PaymentTokenDecimals) &&
		strings.EqualFold(prefixedHex(position.ProjectTokenAddress), prefixedHex(project.TokenAddress)) &&
		position.ProjectTokenDecimals == uint8(project.TokenDecimals)
}

func prefixedHex(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" && !strings.HasPrefix(value, "0x") {
		return "0x" + value
	}
	return value
}

func fairLaunchReconciliationState(db *gorm.DB, projectID uint64) (string, error) {
	var state string
	err := db.Table("ido_reconciliation_run").Select("state").Where("project_id=?", projectID).Order("id DESC").Limit(1).Scan(&state).Error
	if err != nil {
		return "", err
	}
	if state == "" {
		return "unavailable", nil
	}
	return strings.ToLower(state), nil
}

func normalizeFairChainStatus(status string) string {
	switch strings.ToLower(status) {
	case "pending":
		return FairLaunchChainStatusPending
	case "active":
		return FairLaunchChainStatusActive
	case "successfulpendingrelease", "successful_pending_release":
		return FairLaunchChainStatusSuccessfulPendingRelease
	case "successful":
		return FairLaunchChainStatusSuccessful
	case "failed":
		return FairLaunchChainStatusFailed
	case "cancelled", "canceled":
		return FairLaunchChainStatusCancelled
	default:
		return FairLaunchChainStatusNotCreated
	}
}
func projectIDString(id uint64) string {
	const digits = "0123456789"
	if id == 0 {
		return "0"
	}
	b := make([]byte, 0, 20)
	for id > 0 {
		b = append(b, digits[id%10])
		id /= 10
	}
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}
