package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/service/notification"
	"joincare_server/pkg/core/service/sanitize"
)

type newsTranslationPayload struct {
	Language   string `json:"language" binding:"required"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Content    string `json:"content"`
	Author     string `json:"author"`
	AuthorRole string `json:"author_role"`
}

type newsUpsertReq struct {
	Slug              string                   `json:"slug" binding:"required"`
	Category          string                   `json:"category"`
	CoverURL          string                   `json:"cover_url"`
	SourceURL         string                   `json:"source_url"`
	SourceName        string                   `json:"source_name"`
	IsFeatured        int                      `json:"is_featured"`
	IsImportant       int                      `json:"is_important"`
	AlertPriority     int                      `json:"alert_priority"`
	AlertStartAt      *string                  `json:"alert_start_at"`
	AlertEndAt        *string                  `json:"alert_end_at"`
	AlertFrequency    string                   `json:"alert_frequency"`
	AlertPages        string                   `json:"alert_pages"`
	AlertDelaySeconds int                      `json:"alert_delay_seconds"`
	AlertRevision     int                      `json:"alert_revision"`
	Sort              int                      `json:"sort"`
	Status            int                      `json:"status"`
	PublishAt         string                   `json:"publish_at"` // "2026-06-29 12:00:00"
	Translations      []newsTranslationPayload `json:"translations"`
}

type newsDetail struct {
	News         entity.News              `json:"news"`
	Translations []entity.NewsTranslation `json:"translations"`
}

func normalizeAlertFrequency(value string) string {
	switch value {
	case "daily", "session":
		return value
	default:
		return "once"
	}
}

func normalizeAlertPages(value string) string {
	allowed := map[string]bool{
		"all": true, "home": true, "ecosystem": true, "launchpad": true,
		"team": true, "tasks": true, "news": true,
	}
	seen := make(map[string]bool)
	pages := make([]string, 0, 4)
	for _, raw := range strings.Split(value, ",") {
		page := strings.ToLower(strings.TrimSpace(raw))
		if allowed[page] && !seen[page] {
			seen[page] = true
			pages = append(pages, page)
		}
	}
	if len(pages) == 0 {
		return "home,ecosystem,launchpad"
	}
	return strings.Join(pages, ",")
}

func clampAlertDelay(value int) int {
	if value < 0 {
		return 0
	}
	if value > 15 {
		return 15
	}
	return value
}

func normalizeAlertRevision(value int) int {
	if value < 1 {
		return 1
	}
	return value
}

func normalizeFlag(value int) int {
	if value == 0 {
		return 0
	}
	return 1
}

func parseAlertSchedule(startValue, endValue *string) (*time.Time, *time.Time, error) {
	start := parseTime(startValue)
	end := parseTime(endValue)
	if startValue != nil && strings.TrimSpace(*startValue) != "" && start == nil {
		return nil, nil, errors.New("invalid alert_start_at")
	}
	if endValue != nil && strings.TrimSpace(*endValue) != "" && end == nil {
		return nil, nil, errors.New("invalid alert_end_at")
	}
	if start != nil && end != nil && !end.After(*start) {
		return nil, nil, errors.New("alert_end_at must be after alert_start_at")
	}
	return start, end, nil
}

// ListNews GET /admin/v1/news?page=1&page_size=20&category=&status=
func ListNews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	category := c.Query("category")
	keyword := c.Query("keyword")

	q := db.Mysql().Model(&entity.News{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("slug LIKE ? OR source_name LIKE ?", like, like)
	}
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			q = q.Where("status = ?", s)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_NEWS_LIST_FAILED", "message": err.Error()})
		return
	}

	var rows []entity.News
	if err := q.Order("sort ASC, publish_at DESC").
		Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_NEWS_LIST_FAILED", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"items":     rows,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetNews GET /admin/v1/news/:id  (含翻译)
func GetNews(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_ID", "message": "invalid id"})
		return
	}
	var n entity.News
	if err := db.Mysql().Where("id = ?", id).First(&n).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_NEWS_NOT_FOUND", "message": err.Error()})
		return
	}
	var tx []entity.NewsTranslation
	if err := db.Mysql().Where("news_id = ?", n.ID).Find(&tx).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_NEWS_TRANSLATIONS_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": newsDetail{News: n, Translations: tx}})
}

func parsePublishAt(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
}

// CreateNews POST /admin/v1/news
func CreateNews(c *gin.Context) {
	var req newsUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_REQUEST", "message": err.Error()})
		return
	}
	alertStart, alertEnd, err := parseAlertSchedule(req.AlertStartAt, req.AlertEndAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_ALERT_SCHEDULE", "message": err.Error()})
		return
	}
	n := entity.News{
		Slug:              req.Slug,
		Category:          req.Category,
		CoverURL:          req.CoverURL,
		SourceURL:         req.SourceURL,
		SourceName:        req.SourceName,
		IsFeatured:        req.IsFeatured,
		IsImportant:       normalizeFlag(req.IsImportant),
		AlertPriority:     req.AlertPriority,
		AlertStartAt:      alertStart,
		AlertEndAt:        alertEnd,
		AlertFrequency:    normalizeAlertFrequency(req.AlertFrequency),
		AlertPages:        normalizeAlertPages(req.AlertPages),
		AlertDelaySeconds: clampAlertDelay(req.AlertDelaySeconds),
		AlertRevision:     normalizeAlertRevision(req.AlertRevision),
		Sort:              req.Sort,
		Status:            req.Status,
		PublishAt:         parsePublishAt(req.PublishAt),
	}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&n).Error; err != nil {
			return err
		}
		for _, t := range req.Translations {
			row := entity.NewsTranslation{
				NewsID:     n.ID,
				Language:   t.Language,
				Title:      sanitize.PlainText(t.Title),
				Summary:    sanitize.PlainText(t.Summary),
				Content:    sanitize.RichHTML(t.Content),
				Author:     sanitize.PlainText(t.Author),
				AuthorRole: sanitize.PlainText(t.AuthorRole),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_NEWS_CREATE_FAILED", "message": err.Error()})
		return
	}
	// Phase 1: 状态=已发布 时全员广播一条站内信
	// 标题/正文按译文 by lang 注入 extra；Notifications.vue 渲染时优先取当前语言一条
	if req.Status == 1 {
		extra := make(map[string][]string, 3)
		for _, t := range req.Translations {
			lang := constant.NoticeLangEn
			switch t.Language {
			case "tw", "zh-tw":
				lang = constant.NoticeLangTw
			case "zh", "zh-cn":
				lang = constant.NoticeLangCn
			}
			extra[lang] = []string{t.Title, t.Summary}
		}
		go notification.Broadcast(int(n.ID), constant.NewsPublished, extra)
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": n.ID}})
}

// UpdateNews PUT /admin/v1/news/:id  (upsert translations by language)
func UpdateNews(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_ID", "message": "invalid id"})
		return
	}
	var req newsUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_REQUEST", "message": err.Error()})
		return
	}
	alertStart, alertEnd, scheduleErr := parseAlertSchedule(req.AlertStartAt, req.AlertEndAt)
	if scheduleErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_ALERT_SCHEDULE", "message": scheduleErr.Error()})
		return
	}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"slug":                req.Slug,
			"category":            req.Category,
			"cover_url":           req.CoverURL,
			"source_url":          req.SourceURL,
			"source_name":         req.SourceName,
			"is_featured":         req.IsFeatured,
			"is_important":        normalizeFlag(req.IsImportant),
			"alert_priority":      req.AlertPriority,
			"alert_start_at":      alertStart,
			"alert_end_at":        alertEnd,
			"alert_frequency":     normalizeAlertFrequency(req.AlertFrequency),
			"alert_pages":         normalizeAlertPages(req.AlertPages),
			"alert_delay_seconds": clampAlertDelay(req.AlertDelaySeconds),
			"alert_revision":      normalizeAlertRevision(req.AlertRevision),
			"sort":                req.Sort,
			"status":              req.Status,
			"publish_at":          parsePublishAt(req.PublishAt),
		}
		res := tx.Model(&entity.News{}).Where("id = ?", id).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		for _, t := range req.Translations {
			cleanTitle := sanitize.PlainText(t.Title)
			cleanSummary := sanitize.PlainText(t.Summary)
			cleanContent := sanitize.RichHTML(t.Content)
			cleanAuthor := sanitize.PlainText(t.Author)
			cleanAuthorRole := sanitize.PlainText(t.AuthorRole)
			var existing entity.NewsTranslation
			err := tx.Where("news_id = ? AND language = ?", id, t.Language).First(&existing).Error
			if err == nil {
				tx.Model(&existing).Updates(map[string]interface{}{
					"title": cleanTitle, "summary": cleanSummary, "content": cleanContent,
					"author": cleanAuthor, "author_role": cleanAuthorRole,
				})
			} else if err == gorm.ErrRecordNotFound {
				tx.Create(&entity.NewsTranslation{
					NewsID: id, Language: t.Language,
					Title: cleanTitle, Summary: cleanSummary, Content: cleanContent,
					Author: cleanAuthor, AuthorRole: cleanAuthorRole,
				})
			} else {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_NEWS_NOT_FOUND", "message": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_NEWS_UPDATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// DeleteNews DELETE /admin/v1/news/:id
func DeleteNews(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_NEWS_BAD_ID", "message": "invalid id"})
		return
	}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("news_id = ?", id).Delete(&entity.NewsTranslation{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&entity.News{}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_NEWS_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// PublicNewsList GET /client/news/v1/list?language=en&featured=1  (C 端公开)
func PublicNewsList(c *gin.Context) {
	language := validLanguage(c.DefaultQuery("language", "en"))
	featured := c.Query("featured")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	q := db.Mysql().Model(&entity.News{}).
		Where("status = ? AND publish_at <= ?", 1, time.Now())
	if featured == "1" {
		q = q.Where("is_featured = ?", 1)
	}
	var rows []entity.News
	if err := q.Order("sort ASC, publish_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": false, "code": "NEWS_LIST_FAILED", "message": err.Error()})
		return
	}
	ids := make([]uint64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	var txs []entity.NewsTranslation
	if len(ids) > 0 {
		db.Mysql().Select("news_id,language,title,summary,author,author_role").
			Where("news_id IN ?", ids).Find(&txs)
	}
	txMap := make(map[uint64][]entity.NewsTranslation, len(ids))
	for _, t := range txs {
		txMap[t.NewsID] = append(txMap[t.NewsID], t)
	}
	type item struct {
		ID         uint64    `json:"id"`
		Slug       string    `json:"slug"`
		Category   string    `json:"category"`
		CoverURL   string    `json:"cover_url"`
		SourceURL  string    `json:"source_url"`
		SourceName string    `json:"source_name"`
		IsFeatured int       `json:"is_featured"`
		PublishAt  time.Time `json:"publish_at"`
		Title      string    `json:"title"`
		Summary    string    `json:"summary"`
		Author     string    `json:"author"`
		AuthorRole string    `json:"author_role"`
	}
	items := make([]item, 0, len(rows))
	for _, r := range rows {
		t := pickNewsTranslation(txMap[r.ID], language)
		items = append(items, item{
			ID: r.ID, Slug: r.Slug, Category: r.Category,
			CoverURL: r.CoverURL, SourceURL: r.SourceURL, SourceName: r.SourceName,
			IsFeatured: r.IsFeatured, PublishAt: r.PublishAt,
			Title: t.Title, Summary: t.Summary, Author: t.Author, AuthorRole: t.AuthorRole,
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": items}})
}

type PublicNewsDetailResponse struct {
	ID         uint64    `json:"id"`
	Slug       string    `json:"slug"`
	Category   string    `json:"category"`
	CoverURL   string    `json:"cover_url"`
	SourceURL  string    `json:"source_url"`
	SourceName string    `json:"source_name"`
	PublishAt  time.Time `json:"publish_at"`
	Language   string    `json:"language"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Content    string    `json:"content"`
	Author     string    `json:"author"`
	AuthorRole string    `json:"author_role"`
}

