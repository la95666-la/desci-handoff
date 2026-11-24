// Package loginlog 用户登录日志（异步写库，不阻塞登录响应）
package loginlog

import (
	"time"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
)

// LoginType 常量
const (
	TypeWallet   = "wallet"
	TypeAuto     = "auto"
	TypeRegister = "register"
	TypeTwitter  = "twitter"
)

// Record 追加一条登录日志；失败仅 warn，不打断登录流程
// 用 goroutine 异步写，避免慢查询影响 200ms P95
func Record(uid int64, ip, userAgent, loginType string) {
	if uid == 0 {
		return
	}
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}
	go func() {
		row := entity.UserLoginLog{
			UID: uid, IP: ip, UserAgent: userAgent,
			LoginType: loginType, LoggedInAt: time.Now(),
		}
		if err := db.Mysql().Create(&row).Error; err != nil {
			log.GetLogger().Warnf("login log write failed uid=%d err=%v", uid, err)
		}
	}()
}
