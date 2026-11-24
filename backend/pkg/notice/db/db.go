package db

import (
	"joincare_server/pkg/common/database/redis"
)

type Connection struct {
	Redis *redis.Connection
}

var Client = &Connection{}

func Redis() *redis.Connection {
	return Client.Redis
}

func Close() {
	if Client.Redis != nil {
		Client.Redis.Close()
	}
}
