package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/database/db"
	"joincare_server/pkg/common/database/redis"
	commonSchema "joincare_server/pkg/common/database/schema"
	"joincare_server/pkg/common/http/server"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	jobDb "joincare_server/pkg/job/db"
	"joincare_server/pkg/job/job"
	"joincare_server/pkg/job/job/chainevent"
)

func main() {
	location, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = location
	stopCh := make(chan struct{})
	var path string
	flag.StringVar(&path, "path", "./config/", "--path prod")
	// 读取文件中的配置
	err := config.InitConfig(filepath.Join(path, "job"), "config.yaml")
	if err != nil {
		panic(err)
	}
	err = config.InitConfig(filepath.Join(path, "common"), "config.yaml")
	if err != nil {
		panic(err)
	}
	err = config.InitConfig(filepath.Join(path, "secret"), "config.yaml")
	if err != nil {
		panic(err)
	}
	if err := job.ValidateRuntimeMode(viper.GetString("application.environment"), viper.GetString("application.jobMode"), viper.GetString("chain.project.v1.enabled"),
		viper.GetBool("fairLaunchMainnet.scanner.enabled"), viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode")); err != nil {
		panic(err)
	}
	dir := viper.GetString("logger.dir")
	if dir == "" {
		dir = config.RunDir
	}
	log.InitJob(dir, viper.GetString("logger.level"), viper.GetInt("logger.maxDay"))
	err = InitAllConnection(dir, stopCh)
	if err != nil {
		panic(err)
	}

	if viper.GetString("application.environment") == "local" {
		initProxy()
	}

	httpRouter := server.NewEngine()
	server.RegisterHealth(httpRouter)
	chainevent.RegisterHealthRoutes(httpRouter)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", "", viper.GetInt("protocol.http.port")),
		Handler:      httpRouter,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  300 * time.Second,
	}
	httpServer.SetKeepAlivesEnabled(true) // true recommended
	go httpServer.ListenAndServe()
	if err := job.StartAll(); err != nil {
		panic(err)
	}

	elegantExit(&[]*http.Server{httpServer}, stopCh)
}

func InitAllConnection(dir string, stopCh chan struct{}) error {
	var err error
	// 数据库连接类初始化
	jobDb.Client = &jobDb.Connection{}
	writer := log.NewWriter(dir, "mysql", viper.GetInt("logger.maxDay"))
	jobDb.Client.Mysql, err = db.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), viper.GetString("db.mysql.readHost"), viper.GetString("db.mysql.readUser"), viper.GetString("db.mysql.readPassword"), viper.GetString("db.mysql.databaseName"), writer, viper.GetString("logger.db_log_level"))
	if err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventCollationCompatibility(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainTxSigningMetadata(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireIDOPurchaseEntitlement(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireIDOReconciliationEvidence(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgEvidence(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgRecoveryScan(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgMaterialization(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgAssessment(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgPrerequisite(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgGeneration(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireV2OnlySourcePolicy(jobDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireSecurityControlPlane(jobDb.Client.Mysql); err != nil {
		return err
	}
	// redis
	jobDb.Client.Redis, err = redis.Connect(viper.GetStringSlice("db.redis.host"), viper.GetString("db.redis.password"))
	if err != nil {
		return err
	}

	if !viper.GetBool("fairLaunchMainnet.scanner.enabled") || !viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode") {
		err = chain.NewClient()
		if err != nil {
			return err
		}
	}
	return nil
}

func elegantExit(servers *[]*http.Server, stopCh chan struct{}) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	sig := <-quit

	fmt.Println("Shutdown Server...", sig)
	now, timeout := time.Now(), 10*time.Second
	jobsStopped := job.StopAll()
	for _, s := range *servers {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		if err := s.Shutdown(ctx); err != nil {
			fmt.Println("Server Shutdown err:", err)
		}
		cancel()
	}

	// 下级监听后 直接关闭各种通信
	close(stopCh)
	// 等待当前最多一页的固定区块对账/其他 cron job 完成，再关闭 DB/RPC。
	select {
	case <-jobsStopped.Done():
	case <-time.After(70 * time.Second):
		fmt.Println("Timed out waiting for jobs to stop; closing clients fail-closed")
	}
	// 关闭所有client
	jobDb.Close()
	chain.Close()
	fmt.Println("Exited in", time.Since(now))
}

func initProxy() {
	proxyUrl, _ := url.Parse("http://127.0.0.1:7897") // ← 换成你的代理端口
	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
}
