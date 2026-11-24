package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
)

// Setup policy is a draft in display units: payment decimals and max-per-user
// are not known until Set. It never becomes effective until Set validates it.
func canSaveSetupMinPurchase(project entity.Project) bool {
	return project.Status == constant.ProjectStatusSetup &&
		project.FundingMode == constant.FundingModeCappedPresale &&
		project.ContractVersion == constant.ContractVersionUnassigned && !project.OnchainLocked
}

func validateDraftMinPurchase(value, symbol string) (*string, *string, error) {
	value = strings.TrimSpace(value)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if value == "" {
		return nil, nil, nil
	}
	if symbol != constant.PaymentTokenUSDT && symbol != constant.PaymentTokenUSDC {
		return nil, nil, errors.New("select USDT or USDC for the Setup minimum")
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && ch != '.' {
			return nil, nil, errors.New("minimum purchase must be a plain positive decimal")
		}
	}
	amount, err := decimal.NewFromString(value)
	if err != nil || !amount.IsPositive() {
		return nil, nil, errors.New("minimum purchase must be a positive decimal")
	}
	canonical := amount.String()
	if len(canonical) > 78 {
		return nil, nil, errors.New("minimum purchase is too long")
	}
	return &canonical, &symbol, nil
}

// Off-chain UI policy only. Never use this value to reject an accepted chain event.
func validateMinPurchase(value string, project entity.Project) (*string, error) {
	value = strings.TrimSpace(value)
	// UI-only policy: V2 and V3 have explicit independent identity admission.
	// Require both independently persisted identity fields before *any* write.
	isV3 := project.FundingMode == constant.FundingModeCappedPresale && project.ContractVersion == constant.ContractVersionIDOV3Stablecoin
	if !constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) && !isV3 {
		return nil, errors.New("minimum purchase is only available for exact V2 or V3 capped-presale projects")
	}
	if value == "" {
		return nil, nil
	}
	if project.PaymentTokenDecimals < 1 || project.PaymentTokenDecimals > 18 {
		return nil, errors.New("minimum purchase requires payment token decimals between 1 and 18")
	}
	// Do not silently accept scientific notation or rounded display amounts.
	for _, ch := range value {
		if (ch < '0' || ch > '9') && ch != '.' {
			return nil, errors.New("minimum purchase must be a plain positive decimal")
		}
	}
	amount, err := decimal.NewFromString(value)
	if err != nil || !amount.IsPositive() {
		return nil, errors.New("minimum purchase must be a positive decimal")
	}
	raw, err := money.DisplayToRaw(amount, uint8(project.PaymentTokenDecimals))
	if err != nil {
		return nil, err
	}
	max, err := money.NormalizeRawUint256(project.MaxPerUserRaw)
	if err != nil {
		return nil, errors.New("invalid on-chain per-user maximum")
	}
	maxAmount, _ := decimal.NewFromString(max)
	rawAmount, _ := decimal.NewFromString(raw)
	if rawAmount.GreaterThan(maxAmount) {
		return nil, errors.New("minimum purchase exceeds on-chain per-user maximum")
	}
	return &raw, nil
}

// UpdateProjectMinPurchase PUT /admin/v1/projects/:id/min-purchase
// Body min_purchase_amount: empty string clears the policy.
func UpdateProjectMinPurchase(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "invalid project id"})
		return
	}
	var req struct {
		Amount       *string `json:"min_purchase_amount"`
		PaymentToken string  `json:"payment_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "min_purchase_amount is required"})
		return
	}
	var raw *string
	var draft *string
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		var row entity.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? OR project_id = ?", id, id).Take(&row).Error; err != nil {
			return err
		}
		if canSaveSetupMinPurchase(row) {
			amount, token, err := validateDraftMinPurchase(*req.Amount, req.PaymentToken)
			if err != nil {
				return err
			}
			draft = amount
			return tx.Model(&entity.Project{}).Where("id = ? AND status = ? AND onchain_locked = ?", row.ID, row.Status, false).
				Updates(map[string]any{"min_purchase_draft_amount": amount, "min_purchase_draft_token": token, "min_purchase_amount_raw": nil}).Error
		}
		var validationErr error
		raw, validationErr = validateMinPurchase(*req.Amount, row)
		if validationErr != nil {
			return validationErr
		}
		return tx.Model(&entity.Project{}).Where("id = ?", row.ID).
			Updates(map[string]any{"min_purchase_amount_raw": raw, "min_purchase_draft_amount": nil, "min_purchase_draft_token": nil}).Error
	})
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"status": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"min_purchase_amount_raw": raw, "min_purchase_draft_amount": draft}})
}
