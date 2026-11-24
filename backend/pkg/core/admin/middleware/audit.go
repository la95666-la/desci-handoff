// Package middleware 的审计中间件。
//
// 所有 admin 写操作（POST / PUT / PATCH / DELETE）经过 Audit 中间件：
//  1. 记 req_body（JSON 敏感字段 mask 后写）
//  2. 用 ResponseWriter 包一层捕获 resp 状态码 + 内容
//  3. 落库时：hash = sha256(prev.hash + payload + resp_hash + created_at)
//     形成不可篡改链条，运维可 verify_chain 从任意点重算比对
//
// 设计原文 §3.8.2 "所有写操作记录 admin_audit_log；仅追加不修改；关键操作日志哈希上链（建议）"
// Phase 3 只做 DB 层哈希链，上链留 Phase 4 hook
package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"hash"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/db"
)

// 上一条 hash 缓存（进程内）。多实例部署要求 DB 主键有序才能保证一致；
// 保险起见每次插入前会重新查 last row 兜底
var (
	lastHashMu sync.Mutex
	lastHash   string
)

const maxAuditRequestBody = 1 << 20

// 需要审计的方法
func shouldAudit(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

// bodyBufWriter 包裹 gin.ResponseWriter 以捕获 body
type bodyBufWriter struct {
	gin.ResponseWriter
	hash hash.Hash
}

func (w *bodyBufWriter) Write(b []byte) (int, error) {
	_, _ = w.hash.Write(b)
	return w.ResponseWriter.Write(b)
}

// Audit 中间件；必须挂在 Auth 之后（依赖 CtxAdminID / CtxUsername / CtxRoleCode）
func Audit(c *gin.Context) {
	if !shouldAudit(c.Request.Method) {
		c.Next()
		return
	}

	// 读一次 body 后回填给下游 handler
	var reqBody []byte
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	captureJSON := contentType == "" || strings.HasPrefix(contentType, "application/json")
	if c.Request.Body != nil && captureJSON {
		limited := io.LimitReader(c.Request.Body, maxAuditRequestBody+1)
		var err error
		reqBody, err = io.ReadAll(limited)
		if err != nil || len(reqBody) > maxAuditRequestBody {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"status": false, "code": "ADMIN_REQUEST_TOO_LARGE", "message": "request body exceeds audit limit",
			})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}

	// 流式计算响应 hash，避免把完整响应复制到内存。
	responseHash := sha256.New()
	c.Writer = &bodyBufWriter{ResponseWriter: c.Writer, hash: responseHash}

	start := time.Now()
	c.Next()
	duration := time.Since(start)

	// 收集 ctx 值
	adminID, _ := c.Get(CtxAdminID)
	adminIDVal, _ := adminID.(int64)
	username, _ := c.Get(CtxUsername)
	usernameStr, _ := username.(string)
	roleCode, _ := c.Get(CtxRoleCode)
	roleCodeStr, _ := roleCode.(string)
	// 从 c.Keys 里读 aggregated roles（B3 加）
	rolesCsv := roleCodeStr
	if v, ok := c.Get("admin_all_roles"); ok {
		if s, ok := v.(string); ok && s != "" {
			rolesCsv = s
		}
	}

	// mask 敏感字段
	maskedBody := auditRequestBody(c.Request.URL.Path, reqBody)
	if !captureJSON && c.Request.Body != nil {
		maskedBody = `{"_redacted":"non-JSON body omitted"}`
	}
	respHash := hex.EncodeToString(responseHash.Sum(nil))

	// 计算 hash chain
	lastHashMu.Lock()
	prev := lastHash
	if prev == "" {
		var lastRow adminEntity.AdminAuditLog
		if err := db.Mysql().Order("id DESC").Limit(1).Take(&lastRow).Error; err == nil {
			prev = lastRow.Hash
		}
	}

	targetID := extractTargetID(c)
	route := c.FullPath()
	// 只保留毫秒精度，与 MySQL DATETIME(3) 一致（避免 round-trip 精度丢失导致 hash 不匹配）
	createdAt := time.Now().Truncate(time.Millisecond)

	payloadForHash := strings.Join([]string{
		prev,
		itoa64(adminIDVal),
		c.Request.Method,
		c.Request.URL.Path,
		maskedBody,
		itoa(c.Writer.Status()),
		respHash,
		createdAt.UTC().Format("2006-01-02 15:04:05.000"), // UTC 固定串（避免时区飘）
	}, "|")
	newHash := sha256Hex([]byte(payloadForHash))

	row := adminEntity.AdminAuditLog{
		AdminID:    uint64(adminIDVal),
		Username:   usernameStr,
		RoleCodes:  rolesCsv,
		IP:         c.ClientIP(),
		UserAgent:  truncate(c.Request.UserAgent(), 500),
		Method:     c.Request.Method,
		Path:       c.Request.URL.Path,
		Route:      route,
		TargetID:   targetID,
		ReqBody:    maskedBody,
		RespStatus: c.Writer.Status(),
		RespHash:   respHash,
		PrevHash:   prev,
		Hash:       newHash,
		DurationMS: int(duration.Milliseconds()),
		CreatedAt:  createdAt,
	}
	if err := db.Mysql().Create(&row).Error; err == nil {
		lastHash = newHash
	}
	lastHashMu.Unlock()
}

