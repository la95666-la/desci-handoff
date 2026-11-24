package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// 简单内存缓存：60s 内同 key 复用结果，避开每次请求都 COUNT 大表
var (
	statsCacheMu sync.RWMutex
	statsCache   struct {
		at   time.Time
		data gin.H
	}
)

const statsTTL = 60 * time.Second

const (
	dailyVisitKeyPrefix         = "jc:analytics:visits:"
	dailyUniqueVisitorKeyPrefix = "jc:analytics:unique_visitors:"
	dailyVisitRetention         = 400 * 24 * time.Hour
)

var anonymousVisitorIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type recordPageVisitReq struct {
	Path      string `json:"path" binding:"required"`
	VisitorID string `json:"visitor_id"`
}

func normalizeAnonymousVisitorID(visitorID string) (string, bool) {
	visitorID = strings.TrimSpace(visitorID)
	return visitorID, visitorID == "" || anonymousVisitorIDPattern.MatchString(visitorID)
}

func dailyAnonymousVisitorMember(day, visitorID string) string {
	digest := sha256.Sum256([]byte(day + ":" + visitorID))
	return hex.EncodeToString(digest[:])
}

// RecordPageVisit POST /client/stats/v1/visit
// 记录 C 端 SPA 路由访问量（PV / UV）。只保存按天总量和日期隔离的匿名访客哈希。
func RecordPageVisit(c *gin.Context) {
	var req recordPageVisitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "VISIT_BAD_REQUEST", "message": err.Error()})
		return
	}

	path := strings.TrimSpace(req.Path)
	if !strings.HasPrefix(path, "/") || len(path) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "VISIT_INVALID_PATH", "message": "path must start with / and be at most 200 characters"})
		return
	}
	visitorID, validVisitorID := normalizeAnonymousVisitorID(req.VisitorID)
	if !validVisitorID {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "VISIT_INVALID_VISITOR_ID", "message": "visitor_id must be 16-64 URL-safe characters"})
		return
	}

	redisConn := db.Redis()
	if redisConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "VISIT_STORE_UNAVAILABLE"})
		return
	}
	day := time.Now().Format("2006-01-02")
	key := dailyVisitKeyPrefix + day
	count := redisConn.Incr(c.Request.Context(), key)
	if count == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "VISIT_STORE_UNAVAILABLE"})
		return
	}
	if count == 1 {
		redisConn.Expire(c.Request.Context(), key, dailyVisitRetention)
	}
	if visitorID != "" {
		uniqueVisitorKey := dailyUniqueVisitorKeyPrefix + day
		member := dailyAnonymousVisitorMember(day, visitorID)
		if redisConn.SAdd(c.Request.Context(), uniqueVisitorKey, member) == 1 {
			redisConn.Expire(c.Request.Context(), uniqueVisitorKey, dailyVisitRetention)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": true})
}

// PublicStatsOverview GET /client/stats/v1/overview
// 返回首页数据看板的三大计数器
//   - users:       注册用户数（user 表）
//   - tasks:       已启用任务数（task is_enable=1）
//   - total_point: 全平台累计已发积分（user_point.point SUM）
func PublicStatsOverview(c *gin.Context) {
	statsCacheMu.RLock()
	if !statsCache.at.IsZero() && time.Since(statsCache.at) < statsTTL {
		out := statsCache.data
		statsCacheMu.RUnlock()
		c.JSON(http.StatusOK, gin.H{"status": true, "data": out})
		return
	}
	statsCacheMu.RUnlock()

	var (
		users      int64
		tasks      int64
		totalPoint int64
	)

	dbm := db.Mysql()
	dbm.Model(&entity.User{}).Count(&users)
	dbm.Model(&entity.Task{}).Where("is_enable = 1").Count(&tasks)
	// COALESCE 避免空表返 NULL → scan 报 panic
	dbm.Model(&entity.UserPoint{}).Select("COALESCE(SUM(point),0)").Scan(&totalPoint)

	out := gin.H{
		"users":       users,
		"tasks":       tasks,
		"total_point": totalPoint,
	}
	statsCacheMu.Lock()
	statsCache.at = time.Now()
	statsCache.data = out
	statsCacheMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": true, "data": out})
}
