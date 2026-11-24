package server

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/config"
	"joincare_server/pkg/common/log"
)

const (
	maxRequestBodyBytes int64 = 64 << 20
	maxMultipartMemory        = 24 << 20
)

func Start(host string, router *gin.Engine) *http.Server {
	RegisterHealth(router)
	httpServer := &http.Server{
		Addr:              host,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	httpServer.SetKeepAlivesEnabled(true) // true recommended
	go httpServer.ListenAndServe()
	return httpServer
}

func NewEngine() *gin.Engine {
	if config.Prod() {
		gin.SetMode(gin.ReleaseMode)
	}
	gin.DefaultWriter = log.NewWriter(config.RunDir, viper.GetString("application.name")+"-gin", viper.GetInt("logger.maxDay"))
	r := gin.New()
	r.MaxMultipartMemory = maxMultipartMemory
	r.Use(gin.Recovery())
	r.Use(RequestBodyLimit(maxRequestBodyBytes))
	r.Use(Cors)
	return r
}

func RequestBodyLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"status": false, "code": "REQUEST_BODY_TOO_LARGE", "message": "request body exceeds limit",
			})
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func RegisterHealth(r *gin.Engine) {
	g := r.Group("")
	g.GET("/health", func(c *gin.Context) {
		c.String(200, `{"status":true,"data":null,"message":null,"error":null,"code":""}`)
	})
}

// allowedOrigin 判定跨域来源是否合法。
// 白名单来自配置 app.domain / app.adminDomain（bare host 或带 scheme 均可）；
// 非 prod 环境额外放行 localhost / 127.0.0.1 任意端口（本地开发）。
// 空配置 = 全部跨域拒绝（fail-closed，部署清单要求配置域名）。
func AllowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if !config.Prod() {
		if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
			return true
		}
	}
	for _, raw := range []string{viper.GetString("app.domain"), viper.GetString("app.adminDomain")} {
		d := strings.TrimSpace(raw)
		if d == "" {
			continue
		}
		// 配置可能是 bare host（joincarelabs.com）或完整 URL（https://...），统一归一化
		d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
		d = strings.TrimRight(strings.SplitN(d, "/", 2)[0], "/")
		if u.Host == d || u.Hostname() == d || u.Hostname() == "www."+d {
			return true
		}
	}
	return false
}

func Cors(c *gin.Context) {
	origin := c.Request.Header.Get("Origin")
	// 仅对白名单内的 Origin 回显并允许携带凭据；其余来源不下发 CORS 头（浏览器侧拦截）。
	if origin != "" && AllowedOrigin(origin) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Vary", "Origin")
	}
	c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE,PATCH")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
	c.Header("Access-Control-Max-Age", "864000")
	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(http.StatusNoContent)
	}
	// 处理请求
	c.Next()
}
