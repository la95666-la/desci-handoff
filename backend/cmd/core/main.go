package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/database/db"
	"joincare_server/pkg/common/database/redis"
	commonSchema "joincare_server/pkg/common/database/schema"
	"joincare_server/pkg/common/http/server"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/storage"
	"joincare_server/pkg/common/utils"
	"joincare_server/pkg/core/admin"
	adminMiddleware "joincare_server/pkg/core/admin/middleware"
	coreDb "joincare_server/pkg/core/db"
	coreFlag "joincare_server/pkg/core/flag"
	"joincare_server/pkg/core/route"
	projectService "joincare_server/pkg/core/service/project"
	"joincare_server/pkg/core/third/chain"
	"joincare_server/pkg/core/third/twitter"
)

func main() {
	location, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = location
	stopCh := make(chan struct{})
	var path string
	flag.StringVar(&path, "path", "./config/", "--path prod")
	// 读取文件中的配置
	err := config.InitConfig(filepath.Join(path, "core"), "config.yaml")
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
	dir := viper.GetString("logger.dir")
	if dir == "" {
		dir = config.RunDir
	}
	i18n.I18nInit(filepath.Join(path, "common", "i18n"))

	log.Init(dir, viper.GetString("logger.level"), viper.GetInt("logger.maxDay"))
	if err := adminMiddleware.ValidateJWTSecrets(); err != nil {
		panic(err)
	}
	err = InitAllConnection(dir, stopCh)
	if err != nil {
		panic(err)
	}
	utils.SonyFlakeInit(1)

	if viper.GetString("application.environment") == "local" {
		initProxy()
	}

	httpRouter := server.NewEngine()
	server.RegisterHealth(httpRouter)
	// Sprint A: 静态文件（Documents / TipTap 图片）走 backend serve，nginx proxy 到 /uploads/
	httpRouter.Static("/uploads", viper.GetString("storage.local.root_dir"))
	g := httpRouter.Group("/client")
	route.Register(g)
	admin.RegisterClient(g) // 公开新闻 / 文案接口挂在 /client/* 下，无 sign 校验
	admin.Register(httpRouter.Group("/admin"))
	httpServer := &http.Server{
		Addr:        fmt.Sprintf("%s:%d", "", viper.GetInt("protocol.http.port")),
		Handler:     httpRouter,
		ReadTimeout: 60 * time.Second,
		// V3 policy review recomputes a pinned preview, then independently
		// rechecks original receipt and scanner before its immutable write.
		// Only the V3 review path uses the longer proxy/client deadlines;
		// other routes retain their own handler-level budgets.
		WriteTimeout: 180 * time.Second,
		IdleTimeout:  300 * time.Second,
	}
	httpServer.SetKeepAlivesEnabled(true) // true recommended
	go httpServer.ListenAndServe()
	// UpdatePlatformFee 调用已移除（安全审计 LOW-2）：平台费快照在项目 create 时固化，
	// 运行期改配置不应自动用 relayer 私钥直发 setPlatformFees。
	elegantExit(&[]*http.Server{httpServer}, stopCh)
}

