package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
	"joincare_server/pkg/core/service/sanitize"
)

// projectContentPatchReq 用 pointer 语义：nil 跳过；非 nil（含空字符串）覆盖。
// JSON 字段用 json.RawMessage 让 admin 前端直接传对象，backend 存 stringified 到 DB。
type projectContentPatchReq struct {
	NameEn        *string   `json:"name_en"`
	NameCn        *string   `json:"name_cn"`
	Status        *string   `json:"status"`
	Category      *string   `json:"category"`
	CoverURL      *string   `json:"cover_url"`
	CoverVideoURL *string   `json:"cover_video_url"`
	LaunchAt      *string   `json:"launch_at"` // "2026-08-15T10:00:00Z" 或空字符串（清）
	Tagline       *string   `json:"tagline"`
	Description   *string   `json:"description"`
	Tags          *[]string `json:"tags"`
	StoryHTML     *string   `json:"story_html"`
	ResearchHTML  *string   `json:"research_html"`
	RisksHTML     *string   `json:"risks_html"`
	// 双语并列（繁體中文）；pointer 语义，nil 跳过，空字符串主动清空
	TaglineZh      *string         `json:"tagline_zh"`
	StoryHTMLZh    *string         `json:"story_html_zh"`
	ResearchHTMLZh *string         `json:"research_html_zh"`
	RisksHTMLZh    *string         `json:"risks_html_zh"`
	TeamJSON       json.RawMessage `json:"team_json"`
	TokenomicsJSON json.RawMessage `json:"tokenomics_json"`
	RoadmapJSON    json.RawMessage `json:"roadmap_json"`
	DocumentsJSON  json.RawMessage `json:"documents_json"`
	FAQJSON        json.RawMessage `json:"faq_json"`
	OnchainJSON    json.RawMessage `json:"onchain_json"`
}

// validateProtectedOnchainNamespaces keeps system-owned economic metadata out
// of the generic marketing-content editor. Phase 1 has no trusted Set writer
// for this namespace, so this endpoint may preserve it byte-semantically but
// must never add, remove, or change it.
func validateProtectedOnchainNamespaces(current string, requested json.RawMessage) error {
	var currentObject map[string]json.RawMessage
	if strings.TrimSpace(current) != "" {
		if err := json.Unmarshal([]byte(current), &currentObject); err != nil {
			return fmt.Errorf("stored onchain_json is invalid: %w", err)
		}
	}
	var requestedObject map[string]json.RawMessage
	if err := json.Unmarshal(requested, &requestedObject); err != nil {
		return fmt.Errorf("onchain_json must be a JSON object: %w", err)
	}
	// Reserve V3 policy metadata independently of the existing Fair Launch
	// namespace. Marketing edits may retain an existing value but never forge
	// or mutate one; the authoritative V3 policy lives in dedicated tables.
	for _, namespace := range []string{"fairLaunch", "idoV3", "ido_v3"} {
		currentProtected, currentHasProtected := currentObject[namespace]
		requestedProtected, requestedHasProtected := requestedObject[namespace]
		if currentHasProtected != requestedHasProtected {
			return fmt.Errorf("onchain_json.%s is system-managed and cannot be added or removed by content editing", namespace)
		}
		if !currentHasProtected {
			continue
		}
		var currentValue any
		var requestedValue any
		currentDecoder := json.NewDecoder(bytes.NewReader(currentProtected))
		currentDecoder.UseNumber()
		if err := currentDecoder.Decode(&currentValue); err != nil {
			return fmt.Errorf("stored onchain_json.%s is invalid: %w", namespace, err)
		}
		requestedDecoder := json.NewDecoder(bytes.NewReader(requestedProtected))
		requestedDecoder.UseNumber()
		if err := requestedDecoder.Decode(&requestedValue); err != nil {
			return fmt.Errorf("onchain_json.%s must be valid JSON: %w", namespace, err)
		}
		currentCanonical, _ := json.Marshal(currentValue)
		requestedCanonical, _ := json.Marshal(requestedValue)
		if string(currentCanonical) != string(requestedCanonical) {
			return fmt.Errorf("onchain_json.%s is system-managed and cannot be changed by content editing", namespace)
		}
	}
	return nil
}

