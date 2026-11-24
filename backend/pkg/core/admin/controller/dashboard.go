package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// DashboardOverview GET /admin/v1/dashboard/overview
// 后台首页聚合统计：一次拉齐 admin 首页需要的所有真实数据
func DashboardOverview(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sevenDaysAgo := todayStart.AddDate(0, 0, -6)   // 含今天共 7 天
	thirtyDaysAgo := todayStart.AddDate(0, 0, -29) // 含今天共 30 天

	// ===== 用户 =====
	var userTotals struct {
		Total    int64 `gorm:"column:total"`
		NewToday int64
		New7d    int64
	}
	if err := db.Mysql().WithContext(ctx).Raw(`SELECT COUNT(*) AS total FROM user`).Scan(&userTotals).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}

	// 用户增长趋势 —— 最近 30 天每日新增
	type dayCount struct {
		D time.Time `gorm:"column:d"`
		N int64     `gorm:"column:n"`
	}
	var growthRows []dayCount
	if err := db.Mysql().WithContext(ctx).Model(&entity.User{}).
		Select("DATE(created_at) AS d, COUNT(*) AS n").
		Where("created_at >= ?", thirtyDaysAgo).
		Group("DATE(created_at)").
		Order("d ASC").
		Scan(&growthRows).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}
	// 补齐 30 天（缺失的日期填 0）
	growthMap := map[string]int64{}
	sevenDaysKey := sevenDaysAgo.Format("2006-01-02")
	todayKey := todayStart.Format("2006-01-02")
	for _, r := range growthRows {
		day := r.D.Format("2006-01-02")
		growthMap[day] = r.N
		// Compare calendar dates, not time.Location: MySQL DATE Scan can
		// materialize at UTC midnight while todayStart uses the server TZ.
		if day >= sevenDaysKey {
			userTotals.New7d += r.N
		}
		if day >= todayKey {
			userTotals.NewToday += r.N
		}
	}
	userGrowth := make([]gin.H, 0, 30)
	for i := 0; i < 30; i++ {
		d := thirtyDaysAgo.AddDate(0, 0, i).Format("2006-01-02")
		userGrowth = append(userGrowth, gin.H{"date": d, "new_users": growthMap[d]})
	}

	// ===== 项目 =====
	var projectTotals struct {
		Total     int64 `gorm:"column:total"`
		PreLaunch int64 `gorm:"column:pre_launch"`
		Warmup    int64 `gorm:"column:warmup"`
		Active    int64 `gorm:"column:active"`
		Ended     int64 `gorm:"column:ended"`
	}
	if err := db.Mysql().WithContext(ctx).Raw(`
		SELECT COUNT(*) AS total,
		       COALESCE(SUM(status = 'pre_launch'), 0) AS pre_launch,
		       COALESCE(SUM(status = 'warmup'), 0) AS warmup,
		       COALESCE(SUM(status = 'active'), 0) AS active,
		       COALESCE(SUM(status IN ('successful','failed','canceled','ended','vesting','archived')), 0) AS ended
		FROM projects`).Scan(&projectTotals).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}

	// ===== Launchpad engagement =====
	var subscribersTotal, watchesTotal, whitelistTotal int64
	if err := db.Mysql().WithContext(ctx).Model(&entity.ProjectSubscriber{}).Count(&subscribersTotal).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}
	if err := db.Mysql().WithContext(ctx).Model(&entity.ProjectWatch{}).Count(&watchesTotal).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}
	if err := db.Mysql().WithContext(ctx).Model(&entity.ProjectWhitelist{}).Count(&whitelistTotal).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}

	// Top 项目 by (subscribers + watches + whitelist)
	type projectAgg struct {
		ProjectID   int    `gorm:"column:project_id" json:"project_id"`
		Name        string `gorm:"column:name" json:"name"`
		Subscribers int64  `gorm:"column:subscribers" json:"subscribers"`
		Watches     int64  `gorm:"column:watches" json:"watches"`
		Whitelist   int64  `gorm:"column:whitelist" json:"whitelist"`
		Total       int64  `gorm:"column:total" json:"total"`
	}
	var topProjects []projectAgg
	// 用 3 个 LEFT JOIN + 聚合。project.name 用 name_en 展示
	if err := db.Mysql().WithContext(ctx).Raw(`
		SELECT p.project_id AS project_id,
		       p.name_en AS name,
		       COALESCE(s.cnt, 0) AS subscribers,
		       COALESCE(w.cnt, 0) AS watches,
		       COALESCE(wl.cnt, 0) AS whitelist,
		       (COALESCE(s.cnt,0) + COALESCE(w.cnt,0) + COALESCE(wl.cnt,0)) AS total
		FROM projects p
		LEFT JOIN (SELECT project_id, COUNT(*) AS cnt FROM project_subscribers GROUP BY project_id) s ON s.project_id = p.project_id
		LEFT JOIN (SELECT project_id, COUNT(*) AS cnt FROM project_watches      GROUP BY project_id) w ON w.project_id = p.project_id
		LEFT JOIN (SELECT project_id, COUNT(*) AS cnt FROM project_whitelists   GROUP BY project_id) wl ON wl.project_id = p.project_id
		ORDER BY total DESC, p.project_id ASC
		LIMIT 5
	`).Scan(&topProjects).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ADMIN_DASHBOARD_DATA_UNAVAILABLE"})
		return
	}

	// ===== 签到 =====
	var checkinToday int64
	_ = db.Mysql().WithContext(ctx).Model(&entity.UserCheckInRecord{}).
		Where("check_in_date = ?", time.Now().UTC().Format("2006-01-02")).
		Count(&checkinToday).Error

	// ===== TG 活跃 =====
	var tgMsgsToday, tgActiveUsersToday int64
	_ = db.Mysql().WithContext(ctx).Model(&entity.TgDailyActivity{}).
		Where("day = ?", todayStart.Format("2006-01-02")).
		Select("COALESCE(SUM(message_count), 0)").
		Scan(&tgMsgsToday).Error
	_ = db.Mysql().WithContext(ctx).Model(&entity.TgDailyActivity{}).
		Where("day = ?", todayStart.Format("2006-01-02")).
		Select("COUNT(DISTINCT tg_user_id)").
		Scan(&tgActiveUsersToday).Error

	// ===== C 端访问量（PV / UV） =====
	visitKeys := make([]string, 0, 30)
	uniqueVisitorKeys := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		d := thirtyDaysAgo.AddDate(0, 0, i).Format("2006-01-02")
		visitKeys = append(visitKeys, dailyVisitKeyPrefix+d)
		uniqueVisitorKeys = append(uniqueVisitorKeys, dailyUniqueVisitorKeyPrefix+d)
	}
	visitCounts := map[string]string{}
	uniqueVisitorCounts := map[string]int64{}
	if redisConn := db.Redis(); redisConn != nil {
		visitCounts = redisConn.MGet(ctx, visitKeys...)
		uniqueVisitorCounts = redisConn.MSCard(ctx, uniqueVisitorKeys...)
	}
	visits := make([]gin.H, 0, 30)
	for i, key := range visitKeys {
		d := thirtyDaysAgo.AddDate(0, 0, i).Format("2006-01-02")
		count, _ := strconv.ParseInt(visitCounts[key], 10, 64)
		visits = append(visits, gin.H{
			"date":            d,
			"visits":          count,
			"unique_visitors": uniqueVisitorCounts[uniqueVisitorKeys[i]],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"user": gin.H{
				"total":     userTotals.Total,
				"new_today": userTotals.NewToday,
				"new_7d":    userTotals.New7d,
			},
			"user_growth_30d": userGrowth,
			"visits_30d":      visits,
			"project": gin.H{
				"total":      projectTotals.Total,
				"pre_launch": projectTotals.PreLaunch,
				"warmup":     projectTotals.Warmup,
				"active":     projectTotals.Active,
				"ended":      projectTotals.Ended,
			},
			"launchpad": gin.H{
				"subscribers": subscribersTotal,
				"watches":     watchesTotal,
				"whitelist":   whitelistTotal,
			},
			"top_projects": topProjects,
			"checkin": gin.H{
				"today": checkinToday,
			},
			"tg": gin.H{
				"messages_today":     tgMsgsToday,
				"active_users_today": tgActiveUsersToday,
			},
		},
	})
}
