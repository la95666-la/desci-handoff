// Package notification 是 core 进程内的轻量站内信入口。
//
// 历史上 notice 的 Create/BatchCreate 在 pkg/job/notice 里（用 pkg/job/db），
// 那个包给 job worker 处理链上事件用，core 不能直接 import（db 未初始化）。
// 这里只复用 entity.Notification 表，直插 pkg/core/db 即可。
//
// extraByLang 的两层 map[lang][2]{title, content} 结构沿用 job/notice 约定：
//
//	{ "en-us": ["Check-in Done", "You earned 10 pts"], "zh-tw": ["簽到成功", "您獲得 10 點"] }
//
// 前端 Notifications.vue 取当前语言对应那一项展示。
package notification

import (
	"encoding/json/v2"
	"time"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
)

// Push 给单个 uid 写一条站内信（type + relatedID + extraByLang）。
//   - title/content 仍按约定回填 "<type>Title" / "<type>Content" 作 i18n key
//   - 写入失败仅记 warn，不阻塞调用流程
func Push(uid int, relatedID int, notificationType string, extraByLang map[string][]string) {
	extra, _ := json.Marshal(extraByLang)
	row := entity.Notification{
		UID:       uint64(uid),
		Type:      notificationType,
		Title:     notificationType + "Title",
		Content:   notificationType + "Content",
		Extra:     string(extra),
		RelatedID: &relatedID,
		CreatedAt: time.Now(),
	}
	if err := db.Mysql().Create(&row).Error; err != nil {
		log.GetLogger().Warnf("notification push failed uid=%d type=%s err=%v", uid, notificationType, err)
	}
}

// Broadcast 给所有有钱包地址的 user 批量写一条同类型站内信（用于新闻发布等运营广播）。
//   - relatedID 通常是 news.id
//   - 全表分页扫，每批 200 条 batch INSERT
//   - 失败容忍：每批独立 try，单批失败 warn 后继续
func Broadcast(relatedID int, notificationType string, extraByLang map[string][]string) {
	extra, _ := json.Marshal(extraByLang)
	extraStr := string(extra)

	const pageSize = 500
	offset := 0
	now := time.Now()

	for {
		var uids []int64
		if err := db.Mysql().
			Model(&entity.User{}).
			Where("wallet_address <> ''").
			Order("id ASC").
			Offset(offset).Limit(pageSize).
			Pluck("id", &uids).Error; err != nil {
			log.GetLogger().Warnf("notification broadcast scan failed offset=%d err=%v", offset, err)
			return
		}
		if len(uids) == 0 {
			return
		}

		rows := make([]entity.Notification, 0, len(uids))
		for _, uid := range uids {
			rid := relatedID
			rows = append(rows, entity.Notification{
				UID:       uint64(uid),
				Type:      notificationType,
				Title:     notificationType + "Title",
				Content:   notificationType + "Content",
				Extra:     extraStr,
				RelatedID: &rid,
				CreatedAt: now,
			})
		}
		// CreateInBatches: 内部按指定大小切批，性价比比单条 N 次好
		if err := db.Mysql().CreateInBatches(rows, 200).Error; err != nil {
			log.GetLogger().Warnf("notification broadcast insert failed batch_size=%d err=%v", len(rows), err)
		}
		if len(uids) < pageSize {
			return
		}
		offset += pageSize
	}
}