// GetProjectContent GET /admin/v1/projects/:id/content
// 拉一份完整 project 记录（含新扩字段）供 admin 前端预填表单
func GetProjectContent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	var row entity.Project
	if err := db.Mysql().Model(&entity.Project{}).
		Where("id = ? OR project_id = ?", id, id).
		Take(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": err.Error()})
		return
	}
	response, err := newProjectContentResponse(row)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_FUNDING_IDENTITY_INVALID", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": response})
}

// UpdateProjectContent PUT /admin/v1/projects/:id/content
// Patch 语义：nil 跳过；非 nil 覆盖（含空字符串是主动清空）
func UpdateProjectContent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	var req projectContentPatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.NameEn != nil {
		name := strings.TrimSpace(*req.NameEn)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": "name_en cannot be empty"})
			return
		}
		updates["name_en"] = name
	}
	if req.NameCn != nil {
		name := strings.TrimSpace(*req.NameCn)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": "name_cn cannot be empty"})
			return
		}
		updates["name_cn"] = name
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.CoverURL != nil {
		updates["cover_url"] = *req.CoverURL
	}
	if req.CoverVideoURL != nil {
		updates["cover_video_url"] = *req.CoverVideoURL
	}
	if req.LaunchAt != nil {
		if *req.LaunchAt == "" {
			updates["launch_at"] = nil
		} else {
			t, perr := time.Parse(time.RFC3339, *req.LaunchAt)
			if perr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": "launch_at must be RFC3339 or empty string"})
				return
			}
			updates["launch_at"] = t
		}
	}
	if req.Tagline != nil {
		updates["tagline"] = *req.Tagline
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Tags != nil {
		tags := make([]string, 0, len(*req.Tags))
		for _, tag := range *req.Tags {
			if clean := strings.TrimSpace(tag); clean != "" {
				tags = append(tags, clean)
			}
		}
		encoded, _ := json.Marshal(tags)
		updates["tags"] = string(encoded)
	}
	if req.StoryHTML != nil {
		updates["story_html"] = sanitize.RichHTML(*req.StoryHTML)
	}
	if req.ResearchHTML != nil {
		updates["research_html"] = sanitize.RichHTML(*req.ResearchHTML)
	}
	if req.RisksHTML != nil {
		updates["risks_html"] = sanitize.RichHTML(*req.RisksHTML)
	}
	if req.TaglineZh != nil {
		updates["tagline_zh"] = *req.TaglineZh
	}
	if req.StoryHTMLZh != nil {
		updates["story_html_zh"] = sanitize.RichHTML(*req.StoryHTMLZh)
	}
	if req.ResearchHTMLZh != nil {
		updates["research_html_zh"] = sanitize.RichHTML(*req.ResearchHTMLZh)
	}
	if req.RisksHTMLZh != nil {
		updates["risks_html_zh"] = sanitize.RichHTML(*req.RisksHTMLZh)
	}
	if req.TeamJSON != nil {
		updates["team_json"] = string(req.TeamJSON)
	}
	if req.TokenomicsJSON != nil {
		updates["tokenomics_json"] = string(req.TokenomicsJSON)
	}
	if req.RoadmapJSON != nil {
		updates["roadmap_json"] = string(req.RoadmapJSON)
	}
	if req.DocumentsJSON != nil {
		updates["documents_json"] = string(req.DocumentsJSON)
	}
	if req.FAQJSON != nil {
		updates["faq_json"] = string(req.FAQJSON)
	}
	if req.OnchainJSON != nil {
		updates["onchain_json"] = string(req.OnchainJSON)
	}

	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "changed": 0}})
		return
	}

	var res *gorm.DB
	if req.Status != nil || req.OnchainJSON != nil {
		err = db.Mysql().Transaction(func(tx *gorm.DB) error {
			row := entity.Project{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? OR project_id = ?", id, id).Take(&row).Error; err != nil {
				return err
			}
			if req.OnchainJSON != nil {
				if err := validateProtectedOnchainNamespaces(row.OnchainJSON, req.OnchainJSON); err != nil {
					return err
				}
			}
			if req.Status != nil && projectHasChainManagedLifecycle(row) {
				// Chain-managed projects (including V3 before lock) cannot change lifecycle status here.
				// 若提交的 status 与当前一致（未变更），则去掉 status 仅更新内容字段。
				if *req.Status == row.Status {
					delete(updates, "status")
					if len(updates) == 0 {
						res = &gorm.DB{}
						return nil
					}
					res = tx.Model(&entity.Project{}).Where("id = ?", row.ID).Updates(updates)
					return res.Error
				}
				return errors.New("chain-managed project lifecycle status is event-controlled")
			}
			res = tx.Model(&entity.Project{}).Where("id = ?", row.ID).Updates(updates)
			return res.Error
		})
		if err != nil {
			code := "PROJECT_LIFECYCLE_LOCKED"
			if req.OnchainJSON != nil && strings.Contains(err.Error(), "onchain_json") {
				code = "PROJECT_SYSTEM_METADATA_LOCKED"
			}
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": code, "message": err.Error()})
			return
		}
	} else {
		res = db.Mysql().Model(&entity.Project{}).Where("id = ? OR project_id = ?", id, id).Updates(updates)
	}
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_UPDATE_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		if len(updates) == 0 {
			c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "changed": 0}})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": "no rows affected"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "changed": len(updates)}})
}

