package storage

import (
	"fmt"

	"github.com/spf13/viper"
)

// Default 全局默认 uploader；main.go 启动时 Init() 一次
var Default Uploader

// Init 读 viper 配置初始化默认 uploader。
// 配置：
//
//	storage:
//	  driver: local           # local | minio (Phase 2)
//	  local:
//	    root_dir: /app/uploads
//	    public_base: /uploads # nginx 静态 serve 该目录
func Init() error {
	driver := viper.GetString("storage.driver")
	if driver == "" {
		driver = "local"
	}
	switch driver {
	case "local":
		root := viper.GetString("storage.local.root_dir")
		if root == "" {
			root = "/app/uploads"
		}
		pub := viper.GetString("storage.local.public_base")
		if pub == "" {
			pub = "/uploads"
		}
		Default = &LocalFs{RootDir: root, PublicBase: pub}
		return nil
	default:
		return fmt.Errorf("unsupported storage.driver: %s", driver)
	}
}
