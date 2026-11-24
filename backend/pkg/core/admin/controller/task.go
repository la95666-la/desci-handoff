package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
	settingsRepo "joincare_server/pkg/core/repository/settings"
)

type taskUpsertReq struct {
	Platform    string `json:"platform" binding:"required"`
	TaskKey     string `json:"task_key" binding:"required"`
	TargetSlot  *uint8 `json:"target_slot"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	CTALabel    string `json:"cta_label"`
	VerifyKind  string `json:"verify_kind"`
	VerifyMeta  string `json:"verify_meta"`
	AmountPoint int    `json:"amount_point"`
	Sort        int    `json:"sort"`
	IsEnable    *int8  `json:"is_enable"`
}

// ListTasks GET /admin/v1/tasks?platform=twitter&keyword=&is_enable=1
func ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}

	q := db.Mysql().Model(&entity.Task{})
	if pl := c.Query("platform"); pl != "" {
		q = q.Where("platform = ?", pl)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("task_key LIKE ? OR title LIKE ?", like, like)
	}
	if en := c.Query("is_enable"); en != "" {
		if v, err := strconv.Atoi(en); err == nil {
			q = q.Where("is_enable = ?", v)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TASK_LIST_FAILED", "message": err.Error()})
		return
	}

	var rows []entity.Task
	if err := q.Order("platform ASC, sort ASC, id ASC").
		Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TASK_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": pageSize,
	}})
}

// CreateTask POST /admin/v1/tasks
func CreateTask(c *gin.Context) {
	var req taskUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_BAD_REQUEST", "message": err.Error()})
		return
	}
	now := time.Now()
	isEnable := int8(1)
	if req.IsEnable != nil {
		isEnable = *req.IsEnable
	}
	row := entity.Task{
		Platform:       req.Platform,
		TaskKey:        req.TaskKey,
		TargetSlot:     1,
		Title:          req.Title,
		Description:    req.Description,
		URL:            req.URL,
		CTALabel:       req.CTALabel,
		VerifyKind:     ifEmpty(req.VerifyKind, "manual"),
		VerifyMeta:     req.VerifyMeta,
		AmountPoint:    req.AmountPoint,
		Sort:           req.Sort,
		IsEnable:       isEnable,
		Version:        1,
		RoundStartedAt: now,
	}
	if req.TargetSlot != nil {
		row.TargetSlot = normalizedTaskTargetSlot(*req.TargetSlot)
	}
	if err := validateTaskTargetSlot(row.Platform, row.TaskKey, row.TargetSlot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_INVALID_SLOT", "message": err.Error()})
		return
	}
	targetRef, err := taskTargetRef(row.Platform, row.TaskKey, row.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_INVALID_TARGET", "message": err.Error()})
		return
	}
	row.TargetRef = targetRef
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := rejectDuplicateTwitterTarget(tx, row, 0); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return ensureTaskRound(tx, row, targetRef)
	})
	if err != nil {
		if errors.Is(err, errTaskTargetReused) {
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "ADMIN_TASK_TARGET_REUSED", "message": "该帖子已用于同一种 X 互动任务，不能重复配置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TASK_CREATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// UpdateTask PUT /admin/v1/tasks/:id
func UpdateTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_BAD_ID", "message": "invalid id"})
		return
	}
	var req taskUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_BAD_REQUEST", "message": err.Error()})
		return
	}

	var before entity.Task
	var roundChange taskRoundChange
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).Take(&before).Error; err != nil {
			return err
		}

		// Empty strings retain the stored value for compatibility with existing
		// partial-update clients. Numeric fields are full replacements.
		effective := before
		effective.Platform = req.Platform
		effective.TaskKey = req.TaskKey
		if req.TargetSlot != nil {
			effective.TargetSlot = normalizedTaskTargetSlot(*req.TargetSlot)
		} else {
			effective.TargetSlot = normalizedTaskTargetSlot(before.TargetSlot)
		}
		if err := validateTaskTargetSlot(effective.Platform, effective.TaskKey, effective.TargetSlot); err != nil {
			return err
		}
		effective.VerifyKind = ifEmpty(req.VerifyKind, "manual")
		effective.AmountPoint = req.AmountPoint
		effective.Sort = req.Sort
		if req.IsEnable != nil {
			effective.IsEnable = *req.IsEnable
		}
		updates := map[string]interface{}{
			"platform":     effective.Platform,
			"task_key":     effective.TaskKey,
			"target_slot":  effective.TargetSlot,
			"verify_kind":  effective.VerifyKind,
			"amount_point": effective.AmountPoint,
			"sort":         effective.Sort,
			"is_enable":    effective.IsEnable,
		}
		if req.Title != "" {
			effective.Title = req.Title
			updates["title"] = req.Title
		}
		if req.Description != "" {
			effective.Description = req.Description
			updates["description"] = req.Description
		}
		if req.URL != "" {
			effective.URL = req.URL
			updates["url"] = req.URL
		}
		if req.CTALabel != "" {
			effective.CTALabel = req.CTALabel
			updates["cta_label"] = req.CTALabel
		}
		if req.VerifyMeta != "" {
			effective.VerifyMeta = req.VerifyMeta
			updates["verify_meta"] = req.VerifyMeta
		}
		if err := rejectDuplicateTwitterTarget(tx, effective, id); err != nil {
			return err
		}

		var roundErr error
		roundChange, roundErr = applyTaskRoundMutation(tx, before, effective, updates)
		if roundErr != nil {
			return roundErr
		}
		return tx.Model(&entity.Task{}).Where("id = ?", id).Updates(updates).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_TASK_NOT_FOUND", "message": "not found"})
		case errors.Is(err, errInvalidTaskTarget):
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_INVALID_TARGET", "message": err.Error()})
		case errors.Is(err, errTaskTargetReused):
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "ADMIN_TASK_TARGET_REUSED", "message": "该帖子已在此任务的历史轮次中使用，不能再次用于发放奖励"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TASK_UPDATE_FAILED", "message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"id":            id,
		"round_changed": roundChange.Changed,
		"old_version":   roundChange.OldVersion,
		"new_version":   roundChange.NewVersion,
		"target_ref":    roundChange.TargetRef,
	}})
}

// DeleteTask DELETE /admin/v1/tasks/:id
func DeleteTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TASK_BAD_ID", "message": "invalid id"})
		return
	}
	if err := db.Mysql().Where("id = ?", id).Delete(&entity.Task{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TASK_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// PublicTaskList GET /client/taskhall/v1/tasks?platform=twitter
func PublicTaskList(c *gin.Context) {
	platform := c.Query("platform")
	q := db.Mysql().Model(&entity.Task{}).Where("is_enable = 1")
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	var rows []entity.Task
	if err := q.Order("platform ASC, sort ASC, id ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": false, "code": "TASK_LIST_FAILED", "message": err.Error()})
		return
	}
	type item struct {
		ID          uint64 `json:"id"`
		Platform    string `json:"platform"`
		TaskKey     string `json:"task_key"`
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
		CTALabel    string `json:"cta_label"`
		AmountPoint int    `json:"amount_point"`
		Sort        int    `json:"sort"`
		Version     uint32 `json:"version"`
		TargetSlot  uint8  `json:"target_slot"`
	}
	items := make([]item, 0, len(rows))
	for _, r := range rows {
		items = append(items, item{
			ID: r.ID, Platform: r.Platform, TaskKey: r.TaskKey,
			Title: r.Title, Description: r.Description, URL: r.URL,
			CTALabel: r.CTALabel, AmountPoint: r.AmountPoint, Sort: r.Sort,
			Version:    normalizedTaskVersion(r.Version),
			TargetSlot: normalizedTaskTargetSlot(r.TargetSlot),
		})
	}
	verificationDisabled, err := settingsRepo.TwitterVerificationDisabled(db.Mysql())
	if err != nil {
		// Do not publish a task list with a policy that could disagree with the
		// authoritative completion path. Clients fail closed when this read fails.
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "TWITTER_VERIFICATION_POLICY_UNAVAILABLE", "message": "twitter verification policy unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": items, "verificationDisabled": verificationDisabled,
	}})
}

func ifEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func int8If(v, def int8) int8 {
	if v == 0 {
		return def
	}
	return v
}