func InitAllConnection(dir string, stopCh chan struct{}) error {
	var err error
	// 数据库连接类初始化
	coreDb.Client = &coreDb.Connection{}
	writer := log.NewWriter(dir, "mysql", viper.GetInt("logger.maxDay"))
	coreDb.Client.Mysql, err = db.NewMysqlClient(viper.GetString("db.mysql.host"), viper.GetString("db.mysql.user"), viper.GetString("db.mysql.password"), viper.GetString("db.mysql.readHost"), viper.GetString("db.mysql.readUser"), viper.GetString("db.mysql.readPassword"), viper.GetString("db.mysql.databaseName"), writer, viper.GetString("logger.db_log_level"))
	if err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventCollationCompatibility(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainTxSigningMetadata(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireIDOPurchaseEntitlement(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireIDOReconciliationEvidence(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgEvidence(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgRecoveryScan(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgMaterialization(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgAssessment(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgPrerequisite(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireChainEventReorgGeneration(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireV2OnlySourcePolicy(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireSecurityControlPlane(coreDb.Client.Mysql); err != nil {
		return err
	}
	if err := commonSchema.RequireProjectRestore(coreDb.Client.Mysql); err != nil {
		return err
	}
	// redis
	coreDb.Client.Redis, err = redis.Connect(viper.GetStringSlice("db.redis.host"), viper.GetString("db.redis.password"))
	if err != nil {
		return err
	}
	twitter.NewConfig(viper.GetString("twitter.clientID"), viper.GetString("twitter.clientSecret"), viper.GetString("twitter.redirectURL"))

	err = initChainClients()
	if err != nil {
		return err
	}

	// Sprint A: 文件上传器（LocalFs 默认；未来切 MinIO 只改 config）
	if err := storage.Init(); err != nil {
		return err
	}
	return nil
}

func initChainClients() error {
	// Missing v1.enabled preserves legacy behavior. New V2-only environments
	// must set it explicitly to false and must not invent a V1 address.
	v1Configured := strings.TrimSpace(strings.ToLower(viper.GetString("chain.project.v1.enabled")))
	if v1Configured != "" && v1Configured != "true" && v1Configured != "false" {
		return fmt.Errorf("chain.project.v1.enabled must be true or false")
	}
	if v1Configured != "false" {
		if err := initChainClient(constant.ChainProject, false); err != nil {
			return err
		}
	}
	if err := initProjectV2ChainClient(); err != nil {
		return err
	}
	if err := initProjectV3ChainClient(); err != nil {
		return err
	}
	if err := initChainClient(constant.ChainTask, false); err != nil {
		return err
	}
	return nil
}

func initProjectV2ChainClient() error {
	proxy := strings.TrimSpace(viper.GetString("chain.project.v2.contractAddress"))
	if proxy == "" {
		return nil
	}
	jobPrivate := strings.TrimSpace(viper.GetString("chain.project.jobPrivate"))
	if jobPrivate == "" {
		return fmt.Errorf("chain.project.jobPrivate is required when IDO V2 relaying is enabled")
	}
	if err := chain.NewClient(chain.Config{
		Name:            constant.ChainProjectV2,
		URL:             viper.GetString("chain.project.url"),
		Private:         jobPrivate,
		Token:           coreFlag.Token(),
		ChainID:         viper.GetInt64("chain.project.chainID"),
		ContractAddress: proxy,
	}); err != nil {
		return err
	}
	legacy := chain.GetClient(constant.ChainProject)
	v2 := chain.GetClient(constant.ChainProjectV2)
	if legacy != nil && v2 != nil && legacy.Address == v2.Address {
		return fmt.Errorf("IDO V2/job relayer must not share the legacy core relayer address %s", v2.Address.Hex())
	}
	return nil
}

// V3 uses a separate proxy and ABI. The operator may share the sender with
// V2 only through chain_relayer_nonce, never through a V2 client identity.
// This config merely loads a client; routing/signing remains independently
// gated and disabled by default.
func initProjectV3ChainClient() error {
	if !viper.GetBool("idoV3Execution.enabled") {
		return nil
	}
	proxy := strings.TrimSpace(viper.GetString("idoV3Execution.proxyAddress"))
	operator := strings.TrimSpace(viper.GetString("idoV3Execution.operatorAddress"))
	if !common.IsHexAddress(proxy) || !common.IsHexAddress(operator) ||
		common.HexToAddress(proxy) == (common.Address{}) || common.HexToAddress(operator) == (common.Address{}) ||
		strings.EqualFold(proxy, viper.GetString("chain.project.v2.contractAddress")) ||
		strings.TrimSpace(viper.GetString("chain.project.jobPrivate")) == "" {
		return fmt.Errorf("IDO V3 execution identity unavailable")
	}
	if viper.GetBool("idoV3Approved.enabled") {
		// Approved mode has no project-scoped offline release signature. It
		// still requires the independent, fixed deployment and schema, and
		// cannot borrow the V2 client or an unreviewed operator address.
		if !projectService.ApprovedV3ModeEnabled() ||
			viper.GetInt64("chain.project.chainID") != 56 ||
			common.HexToAddress(proxy) != common.HexToAddress("0x0D4040875742975269c55E16C8884d3Ee9510B8E") ||
			common.HexToAddress(operator) != common.HexToAddress("0xfd712061ee08E070A930eCD45E935Ae5d7f5445C") {
			return fmt.Errorf("IDO V3 approved execution prerequisites unavailable")
		}
	} else {
		key, keyErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Set.pinnedPublicKey"))
		sig, sigErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Set.signature"))
		var set idov3whitelist.SetManifest
		decoder := json.NewDecoder(strings.NewReader(viper.GetString("idoV3Set.manifest")))
		decoder.DisallowUnknownFields()
		var extra any
		// The Set grant expires for new creates; its immutable identity remains
		// verifiable for a later, independently admitted lifecycle release.
		if keyErr != nil || sigErr != nil || len(key) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize ||
			decoder.Decode(&set) != nil || decoder.Decode(&extra) != io.EOF ||
			set.ExpiresAt <= 1 ||
			idov3whitelist.VerifySignedSetManifest(ed25519.PublicKey(key), set, sig,
				common.HexToHash(set.RequestDigest), time.Unix(set.ExpiresAt-1, 0)) != nil ||
			!strings.EqualFold(set.ProxyAddress, proxy) || !strings.EqualFold(set.Operator, operator) ||
			set.ChainID != uint64(viper.GetInt64("chain.project.chainID")) {
			return fmt.Errorf("IDO V3 execution signed Set identity unavailable")
		}
		scanKey, scanKeyErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Scanner.pinnedPublicKey"))
		scan, scanErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Scanner.signature"))
		var manifest idov3whitelist.AdmissionManifest
		scanDecoder := json.NewDecoder(strings.NewReader(viper.GetString("idoV3Scanner.manifest")))
		scanDecoder.DisallowUnknownFields()
		var scanExtra any
		if !viper.GetBool("idoV3Scanner.enabled") || scanKeyErr != nil || scanErr != nil ||
			len(scanKey) != ed25519.PublicKeySize || len(scan) != ed25519.SignatureSize ||
			scanDecoder.Decode(&manifest) != nil || scanDecoder.Decode(&scanExtra) != io.EOF ||
			idov3whitelist.VerifySignedScanManifest(ed25519.PublicKey(scanKey), manifest, scan, time.Now()) != nil ||
			manifest.ChainID != set.ChainID || manifest.ProjectID != set.ProjectID ||
			manifest.ProxyAddress != set.ProxyAddress || manifest.ProjectToken != set.ProjectToken ||
			manifest.PaymentToken != set.PaymentToken || manifest.Implementation != set.Implementation ||
			manifest.ProxyCodeHash != set.ProxyCodeHash ||
			manifest.ImplementationCodeHash != set.ImplementationCodeHash ||
			manifest.StartBlock != set.StartBlock || manifest.Confirmations != set.Confirmations ||
			manifest.Governor != set.Governor || manifest.ExpiresAt <= time.Now().Unix() {
			return fmt.Errorf("IDO V3 execution scanner identity unavailable")
		}
	}
	if err := chain.NewClient(chain.Config{
		Name: constant.ChainProjectV3, URL: viper.GetString("chain.project.url"),
		Private: viper.GetString("chain.project.jobPrivate"), Token: coreFlag.Token(),
		ChainID: viper.GetInt64("chain.project.chainID"), ContractAddress: proxy,
	}); err != nil {
		return fmt.Errorf("IDO V3 execution client unavailable")
	}
	client := chain.GetClient(constant.ChainProjectV3)
	if client == nil || client.Address != common.HexToAddress(operator) ||
		client.ContractAddress != common.HexToAddress(proxy) || client.ChainID == nil ||
		!client.ChainID.IsUint64() || client.ChainID.Uint64() != uint64(viper.GetInt64("chain.project.chainID")) {
		return fmt.Errorf("IDO V3 operator custody unavailable")
	}
	if viper.GetBool("idoV3Approved.enabled") && !projectService.ApprovedV3ModeReady() {
		return fmt.Errorf("IDO V3 approved execution prerequisites unavailable")
	}
	return nil
}

func initChainClient(name string, required bool) error {
	prefix := "chain." + name
	if viper.GetString(prefix+".contractAddress") == "" {
		if required {
			return fmt.Errorf("%s.contractAddress is empty", prefix)
		}
		return nil
	}
	return chain.NewClient(chain.Config{
		Name:            name,
		URL:             viper.GetString(prefix + ".url"),
		Private:         viper.GetString(prefix + ".private"),
		Token:           coreFlag.Token(),
		ChainID:         viper.GetInt64(prefix + ".chainID"),
		ContractAddress: viper.GetString(prefix + ".contractAddress"),
	})
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
	coreDb.Close()
	chain.Close()
	fmt.Println("Exited in", time.Since(now))
}

func initProxy() {
	proxyUrl, _ := url.Parse("http://127.0.0.1:7897") // ← 换成你的代理端口
	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
}