func pickNewsTranslation(rows []entity.NewsTranslation, language string) entity.NewsTranslation {
	language = validLanguage(language)
	var english entity.NewsTranslation
	for _, row := range rows {
		if row.Language == language {
			return row
		}
		if row.Language == "en" {
			english = row
		}
	}
	if english.ID != 0 {
		return english
	}
	if len(rows) > 0 {
		return rows[0]
	}
	return entity.NewsTranslation{Language: language}
}

func findPublicNewsTranslation(newsID uint64, language string) entity.NewsTranslation {
	var rows []entity.NewsTranslation
	db.Mysql().Where("news_id = ?", newsID).Find(&rows)
	return pickNewsTranslation(rows, language)
}

func buildPublicNewsDetail(news entity.News, language string) PublicNewsDetailResponse {
	translation := findPublicNewsTranslation(news.ID, language)
	return PublicNewsDetailResponse{
		ID: news.ID, Slug: news.Slug, Category: news.Category,
		CoverURL: news.CoverURL, SourceURL: news.SourceURL, SourceName: news.SourceName,
		PublishAt: news.PublishAt, Language: translation.Language,
		Title: translation.Title, Summary: translation.Summary, Content: translation.Content,
		Author: translation.Author, AuthorRole: translation.AuthorRole,
	}
}

