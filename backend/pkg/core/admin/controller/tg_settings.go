package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// TG 配置 kv keys（存 settings 表）
const (
	tgKeyChatID        = "tg.chat_id"
	tgKeyChatLink      = "tg.chat_link"
	tgKeyMessagePoints = "tg.message_points"
	tgKeyDailyCap      = "tg.daily_cap"

	tgTypeInt    = "int"
	tgTypeString = "string"
)

type tgSettingsPayload struct {
	ChatID        int64  `json:"chatId"`
	ChatLink      string `json:"chatLink"`
	MessagePoints int    `json:"messagePoints"`
	DailyCap      int    `json:"dailyCap"`
	UpdatedAt     int64  `json:"updatedAt"` // unix ts
}

// ListTgSettings GET /admin/v1/tg-settings
// 返回当前 4 个 tg.* 值。缺失字段默认 0/""。
func ListTgSettings(c *gin.Context) {
	res := readTgSettings()
	c.JSON(http.StatusOK, gin.H{"status": true, "data": res})
}

// UpdateTgSettings PUT /admin/v1/tg-settings
// 一次性 upsert 4 个字段
func UpdateTgSettings(c *gin.Context) {
	var req tgSettingsPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TG_SETTINGS_BAD_REQUEST", "message": err.Error()})
		return
	}
	// 简单校验：chat_id 可以是负数（groups/supergroups 都是负）；points 必须 >=0；daily_cap >=1
	if req.MessagePoints < 0 || req.MessagePoints > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TG_SETTINGS_BAD_REQUEST", "message": "messagePoints out of range [0,100000]"})
		return
	}
	if req.DailyCap < 1 || req.DailyCap > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TG_SETTINGS_BAD_REQUEST", "message": "dailyCap out of range [1,100000]"})
		return
	}
	if len(req.ChatLink) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TG_SETTINGS_BAD_REQUEST", "message": "chatLink too long"})
		return
	}

	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		var capSetting entity.Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`key` = ?", tgKeyDailyCap).Take(&capSetting).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		for _, item := range []struct{ key, value, typ string }{
			{tgKeyChatID, strconv.FormatInt(req.ChatID, 10), tgTypeInt},
			{tgKeyChatLink, req.ChatLink, tgTypeString},
			{tgKeyMessagePoints, strconv.Itoa(req.MessagePoints), tgTypeInt},
			{tgKeyDailyCap, strconv.Itoa(req.DailyCap), tgTypeInt},
		} {
			if err := upsertSettingTx(tx, item.key, item.value, item.typ); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		respondSettingsErr(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": readTgSettings()})
}

// EffectiveTgSettings GET /admin/v1/tg-settings/effective
// tgbot 端专用；返回精简结构（不用 gin.H），字段名跟 tgbot config yaml 对齐
type effectiveTgSettings struct {
	ChatID        int64  `json:"chatId"`
	ChatLink      string `json:"chatLink"`
	MessagePoints int    `json:"messagePoints"`
	DailyCap      int    `json:"dailyCap"`
}

func GetEffectiveTgSettings(c *gin.Context) {
	res := readTgSettings()
	c.JSON(http.StatusOK, gin.H{"status": true, "data": effectiveTgSettings{
		ChatID:        res.ChatID,
		ChatLink:      res.ChatLink,
		MessagePoints: res.MessagePoints,
		DailyCap:      res.DailyCap,
	}})
}

// ---- helpers ----

func readTgSettings() tgSettingsPayload {
	var rows []entity.Settings
	_ = db.Mysql().Model(&entity.Settings{}).
		Where("`key` IN ?", []string{tgKeyChatID, tgKeyChatLink, tgKeyMessagePoints, tgKeyDailyCap}).
		Scan(&rows).Error

	out := tgSettingsPayload{UpdatedAt: time.Now().Unix()}
	for _, r := range rows {
		switch r.Key {
		case tgKeyChatID:
			out.ChatID, _ = strconv.ParseInt(r.Value, 10, 64)
		case tgKeyChatLink:
			out.ChatLink = r.Value
		case tgKeyMessagePoints:
			out.MessagePoints, _ = strconv.Atoi(r.Value)
		case tgKeyDailyCap:
			out.DailyCap, _ = strconv.Atoi(r.Value)
		}
	}
	return out
}

// settings 表无 primary key，用 (key) 做去重：先 update 若 rows=0 再 insert
func upsertSetting(key, value, typ string) error {
	return upsertSettingTx(db.Mysql(), key, value, typ)
}

func upsertSettingTx(tx *gorm.DB, key, value, typ string) error {
	res := tx.Model(&entity.Settings{}).
		Where("`key` = ?", key).
		Updates(map[string]interface{}{"value": value, "type": typ})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return tx.Create(&entity.Settings{Key: key, Value: value, Type: typ}).Error
	}
	return nil
}

func respondSettingsErr(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"status":  false,
		"code":    "TG_SETTINGS_WRITE_FAILED",
		"message": err.Error(),
	})
}
