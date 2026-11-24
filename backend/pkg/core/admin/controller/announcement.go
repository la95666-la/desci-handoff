package controller

import (
	"encoding/json/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/service/sanitize"
)

// ListAnnouncements GET /admin/v1/announcements?type=&language=&page=&page_size=
func ListAnnouncements(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	q := db.Mysql().Model(&entity.Announcement{})
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	if l := c.Query("language"); l != "" {
		q = q.Where("language = ?", l)
	}
	var total int64
	q.Count(&total)
	var rows []entity.Announcement
	if err := q.Order("priority DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ANN_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

type announcementReq struct {
	Title    string  `json:"title" binding:"required"`
	Content  string  `json:"content"`
	Type     string  `json:"type"`
	Language string  `json:"language"`
	Priority int     `json:"priority"`
	StartAt  *string `json:"start_at"`
	EndAt    *string `json:"end_at"`
	IsEnable int8    `json:"is_enable"`
}

func validAnnouncementType(s string) string {
	switch s {
	case "popup", "banner":
		return s
	default:
		return "popup"
	}
}

// CreateAnnouncement POST /admin/v1/announcements
func CreateAnnouncement(c *gin.Context) {
	var req announcementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_ANN_BAD_REQUEST", "message": err.Error()})
		return
	}
	row := entity.Announcement{
		Title:    req.Title,
		Content:  sanitize.RichHTML(req.Content),
		Type:     validAnnouncementType(req.Type),
		Language: validLanguage(req.Language),
		Priority: req.Priority,
		StartAt:  parseTime(req.StartAt),
		EndAt:    parseTime(req.EndAt),
		IsEnable: int8If(req.IsEnable, 1),
	}
	if err := db.Mysql().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ANN_CREATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// UpdateAnnouncement PUT /admin/v1/announcements/:id
func UpdateAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_ANN_BAD_ID", "message": "invalid id"})
		return
	}
	var req announcementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_ANN_BAD_REQUEST", "message": err.Error()})
		return
	}
	res := db.Mysql().Model(&entity.Announcement{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":     req.Title,
		"content":   sanitize.RichHTML(req.Content),
		"type":      validAnnouncementType(req.Type),
		"language":  validLanguage(req.Language),
		"priority":  req.Priority,
		"start_at":  parseTime(req.StartAt),
		"end_at":    parseTime(req.EndAt),
		"is_enable": int8If(req.IsEnable, 1),
	})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ANN_UPDATE_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_ANN_NOT_FOUND", "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// DeleteAnnouncement DELETE /admin/v1/announcements/:id
func DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_ANN_BAD_ID", "message": "invalid id"})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.AnnouncementDelPayload{ID: id})
	row, err := approval.Submit(approval.SubmitReq{Operation: approval.OpAnnouncementDel, TargetKind: "announcement", TargetID: strconv.FormatUint(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ANN_DELETE_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
}

// PublicAnnouncementCurrent GET /client/announcement/v1/current?language=en
// 同时返当前生效的 popup（最高优先级 1 条）和 banner（最高优先级 1 条）
func PublicAnnouncementCurrent(c *gin.Context) {
	language := validLanguage(c.DefaultQuery("language", "en"))
	now := time.Now()
	pickOne := func(typ string) *entity.Announcement {
		var row entity.Announcement
		err := db.Mysql().
			Where("type = ? AND language = ? AND is_enable = 1", typ, language).
			Where("(start_at IS NULL OR start_at <= ?)", now).
			Where("(end_at IS NULL OR end_at >= ?)", now).
			Order("priority DESC, id DESC").
			Limit(1).Take(&row).Error
		if err != nil {
			return nil
		}
		return &row
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"popup":  pickOne("popup"),
		"banner": pickOne("banner"),
	}})
}
