package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// DeleteUser DELETE /admin/v1/users/:id
// 联动清理所有关联表 + Redis 缓存，防止孤儿数据导致邀请奖励错发等 bug
func DeleteUser(c *gin.Context) {
	uid, err := strconv.Atoi(c.Param("id"))
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": "invalid id"})
		return
	}

	// 先检查 user 存在
	var count int64
	if err := db.Mysql().Model(&entity.User{}).Where("id = ?", uid).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "DB_ERR", "message": err.Error()})
		return
	}
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "USER_NOT_FOUND", "message": "uid not exist"})
		return
	}

	// 事务清理关联表
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		// 1) 关联表全清（作为受害者视角：uid 是主键或本人）
		for _, tbl := range []interface{}{
			&entity.UserPointFlow{},
			&entity.UserPoint{},
			&entity.UserCheckInRecord{},
			&entity.TaskRecord{},
			&entity.InviteUserCode{}, // 关键：清邀请码，避免残留污染下次注册
		} {
			if err := tx.Where("uid = ?", uid).Delete(tbl).Error; err != nil {
				return err
			}
		}

		// 2) 作为受邀者：删本条邀请记录（会导致原邀请人 pending 数减 1 —— 合理）
		if err := tx.Where("uid = ?", uid).Delete(&entity.InviteRecord{}).Error; err != nil {
			return err
		}

		// 3) 作为邀请人：不能删受邀者，改 inviter_uid = 0 表示邀请人已删（保留审计）
		if err := tx.Model(&entity.InviteRecord{}).Where("inviter_uid = ?", uid).
			Update("inviter_uid", 0).Error; err != nil {
			return err
		}

		// 4) 主表
		if err := tx.Where("id = ?", uid).Delete(&entity.User{}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "DELETE_FAILED", "message": err.Error()})
		return
	}

	// 5) Redis 缓存（Twitter token）—— 事务外 best-effort
	_ = db.Redis().Del(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, uid))

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"deleted_uid": uid}})
}
