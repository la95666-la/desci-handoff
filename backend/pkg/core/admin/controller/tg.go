package controller

import (
	"context"
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
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/points"
)

// ===== POST /admin/v1/tg/bind  ============================================

type tgBindReq struct {
	UID               int64  `json:"uid" binding:"required"`
	TelegramID        string `json:"telegram_id" binding:"required"`
	TelegramUsername  string `json:"telegram_username"`
	TelegramFirstName string `json:"telegram_first_name"`
	TelegramLastName  string `json:"telegram_last_name"`
}

func BindTG(c *gin.Context) {
	var req tgBindReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TG_BAD_REQUEST", "message": err.Error()})
		return
	}

	tgID := strings.TrimSpace(req.TelegramID)
	if tgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TG_BAD_REQUEST", "message": "telegram_id required"})
		return
	}
	// 反作弊：一个 Telegram 账号在整个平台只能绑定一个 JoinCare 用户。
	// DB 侧 user.uk_telegram_id 唯一索引已经兜底，这里前置显式判定以返回三语业务错，
	// 避免 tgbot 用户看到 raw MySQL "Error 1062 Duplicate entry ...";
	// 允许同 uid 重复调（幂等 / token 续期场景）。
	var existing entity.User
	dupErr := db.Mysql().Model(&entity.User{}).
		Select("id").
		Where("telegram_id = ? AND id != ?", tgID, req.UID).
		First(&existing).Error
	if dupErr == nil {
		lang := i18n.FindLang(c.GetHeader(constant.HEADER_JOINCARE_LANGUAGE))
		msg := i18n.Localize(lang, i18n.ERR_001_00018, nil)
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": i18n.ERR_001_00018, "message": msg})
		return
	} else if !errors.Is(dupErr, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TG_BIND_FAILED", "message": dupErr.Error()})
		return
	}

	res := db.Mysql().Model(&entity.User{}).Where("id = ?", req.UID).Updates(map[string]interface{}{
		"telegram_id":         tgID,
		"telegram_username":   req.TelegramUsername,
		"telegram_first_name": req.TelegramFirstName,
		"telegram_last_name":  req.TelegramLastName,
	})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TG_BIND_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_USER_NOT_FOUND", "message": "uid not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"uid": req.UID, "telegram_id": tgID}})
}

// ===== GET /admin/v1/tg/resolve?telegram_id=xxx  ==========================

func ResolveTG(c *gin.Context) {
	tgID := strings.TrimSpace(c.Query("telegram_id"))
	if tgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TG_BAD_REQUEST", "message": "telegram_id required"})
		return
	}
	var u entity.User
	err := db.Mysql().Select("id, type").Where("telegram_id = ?", tgID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"uid": 0, "type": ""}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TG_RESOLVE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"uid": u.ID, "type": u.Type}})
}

// ===== POST /admin/v1/tg/task-complete  ===================================

type tgCompleteReq struct {
	UID     int64  `json:"uid" binding:"required"`
	TaskKey string `json:"task_key" binding:"required"`
	ChatID  int64  `json:"chat_id"`
	Source  string `json:"source"`
}

type tgCompleteResp struct {
	Awarded     int    `json:"awarded"`
	DailyLeft   int    `json:"daily_left"`
	AlreadyDone bool   `json:"already_done"`
	Reason      string `json:"reason"`
}

func CompleteTGTask(c *gin.Context) {
	var req tgCompleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TG_BAD_REQUEST", "message": err.Error()})
		return
	}

	// 找 task 配置
	var task entity.Task
	if err := db.Mysql().Where("platform = ? AND task_key = ? AND is_enable = 1", "telegram", req.TaskKey).
		First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_TG_TASK_NOT_FOUND", "message": "telegram task not configured: " + req.TaskKey})
		return
	}

	// chat 是每日累积型；其它（join 等）是一次性
	if req.TaskKey == "chat" {
		writeChatActivity(c, req, task)
		return
	}

	// 一次性任务：task_record 已存在 → 不重复加分
	var existing entity.TaskRecord
	err := db.Mysql().Select("id").
		Where("uid = ? AND task_id = ? AND platform = ?", req.UID, task.ID, "telegram").
		First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": tgCompleteResp{AlreadyDone: true, Reason: "already done"}})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TG_LOOKUP_FAILED", "message": err.Error()})
		return
	}

	now := time.Now()
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		rec := entity.TaskRecord{
			TaskID:       int(task.ID),
			Platform:     "telegram",
			TaskVersion:  normalizedTaskVersion(task.Version),
			UID:          int(req.UID),
			AmountPoint:  task.AmountPoint,
			VerifyStatus: "manual_pass",
			TargetURL:    task.URL,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		if _, err := points.ChangePoints(int(req.UID), task.AmountPoint, constant.PointFlowTypeTask, "TG task: "+req.TaskKey, *tx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TG_COMPLETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": tgCompleteResp{Awarded: task.AmountPoint}})
}

func writeChatActivity(c *gin.Context, req tgCompleteReq, task entity.Task) {
	ctx := context.Background()
	var awarded, dailyLeft int
	var dayKey string
	reserved := false
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		var lockedTask entity.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND platform = 'telegram' AND task_key = 'chat' AND is_enable = 1", task.ID).Take(&lockedTask).Error; err != nil {
			return errors.New("chat reward rule changed")
		}
		var capSetting entity.Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`key` = ?", tgKeyDailyCap).Take(&capSetting).Error; err != nil {
			return errors.New("chat daily cap rule is unavailable")
		}
		dailyMsgs, parseErr := strconv.Atoi(capSetting.Value)
		if parseErr != nil || dailyMsgs < 1 || lockedTask.AmountPoint <= 0 {
			return errors.New("chat reward rule is invalid")
		}
		cap := dailyMsgs * lockedTask.AmountPoint
		dayKey = fmt.Sprintf("jc:tg:chat_cap:%d:%s", req.UID, time.Now().Format("2006-01-02"))
		newTotal, reserveErr := db.Redis().ReserveCappedIncrement(ctx, dayKey, int64(lockedTask.AmountPoint), int64(cap), 25*time.Hour)
		if reserveErr != nil {
			return errors.New("chat daily cap authority unavailable")
		}
		if newTotal < 0 {
			return errTgDailyCapReached
		}
		reserved = true
		awarded = lockedTask.AmountPoint
		if _, err := points.ChangePoints(int(req.UID), lockedTask.AmountPoint, constant.PointFlowTypeTask, "TG chat activity", *tx); err != nil {
			return err
		}
		dailyLeft = cap - int(newTotal)
		return nil
	})
	if err != nil {
		if reserved {
			_ = db.Redis().ReleaseCappedIncrement(ctx, dayKey, int64(awarded))
		}
		if errors.Is(err, errTgDailyCapReached) {
			c.JSON(http.StatusOK, gin.H{"status": true, "data": tgCompleteResp{Reason: "daily cap reached"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_TG_COMPLETE_FAILED", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": tgCompleteResp{
		Awarded: awarded, DailyLeft: dailyLeft,
	}})
}

var errTgDailyCapReached = errors.New("TG daily cap reached")
