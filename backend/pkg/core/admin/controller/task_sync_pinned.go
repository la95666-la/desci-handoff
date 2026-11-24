package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/third/twitter"
)

type syncPinnedTweetReq struct {
	Username string `json:"username" binding:"required"`
}

type syncPinnedVersion struct {
	TaskID     uint64 `json:"task_id"`
	TaskKey    string `json:"task_key"`
	OldVersion uint32 `json:"old_version"`
	NewVersion uint32 `json:"new_version"`
}

type syncPinnedTweetResp struct {
	Username      string              `json:"username"`
	PinnedTweetID string              `json:"pinned_tweet_id"`
	URL           string              `json:"url"`
	Updated       int64               `json:"updated"`
	Advanced      int64               `json:"advanced"`
	Versions      []syncPinnedVersion `json:"versions"`
}

// SyncPinnedTweet updates slot 1 of like/retweet through the same round-aware
// mutation used by normal task edits. Slot 2 remains independently managed.
func SyncPinnedTweet(c *gin.Context) {
	var req syncPinnedTweetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": err.Error()})
		return
	}
	username := strings.TrimPrefix(strings.TrimSpace(req.Username), "@")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": "username 必填"})
		return
	}

	bearer := viper.GetString("twitter.bearerToken")
	if bearer == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": false, "code": "BEARER_TOKEN_MISSING",
			"message": "twitter.bearerToken 未配置，请在 config/secret/config.yaml 中补充 App-only Bearer Token",
		})
		return
	}

	pinnedID, err := twitter.GetPinnedTweetID(bearer, username)
	if err != nil {
		log.GetLogger().Warnf("[SyncPinnedTweet] fetch failed for @%s: %v", username, err)
		c.JSON(http.StatusBadGateway, gin.H{"status": false, "code": "TWITTER_API_ERR", "message": err.Error()})
		return
	}
	if pinnedID == "" {
		c.JSON(http.StatusOK, gin.H{
			"status": true, "code": "NO_PINNED_TWEET",
			"message": fmt.Sprintf("@%s 当前没有置顶帖子", username),
			"data":    syncPinnedTweetResp{Username: username, Versions: []syncPinnedVersion{}},
		})
		return
	}

	newURL := fmt.Sprintf("https://x.com/%s/status/%s", username, pinnedID)
	result := syncPinnedTweetResp{
		Username:      username,
		PinnedTweetID: pinnedID,
		URL:           newURL,
		Versions:      make([]syncPinnedVersion, 0, 2),
	}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		var tasks []entity.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("platform = ? AND task_key IN ? AND target_slot = ?", "twitter", []string{"like", "retweet"}, 1).
			Order("id ASC").Find(&tasks).Error; err != nil {
			return err
		}
		for _, task := range tasks {
			effective := task
			effective.URL = newURL
			updates := map[string]interface{}{"url": newURL}
			if err := rejectDuplicateTwitterTarget(tx, effective, task.ID); err != nil {
				return err
			}
			change, err := applyTaskRoundMutation(tx, task, effective, updates)
			if err != nil {
				return err
			}
			res := tx.Model(&entity.Task{}).Where("id = ?", task.ID).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			result.Updated += res.RowsAffected
			if change.Changed {
				result.Advanced++
				result.Versions = append(result.Versions, syncPinnedVersion{
					TaskID: task.ID, TaskKey: task.TaskKey,
					OldVersion: change.OldVersion, NewVersion: change.NewVersion,
				})
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errTaskTargetReused) {
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "ADMIN_TASK_TARGET_REUSED", "message": "该置顶帖子已在历史轮次中使用，不能再次用于发放奖励"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "DB_UPDATE_FAILED", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": result})
}