// DisableProject POST /admin/v1/projects/:id/disable
// Disabling a project archives it so public launchpad lists no longer include it.
// The pre-archive lifecycle status is recorded in pre_archive_status so that a
// later controlled restore can revive the project to its exact prior state.
func DisableProject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	status, err := archiveProject(db.Mysql(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": "project not found"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_DISABLE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "status": status, "archived": true}})
}

var errProjectArchiveLifecycleLocked = errors.New("chain-managed projects cannot be archived through content management")
var errProjectArchiveAlreadyArchived = errors.New("project is already archived")

// archiveProject archives a non-V2, never-locked project and records its prior
// lifecycle status as the future restore target. It is idempotent: archiving an
// already archived project keeps the recorded pre-archive status untouched.
func archiveProject(database *gorm.DB, id int64) (string, error) {
	archived := constant.ProjectStatusArchived
	err := database.Transaction(func(tx *gorm.DB) error {
		row := entity.Project{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? OR project_id = ?", id, id).Take(&row).Error; err != nil {
			return err
		}
		if projectHasChainManagedLifecycle(row) {
			return errProjectArchiveLifecycleLocked
		}
		if row.Status == constant.ProjectStatusArchived {
			archived = row.Status
			return nil
		}
		updates := map[string]interface{}{"status": constant.ProjectStatusArchived, "pre_archive_status": row.Status}
		result := tx.Model(&entity.Project{}).Where("id = ? AND status = ?", row.ID, row.Status).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("project status changed while archiving")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return archived, nil
}

// RestoreProject POST /admin/v1/projects/:id/restore
// Revives an archived non-V2 project to its recorded pre-archive lifecycle
// status. It never defaults to pre_launch: if the pre-archive status was not
// recorded (missing migration or legacy archive), the request is rejected.
func RestoreProject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	restored, err := restoreProject(db.Mysql(), id)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": "project not found"})
		case errors.Is(err, errProjectRestoreLifecycleLocked):
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_RESTORE_LIFECYCLE_LOCKED", "message": err.Error()})
		case errors.Is(err, errProjectRestoreNotArchived):
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_RESTORE_NOT_ARCHIVED", "message": err.Error()})
		case errors.Is(err, errProjectRestoreUnknownState):
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_RESTORE_STATE_UNKNOWN", "message": err.Error()})
		case errors.Is(err, errProjectRestoreImmutableFacts):
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_RESTORE_IMMUTABLE_FACTS", "message": err.Error()})
		case errors.Is(err, errProjectRestoreConflict):
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_RESTORE_CONFLICT", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_RESTORE_FAILED", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "status": restored, "restored": true}})
}

