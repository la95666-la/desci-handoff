package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// ListBanners GET /admin/v1/banners?language=&page=&page_size=
func ListBanners(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	language := c.Query("language")

	q := db.Mysql().Model(&entity.HomeBanner{})
	if language != "" {
		q = q.Where("language = ?", language)
	}
	var total int64
	q.Count(&total)

	var rows []entity.HomeBanner
	if err := q.Order("language ASC, sort DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_BANNER_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

type bannerReq struct {
	Title         string  `json:"title"`
	Subtitle      string  `json:"subtitle"`
	CTALabel      string  `json:"cta_label"`
	CTALink       string  `json:"cta_link"`
	ImageURL      string  `json:"image_url"`
	HeroAnimation string  `json:"hero_animation"`
	Language      string  `json:"language"`
	StartAt       *string `json:"start_at"`
	EndAt         *string `json:"end_at"`
	Sort          int     `json:"sort"`
	IsEnable      int8    `json:"is_enable"`
}

func parseTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, *s); err == nil {
			return &t
		}
	}
	return nil
}

func validHeroAnimation(s string) string {
	switch s {
	case "dandelion", "dna", "none":
		return s
	default:
		return "dandelion"
	}
}

func validLanguage(s string) string {
	switch s {
	case "en", "tw", "zh":
		return s
	default:
		return "en"
	}
}

// CreateBanner POST /admin/v1/banners
func CreateBanner(c *gin.Context) {
	var req bannerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_BANNER_BAD_REQUEST", "message": err.Error()})
		return
	}
	row := entity.HomeBanner{
		Title:         req.Title,
		Subtitle:      req.Subtitle,
		CTALabel:      req.CTALabel,
		CTALink:       req.CTALink,
		ImageURL:      req.ImageURL,
		HeroAnimation: validHeroAnimation(req.HeroAnimation),
		Language:      validLanguage(req.Language),
		StartAt:       parseTime(req.StartAt),
		EndAt:         parseTime(req.EndAt),
		Sort:          req.Sort,
		IsEnable:      int8If(req.IsEnable, 1),
	}
	if err := db.Mysql().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_BANNER_CREATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// UpdateBanner PUT /admin/v1/banners/:id
func UpdateBanner(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_BANNER_BAD_ID", "message": "invalid id"})
		return
	}
	var req bannerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_BANNER_BAD_REQUEST", "message": err.Error()})
		return
	}
	res := db.Mysql().Model(&entity.HomeBanner{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":          req.Title,
		"subtitle":       req.Subtitle,
		"cta_label":      req.CTALabel,
		"cta_link":       req.CTALink,
		"image_url":      req.ImageURL,
		"hero_animation": validHeroAnimation(req.HeroAnimation),
		"language":       validLanguage(req.Language),
		"start_at":       parseTime(req.StartAt),
		"end_at":         parseTime(req.EndAt),
		"sort":           req.Sort,
		"is_enable":      int8If(req.IsEnable, 1),
	})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_BANNER_UPDATE_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_BANNER_NOT_FOUND", "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// DeleteBanner DELETE /admin/v1/banners/:id
func DeleteBanner(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_BANNER_BAD_ID", "message": "invalid id"})
		return
	}
	if err := db.Mysql().Where("id = ?", id).Delete(&entity.HomeBanner{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_BANNER_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// PublicCurrentBanner GET /client/banner/v1/current?language=en
// 返回当前生效（语种 + 排期 + 启用）中 sort 最高的一条 banner
func PublicCurrentBanner(c *gin.Context) {
	language := validLanguage(c.DefaultQuery("language", "en"))
	now := time.Now()

	var row entity.HomeBanner
	err := db.Mysql().
		Where("language = ? AND is_enable = 1", language).
		Where("(start_at IS NULL OR start_at <= ?)", now).
		Where("(end_at IS NULL OR end_at >= ?)", now).
		Order("sort DESC, id DESC").
		Limit(1).
		Take(&row).Error
	if err != nil {
		// 没匹配到不算错误，回 fallback 默认 banner
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
			"title":          "",
			"subtitle":       "",
			"cta_label":      "",
			"cta_link":       "",
			"image_url":      "",
			"hero_animation": "dandelion",
			"language":       language,
		}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}