// PublicNewsDetail GET /client/news/v1/:id?language=en
func PublicNewsDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "NEWS_BAD_ID", "message": "invalid id"})
		return
	}
	var news entity.News
	if err := db.Mysql().Where("id = ? AND status = 1 AND publish_at <= ?", id, time.Now()).First(&news).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "NEWS_NOT_FOUND", "message": "not found"})
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"status": true, "data": buildPublicNewsDetail(news, c.DefaultQuery("language", "en"))})
}

type importantNewsAlert struct {
	PublicNewsDetailResponse
	Priority     int      `json:"priority"`
	Frequency    string   `json:"frequency"`
	Pages        []string `json:"pages"`
	DelaySeconds int      `json:"delay_seconds"`
	Revision     int      `json:"revision"`
	TargetURL    string   `json:"target_url"`
}

// PublicImportantNewsAlert GET /client/news/v1/important-alert?language=en
// Returns only the highest-priority currently active spotlight.
func PublicImportantNewsAlert(c *gin.Context) {
	now := time.Now()
	var news entity.News
	err := db.Mysql().
		Where("status = 1 AND is_important = 1").
		Where("publish_at <= ?", now).
		Where("(alert_start_at IS NULL OR alert_start_at <= ?)", now).
		Where("(alert_end_at IS NULL OR alert_end_at >= ?)", now).
		Order("alert_priority DESC, publish_at DESC, id DESC").
		Limit(1).Take(&news).Error
	if err != nil {
		c.Header("Cache-Control", "public, max-age=60")
		c.JSON(http.StatusOK, gin.H{"status": true, "data": nil})
		return
	}
	detail := buildPublicNewsDetail(news, c.DefaultQuery("language", "en"))
	targetURL := news.SourceURL
	if strings.TrimSpace(detail.Content) != "" || targetURL == "" {
		targetURL = "/news/" + strconv.FormatUint(news.ID, 10)
	}
	pages := strings.Split(normalizeAlertPages(news.AlertPages), ",")
	alert := importantNewsAlert{
		PublicNewsDetailResponse: detail,
		Priority:                 news.AlertPriority,
		Frequency:                normalizeAlertFrequency(news.AlertFrequency),
		Pages:                    pages,
		DelaySeconds:             clampAlertDelay(news.AlertDelaySeconds),
		Revision:                 normalizeAlertRevision(news.AlertRevision),
		TargetURL:                targetURL,
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"status": true, "data": alert})
}
