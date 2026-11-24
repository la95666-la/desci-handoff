package db

import (
	"gorm.io/gorm"

	"joincare_server/pkg/common/database/redis"
)

type Connection struct {
	Mysql *gorm.DB
	Redis *redis.Connection
}

var Client = &Connection{}

func Close() {
	if Client.Mysql != nil {
		sqlDb, err := Client.Mysql.DB()
		if err == nil {
			sqlDb.Close()
		}
	}
	if Client.Redis != nil {
		Client.Redis.Close()
	}
}

func Mysql() *gorm.DB {
	return Client.Mysql
}

func Redis() *redis.Connection {
	return Client.Redis
}
