package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

type inviteSettingsPayload struct {
	DailyCap  int   `json:"dailyCap"`
	UpdatedAt int64 `json:"updatedAt"`
}

// ListInviteSettings GET /admin/v1/invite-settings
func ListInviteSettings(c *gin.Context) {
	cap, err := readInviteDailyCap(db.Mysql())
	if err != nil {
		respondInviteSettingsErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": inviteSettingsPayload{DailyCap: cap, UpdatedAt: time.Now().Unix()}})
}

// UpdateInviteSettings PUT /admin/v1/invite-settings
func UpdateInviteSettings(c *gin.Context) {
	var req inviteSettingsPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "INVITE_SETTINGS_BAD_REQUEST", "message": err.Error()})
		return
	}
	if req.DailyCap < 1 || req.DailyCap > constant.InviteDailyCapMax {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "INVITE_SETTINGS_BAD_REQUEST", "message": "dailyCap out of range [1,100000]"})
		return
	}

	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		var current entity.Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`key` = ?", constant.InviteDailyCapSettingKey).Take(&current).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return upsertSettingTx(tx, constant.InviteDailyCapSettingKey, strconv.Itoa(req.DailyCap), tgTypeInt)
	})
	if err != nil {
		respondInviteSettingsErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": inviteSettingsPayload{DailyCap: req.DailyCap, UpdatedAt: time.Now().Unix()}})
}

func readInviteDailyCap(database *gorm.DB) (int, error) {
	var setting entity.Settings
	err := database.Select("value").Where("`key` = ?", constant.InviteDailyCapSettingKey).Take(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return constant.InviteDailyCapDefault, nil
	}
	if err != nil {
		return 0, err
	}
	cap, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil || cap < 1 || cap > constant.InviteDailyCapMax {
		return constant.InviteDailyCapDefault, nil
	}
	return cap, nil
}

func respondInviteSettingsErr(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "INVITE_SETTINGS_WRITE_FAILED", "message": "invite settings operation failed"})
}
