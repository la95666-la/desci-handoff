package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

const chainTxSafeSelect = "id,project_id,action,attempt_no,chain_id,sender_address,contract_address,value_raw,state,nonce,gas_limit,gas_price_raw,tx_hash,broadcast_count,last_broadcast_at,last_error,receipt_status,receipt_block_number,receipt_block_hash,receipt_gas_used,final_event_record_id,finalized_at,cancelled_by_admin_id,cancel_reason,cancelled_at,created_at,updated_at,CASE WHEN action IN ('v2_create','v2_activate') AND state IN ('intent','ready') AND active_slot = 1 AND nonce IS NULL AND gas_limit IS NULL AND raw_tx IS NULL AND tx_hash IS NULL THEN TRUE ELSE FALSE END AS admin_cancellable"

var errAdminProjectExecuteRevoked = errors.New("administrator is disabled, deleted, or no longer has project execute permission")

// ListChainTransactions GET /admin/v1/chain-transactions
func ListChainTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := db.Mysql().Model(&entity.ChainTxSubmission{})
	if state := strings.TrimSpace(c.Query("state")); state != "" {
		if !validChainTxState(state) {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_TX_BAD_STATE", "message": "invalid chain transaction state"})
			return
		}
		query = query.Where("state = ?", state)
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		if !validChainTxAction(action) {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_TX_BAD_ACTION", "message": "invalid chain transaction action"})
			return
		}
		query = query.Where("action = ?", action)
	}
	if projectID := strings.TrimSpace(c.Query("project_id")); projectID != "" {
		parsed, err := strconv.ParseUint(projectID, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_TX_BAD_PROJECT_ID", "message": "invalid project id"})
			return
		}
		query = query.Where("project_id = ?", parsed)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_TX_LIST_FAILED", "message": err.Error()})
		return
	}
	rows := make([]entity.ChainTxSubmission, 0, pageSize)
	if err := query.Select(chainTxSafeSelect).Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_TX_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize}})
}

// GetChainTransaction GET /admin/v1/chain-transactions/:id
func GetChainTransaction(c *gin.Context) {
	id, ok := chainTxID(c)
	if !ok {
		return
	}
	record := entity.ChainTxSubmission{}
	if err := db.Mysql().Select(chainTxSafeSelect).Where("id = ?", id).Take(&record).Error; err != nil {
		chainTxReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": record})
}

// ChainTransactionSummary GET /admin/v1/chain-transactions/summary
func ChainTransactionSummary(c *gin.Context) {
	counts := make([]struct {
		State string `json:"state"`
		Count int64  `json:"count"`
	}, 0, 12)
	if err := db.Mysql().Model(&entity.ChainTxSubmission{}).Select("state, count(*) AS count").Group("state").Scan(&counts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_TX_SUMMARY_FAILED", "message": err.Error()})
		return
	}
	staleBefore := time.Now().Add(-3 * time.Minute)
	var stale int64
	if err := db.Mysql().Model(&entity.ChainTxSubmission{}).Where("active_slot = 1 AND created_at < ?", staleBefore).Count(&stale).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_TX_SUMMARY_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"state_counts": counts, "stale_active": stale, "stale_after_seconds": 180}})
}

// CancelChainTransaction POST /admin/v1/chain-transactions/:id/cancel
func CancelChainTransaction(c *gin.Context) {
	id, ok := chainTxID(c)
	if !ok {
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_TX_CANCEL_REASON_REQUIRED", "message": "cancellation reason is required"})
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	reasonLength := utf8.RuneCountInString(request.Reason)
	if reasonLength < 3 || reasonLength > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_TX_CANCEL_REASON_INVALID", "message": "cancellation reason must be 3-500 characters"})
		return
	}
	adminID := currentAdminID(c)
	if adminID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "CHAIN_TX_ADMIN_REQUIRED", "message": "authenticated administrator is required"})
		return
	}
	err := cancelChainTransaction(id, adminID, request.Reason)
	if err != nil {
		status, code := http.StatusConflict, "CHAIN_TX_NOT_CANCELLABLE"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status, code = http.StatusNotFound, "CHAIN_TX_NOT_FOUND"
		} else if errors.Is(err, errAdminProjectExecuteRevoked) {
			status, code = http.StatusForbidden, "CHAIN_TX_ADMIN_REVOKED"
		} else if !isChainTxConflict(err) {
			status, code = http.StatusInternalServerError, "CHAIN_TX_CANCEL_FAILED"
		}
		c.JSON(status, gin.H{"status": false, "code": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "state": constant.ChainTxStateAdminCancelled, "reason": request.Reason}})
}

