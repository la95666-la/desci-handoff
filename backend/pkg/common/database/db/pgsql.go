package db

import (
	"fmt"
	"net/url"
	"runtime"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

var PgClient *gorm.DB

func NewPgsqlClient(addr, resolveAddr, user, password, database string, logWriter *rotatelogs.RotateLogs, logLevel string) (*gorm.DB, error) {
	encodedPassword := url.QueryEscape(password)
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, encodedPassword, addr, database)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: GetLogger(logWriter, logLevel),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 关闭复数形式，使用单数表名
		},
	})
	if err != nil {

		return nil, err
	}
	if resolveAddr != "" {
		replicaDns := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, encodedPassword, resolveAddr, database)
		db.Use(dbresolver.Register(dbresolver.Config{
			Sources:  []gorm.Dialector{postgres.Open(dsn)},
			Replicas: []gorm.Dialector{postgres.Open(replicaDns)},
			Policy:   dbresolver.RandomPolicy{},
		}).SetConnMaxIdleTime(10).SetConnMaxLifetime(5 * time.Minute).SetMaxOpenConns(4 * runtime.GOMAXPROCS(0)))
		PgClient = db
	}
	return db, nil
}
