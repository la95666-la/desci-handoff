package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projecttracking"
	"joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
)

// projectListRow admin 项目列表返回的精简行（不带 story_html / research_html 等大字段，
// 避免列表页拉一堆 mediumtext）
type projectListRow struct {
	ID                   uint64                    `json:"id"`
	ProjectID            string                    `json:"project_id"`
	NameEn               string                    `json:"name_en"`
	NameCn               string                    `json:"name_cn"`
	Tagline              string                    `json:"tagline"`
	Category             string                    `json:"category"`
	Status               string                    `json:"status"`
	PreArchiveStatus     string                    `json:"pre_archive_status"`
	RestoreAllowed       bool                      `json:"restore_allowed"`
	RestoreBlocker       string                    `json:"restore_blocker,omitempty"`
	ContractVersion      string                    `json:"contract_version"`
	FundingMode          string                    `json:"funding_mode"`
	ChainStatus          string                    `json:"chain_status,omitempty"`
	SaleTokenAmount      string                    `json:"sale_token_amount,omitempty"`
	SaleTokenAmountRaw   string                    `json:"sale_token_amount_raw,omitempty"`
	SoftCap              string                    `json:"soft_cap,omitempty"`
	SoftCapRaw           string                    `json:"soft_cap_raw,omitempty"`
	OnchainLocked        bool                      `json:"onchain_locked"`
	ActiveSubmission     *projectSubmissionSummary `json:"active_submission"`
	IsListed             bool                      `json:"is_listed"`
	LaunchAt             *string                   `json:"launch_at"` // RFC3339 or nil
	EndTime              *string                   `json:"end_time"`  // RFC3339 or nil
	PlatformFeeWithdrawn int                       `json:"platform_fee_withdrawn"`
	CoverURL             string                    `json:"cover_url"`
	CreatedAt            string                    `json:"created_at"`
	UpdatedAt            string                    `json:"updated_at"`
	Tracking             *projecttracking.Row      `json:"tracking,omitempty"`
}

type projectSubmissionSummary struct {
	ID     uint64  `json:"id"`
	Action string  `json:"action"`
	State  string  `json:"state"`
	TxHash *string `json:"tx_hash"`
}

// ListProjects GET /admin/v1/projects?page=&size=&status=&keyword=
// 分页 + 状态筛选 + 名称模糊搜索
func ListProjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	status := strings.TrimSpace(c.Query("status"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	q := db.Mysql().Model(&entity.Project{}).
		Select("id, project_id, name_en, name_cn, tagline, category, status, pre_archive_status, funding_mode, contract_version, onchain_locked, token_decimals, payment_token_decimals, soft_cap_raw, onchain_json, is_listed, launch_at, end_time, platform_fee_withdrawn, cover_url, created_at, updated_at")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("name_en LIKE ? OR name_cn LIKE ? OR tagline LIKE ?", kw, kw, kw)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_LIST_FAILED", "message": err.Error()})
		return
	}

	var rows []struct {
		entity.Project
	}
	if err := q.
		Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_LIST_FAILED", "message": err.Error()})
		return
	}

	items := make([]projectListRow, 0, len(rows))
	projectIDs := make([]uint64, 0, len(rows))
	v3FeeIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		projectIDs = append(projectIDs, uint64(row.ProjectID))
		if row.ContractVersion == constant.ContractVersionIDOV3Stablecoin && row.FundingMode == constant.FundingModeCappedPresale && row.OnchainLocked {
			v3FeeIDs = append(v3FeeIDs, uint64(row.ProjectID))
		}
	}
	v3FeeWithdrawn, err := readV3PlatformFeeWithdrawn(c.Request.Context(), db.Mysql(), v3FeeIDs)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_PLATFORM_FEE_STATUS_UNAVAILABLE"})
		return
	}
	activeByProject := make(map[uint64]*projectSubmissionSummary, len(projectIDs))
	if len(projectIDs) != 0 {
		submissions := make([]entity.ChainTxSubmission, 0, len(projectIDs))
		if err := db.Mysql().Select("id,project_id,action,state,tx_hash").Where("project_id IN ? AND active_slot = 1", projectIDs).Order("id DESC").Find(&submissions).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_LIST_FAILED", "message": err.Error()})
			return
		}
		for _, submission := range submissions {
			if activeByProject[submission.ProjectID] == nil {
				activeByProject[submission.ProjectID] = &projectSubmissionSummary{ID: submission.ID, Action: submission.Action, State: submission.State, TxHash: submission.TxHash}
			}
		}
	}
	for _, r := range rows {
		feeWithdrawn := r.PlatformFeeWithdrawn
		if r.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
			feeWithdrawn = 0 // V2 compatibility flag is not a V3 payment fact.
			if v3FeeWithdrawn[uint64(r.ProjectID)] {
				feeWithdrawn = 1
			}
		}
		var tracking *projecttracking.Row
		if projecttracking.Enabled() && r.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
			tr, err := projecttracking.Read(c.Request.Context(), db.Mysql(), uint64(r.ProjectID), false)
			if err != nil {
				c.JSON(409, gin.H{"status": false, "code": "TRACKING_UNAVAILABLE"})
				return
			}
			tr.Finished = tr.Finished && tr.EvidenceCurrent && tr.WakeBlock == 0 && !tr.Urgent
			tracking = &tr
		}
		funding, err := projectService.ProjectFundingReadProjection(r.Project)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_FUNDING_IDENTITY_INVALID", "message": err.Error()})
			return
		}
		var launchAt *string
		if r.LaunchAt != nil {
			s := r.LaunchAt.Format("2006-01-02T15:04:05Z07:00")
			launchAt = &s
		}
		var endTime *string
		if !r.EndTime.IsZero() {
			s := r.EndTime.Format("2006-01-02T15:04:05Z07:00")
			endTime = &s
		}
		restoreAllowed, restoreBlocker := projectRestoreCapability(r.Project, activeByProject[uint64(r.ProjectID)] != nil)
		items = append(items, projectListRow{
			ID:                   r.ID,
			ProjectID:            strconv.FormatInt(int64(r.ProjectID), 10),
			NameEn:               r.NameEn,
			NameCn:               r.NameCn,
			Tagline:              r.Tagline,
			Category:             r.Category,
			Status:               r.Status,
			PreArchiveStatus:     r.PreArchiveStatus,
			RestoreAllowed:       restoreAllowed,
			RestoreBlocker:       restoreBlocker,
			ContractVersion:      r.ContractVersion,
			FundingMode:          funding.FundingMode,
			ChainStatus:          funding.ChainStatus,
			SaleTokenAmount:      funding.SaleTokenAmount,
			SaleTokenAmountRaw:   funding.SaleTokenAmountRaw,
			SoftCap:              funding.SoftCap,
			SoftCapRaw:           funding.SoftCapRaw,
			OnchainLocked:        r.OnchainLocked,
			ActiveSubmission:     activeByProject[uint64(r.ProjectID)],
			IsListed:             r.IsListed == 1,
			LaunchAt:             launchAt,
			EndTime:              endTime,
			PlatformFeeWithdrawn: feeWithdrawn,
			CoverURL:             r.CoverURL,
			CreatedAt:            r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:            r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Tracking:             tracking,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": size,
		},
	})
}