func cancelChainTransaction(id, adminID uint64, reason string) error {
	preview := entity.ChainTxSubmission{}
	if err := db.Mysql().Select("id,project_id,action").Where("id = ?", id).Take(&preview).Error; err != nil {
		return err
	}
	if preview.Action != constant.TransactionTypeV2Create && preview.Action != constant.TransactionTypeV2Activate {
		return errors.New("only unsigned V2 create or activate intents may be cancelled")
	}
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := requireCurrentProjectExecuteAdmin(tx, adminID); err != nil {
			return err
		}
		if preview.Action == constant.TransactionTypeV2Create {
			project := entity.Project{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("project_id,status,onchain_locked,funding_mode,contract_version,chain_id,contract_address").Where("project_id = ?", preview.ProjectID).Take(&project).Error; err != nil {
				return err
			}
			record := entity.ChainTxSubmission{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&record).Error; err != nil {
				return err
			}
			if record.ProjectID != uint64(project.ProjectID) || record.Action != constant.TransactionTypeV2Create || record.ChainID != project.ChainID || !strings.EqualFold(record.ContractAddress, project.ContractAddress) {
				return errors.New("V2 create intent does not match the locked project identity")
			}
			if !constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) || project.Status != constant.ProjectStatusSet || !project.OnchainLocked {
				return errors.New("V2 create project is not an exact V2 locked Set project")
			}
			if !chaintx.AdminCancellable(record) {
				return errors.New("chain transaction is not an unsigned cancellable intent")
			}
			if err := chaintx.CancelUnsignedTx(tx, id, adminID, reason); err != nil {
				return err
			}
			result := tx.Model(&entity.Project{}).Where("project_id = ? AND status = ? AND onchain_locked = ?", project.ProjectID, constant.ProjectStatusSet, true).Updates(map[string]interface{}{"onchain_locked": false, "fee_receiver": "", "success_fee_rate": 0})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("V2 create project changed before cancellation")
			}
			return nil
		}
		record := entity.ChainTxSubmission{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&record).Error; err != nil {
			return err
		}
		if record.ProjectID != preview.ProjectID || record.Action != constant.TransactionTypeV2Activate {
			return errors.New("V2 activate intent identity changed before cancellation")
		}
		project := entity.Project{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("project_id,funding_mode,contract_version,chain_id,contract_address").Where("project_id = ?", record.ProjectID).Take(&project).Error; err != nil {
			return err
		}
		if !constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) || record.ChainID != project.ChainID || !strings.EqualFold(record.ContractAddress, project.ContractAddress) {
			return errors.New("V2 activate intent does not match an exact V2 project identity")
		}
		return chaintx.CancelUnsignedTx(tx, id, adminID, reason)
	})
}

func chainTxID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_TX_BAD_ID", "message": "invalid chain transaction id"})
		return 0, false
	}
	return id, true
}

func chainTxReadError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "CHAIN_TX_GET_FAILED"
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status, code = http.StatusNotFound, "CHAIN_TX_NOT_FOUND"
	}
	c.JSON(status, gin.H{"status": false, "code": code, "message": err.Error()})
}

func hasCurrentProjectExecutePermission(tx *gorm.DB, admin adminEntity.AdminUser) (bool, error) {
	roles := make([]string, 0, 4)
	if role := strings.TrimSpace(admin.RoleCode); role != "" {
		roles = append(roles, role)
	}
	joinedRows := make([]adminEntity.AdminUserRole, 0, 4)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,role_code").Where("admin_id = ?", admin.ID).Find(&joinedRows).Error; err != nil {
		return false, err
	}
	for _, row := range joinedRows {
		roles = append(roles, row.RoleCode)
	}
	seen := make(map[string]struct{}, len(roles))
	unique := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		if role == "super_admin" {
			return true, nil
		}
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		unique = append(unique, role)
	}
	if len(unique) == 0 {
		return false, nil
	}
	permissions := make([]adminEntity.AdminRolePermission, 0, len(unique))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,role_code").Where("role_code IN ? AND module = ? AND action = ?", unique, middleware.ModProject, middleware.ActExecute).Find(&permissions).Error; err != nil {
		return false, err
	}
	return len(permissions) > 0, nil
}

func requireCurrentProjectExecuteAdmin(tx *gorm.DB, adminID uint64) error {
	admin := adminEntity.AdminUser{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,is_enable,role_code").Where("id = ? AND is_enable = 1", adminID).Take(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errAdminProjectExecuteRevoked
		}
		return err
	}
	allowed, err := hasCurrentProjectExecutePermission(tx, admin)
	if err != nil {
		return err
	}
	if !allowed {
		return errAdminProjectExecuteRevoked
	}
	return nil
}

func validChainTxAction(action string) bool {
	return action == "ido_v3_create" || action == "ido_v3_activate" || action == "ido_v3_end" || action == "ido_v3_withdraw_platform_fee" || action == constant.TransactionTypeV2Create || action == constant.TransactionTypeV2Activate || action == constant.TransactionTypeProjectEnd ||
		action == constant.TransactionTypeProjectCancel || action == constant.TransactionTypePlatformFee ||
		action == constant.TransactionTypeGlobalPause || action == constant.TransactionTypeGlobalUnpause
}

func validChainTxState(state string) bool {
	switch state {
	case constant.ChainTxStateIntent, constant.ChainTxStateReady, constant.ChainTxStateSigned,
		constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown,
		constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert,
		constant.ChainTxStateFinalSuccess, constant.ChainTxStateFinalRevert,
		constant.ChainTxStateApplied, constant.ChainTxStateCancelled, constant.ChainTxStateAdminCancelled, constant.ChainTxStateAmbiguous:
		return true
	default:
		return false
	}
}

func isChainTxConflict(err error) bool {
	message := err.Error()
	return strings.Contains(message, "not an unsigned cancellable intent") ||
		strings.Contains(message, "only unsigned V2") ||
		strings.Contains(message, "does not match the locked project") ||
		strings.Contains(message, "activate intent identity changed") ||
		strings.Contains(message, "not a locked Set project") ||
		strings.Contains(message, "changed before cancellation") ||
		strings.Contains(message, "nonce fence")
}