var (
	errProjectRestoreLifecycleLocked = errors.New("chain-managed projects cannot be restored through content management")
	errProjectRestoreNotArchived     = errors.New("project is not archived and cannot be restored")
	errProjectRestoreUnknownState    = errors.New("pre-archive status was not recorded; refusing to guess a restore target")
	errProjectRestoreImmutableFacts  = errors.New("project has immutable on-chain or economic facts and cannot be restored")
	errProjectRestoreConflict        = errors.New("project changed while restoring")
)

// immutableProjectFactTables mirrors deleteProjectDraft: a project that ever
// touched the chain or real funds must not be silently revived. Archived
// projects should never reach these rows, but defense in depth keeps restore
// from resurrecting a project whose facts cannot be unwound.
var immutableProjectFactTables = []string{"chain_tx_submission", "transaction_record", "user_project", "user_claim_record", "user_refund_record", "fund_ledger", "ido_purchase_entitlement"}

// restoreProject transactions: lock the row, refuse V2 / locked projects,
// require the archived state, resolve the recorded pre-archive status from a
// strict allowlist, refuse projects with immutable facts, then atomically flip
// status and clear the recorded target.
func restoreProject(database *gorm.DB, id int64) (string, error) {
	restored := ""
	err := database.Transaction(func(tx *gorm.DB) error {
		row := entity.Project{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? OR project_id = ?", id, id).Take(&row).Error; err != nil {
			return err
		}
		if projectHasChainManagedLifecycle(row) {
			return errProjectRestoreLifecycleLocked
		}
		if row.Status != constant.ProjectStatusArchived {
			return fmt.Errorf("%w: got %q", errProjectRestoreNotArchived, row.Status)
		}
		target, err := resolveRestoreTarget(row.PreArchiveStatus)
		if err != nil {
			return err
		}
		if err := ensureNoImmutableProjectFacts(tx, uint64(row.ProjectID)); err != nil {
			return err
		}
		result := tx.Model(&entity.Project{}).Where("id = ? AND status = ?", row.ID, constant.ProjectStatusArchived).
			Updates(map[string]interface{}{"status": target, "pre_archive_status": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errProjectRestoreConflict
		}
		restored = target
		return nil
	})
	if err != nil {
		return "", err
	}
	return restored, nil
}

// resolveRestoreTarget validates the recorded pre-archive status. Empty or
// unknown values are rejected: restoring must never invent a default status
// such as pre_launch, because doing so could republish a project in a state it
// was never actually in.
func resolveRestoreTarget(preArchiveStatus string) (string, error) {
	target := strings.TrimSpace(preArchiveStatus)
	switch target {
	case constant.ProjectStatusSetup, constant.ProjectStatusSet, constant.ProjectStatusPreLaunch, constant.ProjectStatusWarmup:
		return target, nil
	default:
		return "", fmt.Errorf("%w: recorded %q", errProjectRestoreUnknownState, preArchiveStatus)
	}
}

func ensureNoImmutableProjectFacts(tx *gorm.DB, projectID uint64) error {
	for _, table := range immutableProjectFactTables {
		var count int64
		if err := tx.Table(table).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("%w: %s has %d rows", errProjectRestoreImmutableFacts, table, count)
		}
	}
	return nil
}

