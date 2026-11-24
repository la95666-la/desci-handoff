package controller

import (
	"strconv"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	projectService "joincare_server/pkg/core/service/project"
)

// projectContentResponse keeps the persisted content payload while exposing
// exact, derived funding read fields without introducing an unapplied column.
type projectContentResponse struct {
	entity.Project
	// Shadow the embedded numeric field so JavaScript never rounds Sonyflake IDs.
	ProjectID              string `json:"project_id"`
	FundingMode            string `json:"funding_mode"`
	ChainStatus            string `json:"chain_status,omitempty"`
	SaleTokenAmount        string `json:"sale_token_amount,omitempty"`
	SaleTokenAmountRaw     string `json:"sale_token_amount_raw,omitempty"`
	SoftCap                string `json:"soft_cap,omitempty"`
	SoftCapRaw             string `json:"soft_cap_raw,omitempty"`
	MinPurchaseAmount      string `json:"min_purchase_amount"`
	MinPurchaseDraftAmount string `json:"min_purchase_draft_amount"`
	MinPurchaseDraftToken  string `json:"min_purchase_draft_token"`
}

func newProjectContentResponse(row entity.Project) (projectContentResponse, error) {
	funding, err := projectService.ProjectFundingReadProjection(row)
	if err != nil {
		return projectContentResponse{}, err
	}
	// An unassigned Setup project has no on-chain funding formula yet, so the
	// read projection intentionally returns no derived mode. The Admin editor
	// still needs its explicitly persisted capped-presale intent to expose the
	// pre-Set purchase-policy draft. Do not infer a mode for other versions.
	fundingMode := funding.FundingMode
	if row.ContractVersion == constant.ContractVersionUnassigned &&
		row.Status == constant.ProjectStatusSetup &&
		row.FundingMode == constant.FundingModeCappedPresale {
		fundingMode = row.FundingMode
	}
	minAmount := ""
	if row.MinPurchaseAmountRaw != nil {
		minAmount, err = money.FormatRaw(*row.MinPurchaseAmountRaw, uint8(row.PaymentTokenDecimals))
		if err != nil {
			return projectContentResponse{}, err
		}
	}
	draftAmount, draftToken := "", ""
	if row.MinPurchaseDraftAmount != nil {
		draftAmount = *row.MinPurchaseDraftAmount
	}
	if row.MinPurchaseDraftToken != nil {
		draftToken = *row.MinPurchaseDraftToken
	}
	return projectContentResponse{
		Project:                row,
		ProjectID:              strconv.FormatInt(int64(row.ProjectID), 10),
		FundingMode:            fundingMode,
		ChainStatus:            funding.ChainStatus,
		SaleTokenAmount:        funding.SaleTokenAmount,
		SaleTokenAmountRaw:     funding.SaleTokenAmountRaw,
		SoftCap:                funding.SoftCap,
		SoftCapRaw:             funding.SoftCapRaw,
		MinPurchaseAmount:      minAmount,
		MinPurchaseDraftAmount: draftAmount,
		MinPurchaseDraftToken:  draftToken,
	}, nil
}
