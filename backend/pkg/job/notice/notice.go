package notice

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/client"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/model"
	"joincare_server/pkg/job/db"
)

func Create(uid, relatedID int, notificationType string, extraByLang map[string][]string) error {
	if err := CreateWithDB(db.Mysql(), uid, relatedID, notificationType, extraByLang); err != nil {
		return err
	}
	return Push(uid, relatedID, notificationType)
}

// CreateWithDB 只持久化通知，允许业务把通知记录与自身状态更新放进同一事务。
func CreateWithDB(tx *gorm.DB, uid, relatedID int, notificationType string, extraByLang map[string][]string) error {
	title, content := GetNotificationTypeCopywriting(notificationType)
	extra, _ := json.Marshal(extraByLang)
	insert := entity.Notification{
		UID:       uint64(uid),
		Type:      notificationType,
		Title:     title,
		Content:   content,
		Extra:     string(extra),
		RelatedID: &relatedID,
		CreatedAt: time.Now(),
	}
	return tx.Create(&insert).Error
}

// Push 在通知已持久化后尝试实时推送；失败不应回滚已提交的业务事务。
func Push(uid, relatedID int, notificationType string) error {
	return send(uid, constant.ReadPoint, ReadPointSt{
		Type:      notificationType,
		RelatedID: relatedID,
	})
}

func BatchCreate(uids, relatedIDs []int, notificationType string, extraByLang map[string][]string) error {
	title, content := GetNotificationTypeCopywriting(notificationType)
	extra, _ := json.Marshal(extraByLang)
	inserts := make([]entity.Notification, 0, 100)
	now := time.Now()
	for i, uid := range uids {
		inserts = append(inserts, entity.Notification{
			UID:       uint64(uid),
			Type:      notificationType,
			Title:     title,
			Content:   content,
			Extra:     string(extra),
			RelatedID: &relatedIDs[i],
			CreatedAt: now,
		})
		if len(inserts) >= 100 {
			err := db.Mysql().Create(&inserts).Error
			if err != nil {
				return err
			}
			inserts = make([]entity.Notification, 0, 100)
		}
	}
	if len(inserts) > 0 {
		err := db.Mysql().Create(&inserts).Error
		if err != nil {
			return err
		}
	}
	err := batchSend(uids, constant.ReadPoint, ReadPointSt{
		Type: notificationType,
	})
	if err != nil {
		return err
	}
	return nil
}

func GetNotificationTypeCopywriting(notificationType string) (string, string) {
	return notificationType + "Title", notificationType + "Content"
}

func send(uid int, event string, data any) error {
	sendData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	request := model.NoticeRequest{
		Uid:   uid,
		Event: event,
		Data:  sendData,
	}
	userInfo := model.PersonInfo{}
	err = db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_PERSON_INFO, uid), &userInfo)
	if err != nil {
		return nil
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	c := client.GetHTTPClient()
	resp, err := c.Post(userInfo.OwnerHost+"/notice/v1/send", body)
	if err != nil {
		return err
	}
	r := &ret.Result{}
	err = json.Unmarshal(resp.Body(), r)
	if err != nil {
		return err
	}
	if !r.Status {
		if r.Code != "" {
			return ret.CusError(r.Code)
		} else {
			return errors.New(r.Message)
		}
	}
	return nil
}

func batchSend(uids []int, event string, data any) error {
	sendData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(uids))
	for _, uid := range uids {
		keys = append(keys, fmt.Sprintf(constant.KEY_PERSON_INFO, uid))
	}
	sendMap := make(map[string][]int, 8)
	res := db.Redis().MGet(context.Background(), keys...)
	for _, value := range res {
		userInfo := model.PersonInfo{}
		err = json.Unmarshal([]byte(value), &userInfo)
		if err != nil {
			continue
		}
		sendMap[userInfo.OwnerHost] = append(sendMap[userInfo.OwnerHost], userInfo.Uid)
	}
	var retErr error
	for host, sendUids := range sendMap {
		request := model.BatchNoticeRequest{
			Uids:  sendUids,
			Event: event,
			Data:  sendData,
		}
		body, err := json.Marshal(request)
		if err != nil {
			retErr = err
			continue
		}
		c := client.GetHTTPClient()
		resp, err := c.Post("http://"+host+"/notice/v1/batchSend", body)
		if err != nil {
			retErr = err
			continue
		}
		r := &ret.Result{}
		err = json.Unmarshal(resp.Body(), r)
		if err != nil {
			retErr = err
			continue
		}
		if !r.Status {
			if r.Code != "" {
				retErr = ret.CusError(r.Code)
				continue
			} else {
				retErr = errors.New(r.Message)
				continue
			}
		}
	}
	return retErr
}