func deleteProjectDraft(database *gorm.DB, id int64) error {
	return database.Transaction(func(tx *gorm.DB) error {
		var row entity.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? OR project_id = ?", id, id).Take(&row).Error; err != nil {
			return err
		}
		if row.ContractVersion != constant.ContractVersionUnassigned || row.OnchainLocked {
			return errors.New("only unassigned, never-locked project drafts may be deleted")
		}
		pid := row.ProjectID
		for _, check := range []struct{ table string }{{"chain_tx_submission"}, {"transaction_record"}, {"user_project"}, {"user_claim_record"}, {"user_refund_record"}, {"fund_ledger"}, {"ido_purchase_entitlement"}} {
			var count int64
			if err := tx.Table(check.table).Where("project_id = ?", pid).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf("project has immutable %s facts", check.table)
			}
		}
		if err := tx.Where("project_id = ?", pid).Delete(&entity.ProjectUpdate{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", pid).Delete(&entity.ProjectSubscriber{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", pid).Delete(&entity.ProjectWatch{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", pid).Delete(&entity.ProjectWhitelist{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", pid).Delete(&entity.ProjectFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", pid).Delete(&entity.ProjectTge{}).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

// DeleteProject DELETE /admin/v1/projects/:id
// This removes the project row and launchpad-side child rows keyed by project_id.
func DeleteProject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}

	err = deleteProjectDraft(db.Mysql(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// ============ 链上参数（Set 阶段专用表单，四眼审批） ============

// PreviewProjectOnchainParams POST /admin/v1/projects/:id/onchain-params/preview
// dry-run 等价物：全部校验 + 换算结果预览，不写库、不进审批。
// 前端必须先展示本预览并经 Admin 确认，才允许调用 submit。
func PreviewProjectOnchainParams(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	var req projectService.OnchainParamsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": err.Error()})
		return
	}
	preview, err := projectService.PrepareOnchainParamsPreview(projectID, req)
	if err != nil {
		if respondV3Diagnostic(c, http.StatusUnprocessableEntity, "PROJECT_ONCHAIN_PARAMS_INVALID", err) {
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "PROJECT_ONCHAIN_PARAMS_INVALID", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": preview})
}

// SubmitProjectOnchainParams POST /admin/v1/projects/:id/onchain-params
// 不直接执行：进四眼审批（另一位 Admin approve 后由 approval handler 落库）。
// 提交前同样做全量校验（防审批期间参数已错），落库时 handler 再校验一次。
// GetProjectOnchainParams GET /admin/v1/projects/:id/onchain-params
// 回显项目当前已保存的 Set 参数（表单回填用）；未设置时 configured=false。
func GetProjectOnchainParams(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	current, err := projectService.GetOnchainParams(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_NOT_FOUND", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": current})
}

func SubmitProjectOnchainParams(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	var req projectService.OnchainParamsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_REQUEST", "message": err.Error()})
		return
	}
	// A V3 preview alone never authorizes Set. In approved mode this submits
	// only a pending request; the distinct approver repeats the chain read.
	var admissionErr error
	if req.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		if projectService.ApprovedV3ModeReady() {
			_, admissionErr = projectService.PrepareOnchainParamsPreview(projectID, req)
		} else {
			admissionErr = errors.New("V3 approved Set prerequisites unavailable")
		}
	} else {
		_, admissionErr = projectService.PrepareOnchainParamsPreview(projectID, req)
	}
	if admissionErr != nil {
		if respondV3Diagnostic(c, http.StatusUnprocessableEntity, "PROJECT_ONCHAIN_PARAMS_INVALID", admissionErr) {
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "PROJECT_ONCHAIN_PARAMS_INVALID", "message": admissionErr.Error()})
		return
	}
	payloadBytes, err := json.Marshal(approval.ProjectSetOnchainParamsPayload{
		ProjectID: projectID, Params: req,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_ONCHAIN_PARAMS_FAILED", "message": err.Error()})
		return
	}
	initID, initName := currentInitiator(c)
	row, err := approval.Submit(approval.SubmitReq{
		Operation:  approval.OpProjectSetOnchainParams,
		TargetKind: "project", TargetID: strconv.FormatInt(projectID, 10),
		Payload: string(payloadBytes), Reason: c.Query("reason"),
		InitiatorID: initID, InitiatorName: initName,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_ONCHAIN_PARAMS_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": row.Status}})
}
