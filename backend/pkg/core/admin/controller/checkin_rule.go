package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// ListCheckInRules GET /admin/v1/checkin-rules
func ListCheckInRules(c *gin.Context) {
	var rows []entity.CheckInRule
	if err := db.Mysql().Order("day_threshold ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CR_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": rows}})
}

type checkInRuleReq struct {
	// day_threshold 必须 > 0（连续第 N 天，N ≥ 1）
	DayThreshold int `json:"day_threshold" binding:"required,gt=0,lte=3650"`
	// bonus_point 允许 0（管理员可显式配置某档不发分），但拒绝负数 / 单档发超过 100 万分
	BonusPoint int  `json:"bonus_point" binding:"gte=0,lte=1000000"`
	IsEnable   int8 `json:"is_enable"`
	Sort       int  `json:"sort"`
}

// CreateCheckInRule POST /admin/v1/checkin-rules
func CreateCheckInRule(c *gin.Context) {
	var req checkInRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CR_BAD_REQUEST", "message": err.Error()})
		return
	}
	row := entity.CheckInRule{
		DayThreshold: req.DayThreshold,
		BonusPoint:   req.BonusPoint,
		IsEnable:     int8If(req.IsEnable, 1),
		Sort:         req.Sort,
	}
	if err := db.Mysql().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CR_CREATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// UpdateCheckInRule PUT /admin/v1/checkin-rules/:id
func UpdateCheckInRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CR_BAD_ID", "message": "invalid id"})
		return
	}
	var req checkInRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CR_BAD_REQUEST", "message": err.Error()})
		return
	}
	res := db.Mysql().Model(&entity.CheckInRule{}).Where("id = ?", id).Updates(map[string]interface{}{
		"day_threshold": req.DayThreshold,
		"bonus_point":   req.BonusPoint,
		"is_enable":     int8If(req.IsEnable, 1),
		"sort":          req.Sort,
	})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CR_UPDATE_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_CR_NOT_FOUND", "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// DeleteCheckInRule DELETE /admin/v1/checkin-rules/:id
func DeleteCheckInRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CR_BAD_ID", "message": "invalid id"})
		return
	}
	if err := db.Mysql().Where("id = ?", id).Delete(&entity.CheckInRule{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CR_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

