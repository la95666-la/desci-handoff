package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/database/redis"
	"joincare_server/pkg/common/http/server"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/notice/db"
	"joincare_server/pkg/notice/route"
	"joincare_server/pkg/notice/websocket"
)

func main() {
	location, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = location
	stopCh := make(chan struct{})
	var path string
	flag.StringVar(&path, "path", "./config/", "--path prod")
	// 读取文件中的配置
	err := config.InitConfig(filepath.Join(path, "notice"), "config.yaml")
	if err != nil {
		panic(err)
	}
	err = config.InitConfig(filepath.Join(path, "common"), "config.yaml")
	if err != nil {
		panic(err)
	}
	dir := viper.GetString("logger.dir")
	if dir == "" {
		dir = config.RunDir
	}
	i18n.I18nInit(filepath.Join(path, "common", "i18n"))

	log.Init(dir, viper.GetString("logger.level"), viper.GetInt("logger.maxDay"))
	err = InitAllConnection(stopCh)
	if err != nil {
		panic(err)
	}
	httpRouter := server.NewEngine()
	server.RegisterHealth(httpRouter)
	g := httpRouter.Group("/notice")
	route.Register(g)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", "", viper.GetInt("protocol.http.port")),
		Handler:      httpRouter,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  300 * time.Second,
	}
	httpServer.SetKeepAlivesEnabled(true) // true recommended
	go httpServer.ListenAndServe()
	wsServer := websocket.Start(stopCh)

	elegantExit(&[]*http.Server{httpServer, wsServer}, stopCh)
}

func InitAllConnection(stopCh chan struct{}) error {
	var err error
	// 数据库连接类初始化
	db.Client = &db.Connection{}
	// redis
	db.Client.Redis, err = redis.Connect(viper.GetStringSlice("db.redis.host"), viper.GetString("db.redis.password"))
	if err != nil {
		return err
	}
	return nil
}

func elegantExit(servers *[]*http.Server, stopCh chan struct{}) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	sig := <-quit

	fmt.Println("Shutdown Server...", sig)
	now, timeout := time.Now(), 10*time.Microsecond
	for _, s := range *servers {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			fmt.Println("Server Shutdown err:", err)
		}
	}

	// 下级监听后 直接关闭各种通信
	close(stopCh)
	// 等待后关闭
	time.Sleep(timeout)
	// 关闭所有client
	db.Close()
	fmt.Println("Exited in", time.Since(now))
}
