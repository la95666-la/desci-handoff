package repository

import (
	"time"

	"gorm.io/gorm"

	"joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/db"
)

func GetByUsername(username string) (*entity.AdminUser, error) {
	var u entity.AdminUser
	err := db.Mysql().Where("username = ?", username).First(&u).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func GetByID(id int64) (*entity.AdminUser, error) {
	var u entity.AdminUser
	err := db.Mysql().Where("id = ?", id).First(&u).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func UpdateLoginInfo(id int64, ip string) error {
	now := time.Now()
	return db.Mysql().Model(&entity.AdminUser{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_login_at": now,
			"last_login_ip": ip,
		}).Error
}
