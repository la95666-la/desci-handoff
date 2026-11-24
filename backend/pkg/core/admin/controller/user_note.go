package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// ListUserNotes GET /admin/v1/users/:id/notes
func ListUserNotes(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	var rows []entity.UserNote
	if err := db.Mysql().Where("uid = ?", id).Order("id DESC").Limit(200).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_NOTE_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": rows}})
}

type userNoteReq struct {
	Note string `json:"note" binding:"required,max=1000"`
}

// CreateUserNote POST /admin/v1/users/:id/notes
func CreateUserNote(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	var req userNoteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_NOTE_BAD_REQUEST", "message": err.Error()})
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if req.Note == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_NOTE_EMPTY", "message": "note empty"})
		return
	}

	// 目标 user 必须存在
	var cnt int64
	db.Mysql().Model(&entity.User{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_USR_NOT_FOUND", "message": "user not found"})
		return
	}

	adminIDAny, _ := c.Get(middleware.CtxAdminID)
	usernameAny, _ := c.Get(middleware.CtxUsername)
	adminID, _ := adminIDAny.(int64)
	username, _ := usernameAny.(string)

	row := entity.UserNote{
		UID: id, AdminID: uint64(adminID), AdminUsername: username, Note: req.Note,
	}
	if err := db.Mysql().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_NOTE_CREATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// DeleteUserNote DELETE /admin/v1/users/notes/:noteId
func DeleteUserNote(c *gin.Context) {
	nid, err := strconv.ParseUint(c.Param("noteId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_NOTE_BAD_ID", "message": "invalid noteId"})
		return
	}
	if err := db.Mysql().Where("id = ?", nid).Delete(&entity.UserNote{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_NOTE_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": nid}})
}

// ListUserLoginLogs GET /admin/v1/users/:id/login-logs?page=1&page_size=20
func ListUserLoginLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	var total int64
	db.Mysql().Model(&entity.UserLoginLog{}).Where("uid = ?", id).Count(&total)
	var rows []entity.UserLoginLog
	if err := db.Mysql().Where("uid = ?", id).Order("id DESC").
		Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_LOGIN_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}
