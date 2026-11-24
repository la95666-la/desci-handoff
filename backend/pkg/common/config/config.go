package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var (
	RunDir string
)

func InitConfig(path string, file string) error {
	viper.SetConfigType("yaml")
	RunDir, _ = os.Getwd()
	if !filepath.IsAbs(path) {
		path = filepath.Join(RunDir, path)
	}
	viper.SetConfigFile(filepath.Join(path, file))
	err := viper.MergeInConfig()
	if err != nil {
		return err
	}
	return nil
}

func GetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		value = viper.GetString("application.env." + key)
	}
	return value
}

func Test() bool {
	return viper.GetString("application.environment") == "test"
}

func Prod() bool {
	return viper.GetString("application.environment") == "prod"
}
