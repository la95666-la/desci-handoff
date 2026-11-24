package db

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

var levels = map[string]gLogger.LogLevel{"INFO": gLogger.Info, "ERROR": gLogger.Error}

func NewMysqlClient(host, userName, password, readHost, readUserName, readPassword, databaseName string, logWriter *rotatelogs.RotateLogs, logLevel string) (*gorm.DB, error) {

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s&readTimeout=30s&writeTimeout=30s", userName, password, host, databaseName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: GetLogger(logWriter, logLevel),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 关闭复数形式，使用单数表名
		},
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	maxOpenConns := 4 * runtime.GOMAXPROCS(0)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxOpenConns / 2)
	sqlDB.SetConnMaxIdleTime(20 * time.Second)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if readHost != "" {
		replicaDns := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", readUserName, readPassword, readHost, databaseName)
		db.Use(dbresolver.Register(dbresolver.Config{
			Sources:  []gorm.Dialector{mysql.Open(dsn)},
			Replicas: []gorm.Dialector{mysql.Open(replicaDns)},
			Policy:   dbresolver.RandomPolicy{},
		}).SetConnMaxIdleTime(10).
			SetConnMaxLifetime(5 * time.Minute).
			SetMaxOpenConns(4 * runtime.GOMAXPROCS(0)))
		PgClient = db
	}

	return db, nil
}

func GetLogger(logWriter *rotatelogs.RotateLogs, logLevel string) gLogger.Interface {
	var newLogger gLogger.Interface
	if logWriter != nil {

		newLogger = gLogger.New(
			log.New(logWriter, "\r\n", log.LstdFlags),
			gLogger.Config{
				SlowThreshold: time.Second,      // 慢 SQL 阈值
				LogLevel:      levels[logLevel], // Log level
				Colorful:      false,            // 禁用彩色打印
			},
		)
	} else {

		newLogger = gLogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gLogger.Config{
				SlowThreshold: time.Second,      // 慢 SQL 阈值
				LogLevel:      levels[logLevel], // Log level
				Colorful:      false,            // 禁用彩色打印
			},
		)
	}
	return newLogger
}