type projectListingVisibilityReq struct {
	IsListed *bool `json:"is_listed"`
}

// UpdateProjectListingVisibility PUT /admin/v1/projects/:id/listing-visibility
// Changes only public list discovery. The project lifecycle, direct detail URL,
// chain actions, purchases, refunds and all other project behavior are unchanged.
func UpdateProjectListingVisibility(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	var req projectListingVisibilityReq
	if err := c.ShouldBindJSON(&req); err != nil || req.IsListed == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": "is_listed must be boolean"})
		return
	}

	var row entity.Project
	if err := db.Mysql().Select("id, project_id, is_listed").Where("id = ? OR project_id = ?", id, id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_LISTING_UPDATE_FAILED", "message": err.Error()})
		return
	}

	target := 0
	if *req.IsListed {
		target = 1
	}
	if row.IsListed != target {
		result := db.Mysql().Model(&entity.Project{}).
			Where("id = ? AND is_listed = ?", row.ID, row.IsListed).
			Update("is_listed", target)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_LISTING_UPDATE_FAILED", "message": result.Error.Error()})
			return
		}
		if result.RowsAffected != 1 {
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_LISTING_CONFLICT", "message": "project listing visibility changed concurrently"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"project_id": strconv.FormatInt(int64(row.ProjectID), 10),
		"is_listed":  *req.IsListed,
	}})
}

func projectRestoreCapability(project entity.Project, hasActiveSubmission bool) (bool, string) {
	if project.Status != "archived" {
		return false, "PROJECT_RESTORE_NOT_ARCHIVED"
	}
	if projectHasChainManagedLifecycle(project) {
		return false, "PROJECT_RESTORE_LIFECYCLE_LOCKED"
	}
	if hasActiveSubmission {
		return false, "PROJECT_RESTORE_ACTIVE_SUBMISSION"
	}
	switch project.PreArchiveStatus {
	case "Setup", "Set", "pre_launch", "warmup":
		return true, ""
	default:
		return false, "PROJECT_RESTORE_STATE_UNKNOWN"
	}
}

// Content management must not mutate chain-managed lifecycle state, even
// before the first on-chain lock. V3 has no generic archive/restore path.
func projectHasChainManagedLifecycle(project entity.Project) bool {
	return project.OnchainLocked || project.ContractVersion == constant.ContractVersionV2Stablecoin ||
		project.ContractVersion == constant.ContractVersionIDOV3Stablecoin || project.ContractVersion == constant.ContractVersionFairLaunchV1
}
