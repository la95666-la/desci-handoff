package notification

import (
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/notification"
)

func List(uid, cursor int, lang string) (*ListResponse, error) {
	res, err := notification.List(uid, cursor, lang)
	if err != nil {
		return nil, err
	}
	resp := &ListResponse{
		List:   res,
		Cursor: -1,
	}
	if len(res) > 0 {
		resp.Cursor = res[len(res)-1].ID
		go notification.Read(uid)
	}
	return resp, nil
}

func RedPoint(uid int) RedPointResponse {
	info := entity.Notification{}
	err := db.Mysql().Select("created_at").
		Model(&entity.Notification{}).Where("uid = ?", uid).Order("id desc").First(&info).Error
	if err != nil {
		return RedPointResponse{Light: false}
	}

	readInfo := entity.NotificationReadTime{}
	err = db.Mysql().Select("last_read_at").Model(&readInfo).Where("uid = ?", uid).Scan(&readInfo).Error
	if err != nil {
		return RedPointResponse{Light: true}
	}
	if readInfo.LastReadAt.UnixMilli() > info.CreatedAt.UnixMilli() {
		return RedPointResponse{Light: false}
	}
	return RedPointResponse{Light: true}
}
