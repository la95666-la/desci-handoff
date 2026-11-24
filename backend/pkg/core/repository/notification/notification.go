package notification

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/settings"
)

func Read(uid int) {
	info := &entity.NotificationReadTime{}
	now := time.Now()
	db.Mysql().Select("id").Model(&entity.NotificationReadTime{}).Where("uid = ?", uid).Scan(info)
	if info.ID == 0 {
		insert := entity.NotificationReadTime{
			UID:        uint64(uid),
			LastReadAt: now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		db.Mysql().Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "uid"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"last_read_at": now,
			}),
		}).Create(&insert)
	} else {
		db.Mysql().Model(&entity.NotificationReadTime{}).
			Where("uid = ?", uid).
			Updates(map[string]interface{}{
				"last_read_at": now,
			})
	}
}

func List(uid, cursor int, lang string) ([]Data, error) {
	lang = string(i18n.FindLang(lang))
	readInfo := entity.NotificationReadTime{}
	err := db.Mysql().Select("last_read_at").Model(&readInfo).Where("uid = ?", uid).Scan(&readInfo).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	infos := make([]entity.Notification, 0, 20)
	sql := db.Mysql().Select("id,uid,type,title,content,related_id,created_at,extra").
		Model(&entity.Notification{}).Where("uid = ?", uid)
	if cursor > 0 {
		sql = sql.Where("id < ?", cursor)
	}
	err = sql.Order("id desc").Limit(20).Find(&infos).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	cwMap := make(map[string]string, len(infos)*2)
	res := make([]Data, 0, len(infos))
	for _, info := range infos {
		title, ok := cwMap[info.Title]
		if !ok {
			cwMap[info.Title] = settings.CopywritingFromDB(info.Title, lang)
			title = cwMap[info.Title]
		}
		content, ok := cwMap[info.Content]
		if !ok {
			cwMap[info.Content] = settings.CopywritingFromDB(info.Content, lang)
			content = cwMap[info.Content]
		}
		relatedId := 0
		if info.RelatedID != nil {
			relatedId = *info.RelatedID
		}
		isRead := false
		if readInfo.LastReadAt.UnixMilli() > info.CreatedAt.UnixMilli() {
			isRead = true
		}
		extraMap := map[string][]any{}
		args := []any{}
		if len(info.Extra) > 0 {
			json.Unmarshal([]byte(info.Extra), &extraMap)
			args = extraMap[lang]
			if len(args) == 0 && lang == constant.NoticeLangTw {
				args = extraMap[constant.NoticeLangCn]
			}
		}
		res = append(res, Data{
			ID:        int(info.ID),
			Type:      info.Type,
			Title:     title,
			Content:   fmt.Sprintf(content, args...),
			RelatedId: relatedId,
			IsRead:    isRead,
			CreatedAt: info.CreatedAt.Unix(),
		})
	}
	return res, nil
}