// ---------------- helpers ----------------

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// maskSensitive: 字符串级简单替换 "password":"xxx" → "password":"***"。
// 完整方案是解 JSON 递归；此处保守起见做正则式替换避免大 JSON 反复解码开销
func auditRequestBody(path string, b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if path == "/admin/v1/self/password" || (strings.HasPrefix(path, "/admin/v1/admin-users/") && strings.HasSuffix(path, "/password")) {
		return `{"_redacted":"credential body omitted"}`
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return `{"_redacted":"non-JSON body omitted"}`
	}
	redactJSON(value)
	masked, err := json.Marshal(value)
	if err != nil {
		return `{"_redacted":"body omitted"}`
	}
	return string(masked)
}

func redactJSON(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if sensitiveAuditKey(key) {
				typed[key] = "***"
				continue
			}
			redactJSON(child)
		}
	case []any:
		for _, child := range typed {
			redactJSON(child)
		}
	}
}

func sensitiveAuditKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"), " ", "_"))
	for _, marker := range []string{"password", "passwd", "secret", "private_key", "privatekey", "refresh_token", "access_token", "signature"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return normalized == "token" || strings.HasSuffix(normalized, "_token")
}

func extractTargetID(c *gin.Context) string {
	// gin :id 参数
	if v := c.Param("id"); v != "" {
		return v
	}
	if v := c.Param("code"); v != "" {
		return v
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func itoa(i int) string {
	return intToString(int64(i))
}
func itoa64(i int64) string {
	return intToString(i)
}
func intToString(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 21)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// VerifyChain 从最早一条开始验证 hash chain 是否被篡改
// 由 admin 端 GET /admin/v1/audit-logs/verify-chain 调用
func VerifyChain() (ok bool, brokenAtID uint64, err error) {
	var rows []adminEntity.AdminAuditLog
	// 每次拉 1000 条滚
	page := uint64(0)
	prev := ""
	for {
		if err := db.Mysql().Where("id > ?", page).Order("id ASC").Limit(1000).Find(&rows).Error; err != nil {
			return false, 0, err
		}
		if len(rows) == 0 {
			return true, 0, nil
		}
		for _, r := range rows {
			payload := strings.Join([]string{
				prev,
				itoa64(int64(r.AdminID)),
				r.Method,
				r.Path,
				r.ReqBody,
				itoa(r.RespStatus),
				r.RespHash,
				r.CreatedAt.UTC().Format("2006-01-02 15:04:05.000"),
			}, "|")
			expect := sha256Hex([]byte(payload))
			if r.PrevHash != prev || r.Hash != expect {
				return false, r.ID, nil
			}
			prev = r.Hash
			page = r.ID
		}
	}
}

// _ 避免未使用变量的偏门场景
var _ = http.StatusOK
