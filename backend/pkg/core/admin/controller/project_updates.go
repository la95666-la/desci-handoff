package controller

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/service/notification"
	"joincare_server/pkg/core/service/sanitize"
)

// ListProjectUpdates GET /admin/v1/projects/:id/updates?page=&size=
// 含草稿（Sprint C admin 编辑用；C 端只查已发布走 client 端点）
func ListProjectUpdates(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}

	var total int64
	db.Mysql().Model(&entity.ProjectUpdate{}).Where("project_id = ?", pid).Count(&total)

	var rows []entity.ProjectUpdate
	err = db.Mysql().Model(&entity.ProjectUpdate{}).
		Where("project_id = ?", pid).
		Order("COALESCE(published_at, created_at) DESC, id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_UPDATES_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

type projectUpdateUpsertReq struct {
	Title       string `json:"title" binding:"required,max=200"`
	ContentHTML string `json:"content_html"`
	Publish     bool   `json:"publish"` // true = 立即发布；false = 仅存草稿
}

// CreateProjectUpdate POST /admin/v1/projects/:id/updates
func CreateProjectUpdate(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid id"})
		return
	}
	var req projectUpdateUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_UPDATE_BAD_REQUEST", "message": err.Error()})
		return
	}

	cleanTitle := sanitize.PlainText(req.Title)
	cleanContent := sanitize.RichHTML(req.ContentHTML)
	row := entity.ProjectUpdate{
		ProjectID:     int64(pid),
		AuthorAdminID: currentAdminID(c),
		Title:         cleanTitle,
		ContentHTML:   cleanContent,
	}
	if req.Publish {
		now := time.Now()
		row.IsPublished = 1
		row.PublishedAt = &now
	}
	if err := db.Mysql().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_UPDATE_CREATE_FAILED", "message": err.Error()})
		return
	}

	// Sprint C：发布触发异步推送
	if req.Publish {
		go pushProjectUpdate(row.ProjectID, row.ID, row.Title, row.ContentHTML)
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// UpdateProjectUpdate PUT /admin/v1/project-updates/:updateId
func UpdateProjectUpdate(c *gin.Context) {
	uid, err := strconv.ParseUint(c.Param("updateId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_UPDATE_BAD_ID", "message": "invalid id"})
		return
	}
	var req projectUpdateUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_UPDATE_BAD_REQUEST", "message": err.Error()})
		return
	}

	// 读旧值决定是否触发首次发布
	var before entity.ProjectUpdate
	if err := db.Mysql().Where("id = ?", uid).Take(&before).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "PROJECT_UPDATE_NOT_FOUND", "message": err.Error()})
		return
	}

	cleanTitle := sanitize.PlainText(req.Title)
	cleanContent := sanitize.RichHTML(req.ContentHTML)
	updates := map[string]interface{}{
		"title":        cleanTitle,
		"content_html": cleanContent,
	}
	firstPublish := req.Publish && before.IsPublished == 0
	if firstPublish {
		now := time.Now()
		updates["is_published"] = 1
		updates["published_at"] = now
	}

	if err := db.Mysql().Model(&entity.ProjectUpdate{}).Where("id = ?", uid).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_UPDATE_SAVE_FAILED", "message": err.Error()})
		return
	}
	// Sprint C：首次发布触发异步推送（重复保存已发布态不再推）
	if firstPublish {
		go pushProjectUpdate(before.ProjectID, uid, cleanTitle, cleanContent)
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": uid}})
}

// DeleteProjectUpdate DELETE /admin/v1/project-updates/:updateId
func DeleteProjectUpdate(c *gin.Context) {
	uid, err := strconv.ParseUint(c.Param("updateId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_UPDATE_BAD_ID", "message": "invalid id"})
		return
	}
	if err := db.Mysql().Where("id = ?", uid).Delete(&entity.ProjectUpdate{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PROJECT_UPDATE_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": uid}})
}

func currentAdminID(c *gin.Context) uint64 {
	if v, ok := c.Get(middleware.CtxAdminID); ok {
		if i, ok := v.(int64); ok {
			return uint64(i)
		}
	}
	return 0
}

// pushProjectUpdate 异步广播 project update 到订阅者。
// 分页扫 project_subscribers 表，逐条 notification.Push 写站内信。
// 出错仅 warn，不影响 API 返回。
func pushProjectUpdate(projectID int64, updateID uint64, title, contentHTML string) {
	excerpt := htmlExcerpt(contentHTML, 120)
	extra := map[string][]string{
		constant.NoticeLangEn: {title, excerpt},
		constant.NoticeLangTw: {title, excerpt},
		constant.NoticeLangCn: {title, excerpt},
	}

	const pageSize = 500
	offset := 0
	for {
		var uids []int64
		err := db.Mysql().Model(&entity.ProjectSubscriber{}).
			Where("project_id = ?", projectID).
			Order("id ASC").
			Offset(offset).Limit(pageSize).
			Pluck("uid", &uids).Error
		if err != nil || len(uids) == 0 {
			return
		}
		relID := int(updateID)
		for _, uid := range uids {
			notification.Push(int(uid), relID, constant.ProjectUpdatePublished, extra)
		}
		if len(uids) < pageSize {
			return
		}
		offset += pageSize
	}
}

// htmlExcerpt strips the already-sanitized HTML for notification previews.
var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

func htmlExcerpt(html string, max int) string {
	if html == "" {
		return ""
	}
	text := htmlTagRE.ReplaceAllString(html, "")
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return text
}
