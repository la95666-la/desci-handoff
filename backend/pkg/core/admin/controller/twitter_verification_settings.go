package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
	settingsRepo "joincare_server/pkg/core/repository/settings"
)

type twitterVerificationSettingsPayload struct {
	VerificationDisabled bool  `json:"verificationDisabled"`
	UpdatedAt            int64 `json:"updatedAt"`
}

type updateTwitterVerificationSettingsRequest struct {
	VerificationDisabled *bool `json:"verificationDisabled" binding:"required"`
}

// ListTwitterVerificationSettings GET /admin/v1/twitter-verification-settings
func ListTwitterVerificationSettings(c *gin.Context) {
	disabled, err := settingsRepo.TwitterVerificationDisabled(db.Mysql())
	if err != nil {
		respondTwitterVerificationSettingsErr(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": twitterVerificationSettingsPayload{
		VerificationDisabled: disabled,
		UpdatedAt:            time.Now().Unix(),
	}})
}

// UpdateTwitterVerificationSettings PUT /admin/v1/twitter-verification-settings
func UpdateTwitterVerificationSettings(c *gin.Context) {
	var req updateTwitterVerificationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TWITTER_VERIFICATION_SETTINGS_BAD_REQUEST", "message": err.Error()})
		return
	}

	value := "0"
	if *req.VerificationDisabled {
		value = "1"
	}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		var current entity.Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("`key` = ?", constant.TwitterVerificationDisabledSettingKey).
			Take(&current).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return upsertSettingTx(tx, constant.TwitterVerificationDisabledSettingKey, value, tgTypeInt)
	})
	if err != nil {
		respondTwitterVerificationSettingsErr(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": twitterVerificationSettingsPayload{
		VerificationDisabled: *req.VerificationDisabled,
		UpdatedAt:            time.Now().Unix(),
	}})
}

func respondTwitterVerificationSettingsErr(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"status":  false,
		"code":    "TWITTER_VERIFICATION_SETTINGS_WRITE_FAILED",
		"message": "twitter verification settings operation failed",
	})
}
