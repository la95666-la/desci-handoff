package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// ListCopywriting GET /admin/v1/copywriting?page=1&page_size=20&language=en&keyword=
func ListCopywriting(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 500 {
		pageSize = 20
	}
	language := c.Query("language")
	keyword := c.Query("keyword")

	q := db.Mysql().Model(&entity.CopywritingConfig{})
	if language != "" {
		q = q.Where("language = ?", language)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("`key` LIKE ? OR `value` LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_LIST_FAILED", "message": err.Error()})
		return
	}

	var rows []entity.CopywritingConfig
	if err := q.Order("`key` ASC, language ASC").
		Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_LIST_FAILED", "message": err.Error()})
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

type copywritingUpsertReq struct {
	Key      string `json:"key" binding:"required"`
	Value    string `json:"value"`
	Language string `json:"language" binding:"required"`
}

// CreateCopywriting POST /admin/v1/copywriting
func CreateCopywriting(c *gin.Context) {
	var req copywritingUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_REQUEST", "message": err.Error()})
		return
	}
	row := entity.CopywritingConfig{Key: req.Key, Value: req.Value, Language: req.Language}
	if err := db.Mysql().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_CREATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// UpdateCopywriting PUT /admin/v1/copywriting/:id
func UpdateCopywriting(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_ID", "message": "invalid id"})
		return
	}
	var req copywritingUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_REQUEST", "message": err.Error()})
		return
	}
	res := db.Mysql().Model(&entity.CopywritingConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"key":      req.Key,
		"value":    req.Value,
		"language": req.Language,
	})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_UPDATE_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_CW_NOT_FOUND", "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// DeleteCopywriting DELETE /admin/v1/copywriting/:id
func DeleteCopywriting(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_ID", "message": "invalid id"})
		return
	}
	if err := db.Mysql().Where("id = ?", id).Delete(&entity.CopywritingConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// PublicCopywriting GET /admin/v1/copywriting/public?language=en  (用于 C 端读取多语言文案；无需 token)
//  注：实际 C 端应走 /client/* 路径，这里临时挂 admin 下方便联调；后续在 /client/copywriting/v1/list 提供同源接口
func PublicCopywriting(c *gin.Context) {
	language := c.DefaultQuery("language", "en")
	var rows []entity.CopywritingConfig
	if err := db.Mysql().Where("language = ?", language).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CW_PUBLIC_FAILED", "message": err.Error()})
		return
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": out})
}

// ============ 双语聚合视图（每个 key 一行，含 en + tw 两个 value）============

type bilingualRow struct {
	Key       string `json:"key"`
	EN        string `json:"en"`
	TW        string `json:"tw"`
	UpdatedAt string `json:"updated_at"`
}

// ListCopywritingBilingual GET /admin/v1/copywriting/bilingual?page=1&page_size=20&keyword=&prefix=home
//   按 key 聚合，每行同时返回 en + tw value；不存在的语言为空串。
//   prefix 用于按页面过滤（如 home / navbar / taskhall），匹配 "<prefix>." 开头的 key。
func ListCopywritingBilingual(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}
	keyword := c.Query("keyword")
	prefix := strings.TrimSpace(c.Query("prefix"))

	// 1) 拿到所有匹配的 distinct keys
	keysQ := db.Mysql().Model(&entity.CopywritingConfig{}).
		Select("DISTINCT `key`")
	if prefix != "" {
		keysQ = keysQ.Where("`key` LIKE ?", prefix+".%")
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		keysQ = keysQ.Where("`key` LIKE ? OR `value` LIKE ?", like, like)
	}

	var allKeys []string
	if err := keysQ.Order("`key` ASC").Pluck("key", &allKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_BI_FAILED", "message": err.Error()})
		return
	}
	total := int64(len(allKeys))

	// 2) 分页
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(allKeys) {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
			"items": []bilingualRow{}, "total": total, "page": page, "page_size": pageSize,
		}})
		return
	}
	if end > len(allKeys) {
		end = len(allKeys)
	}
	pageKeys := allKeys[start:end]

	// 3) 拉本页对应的所有行（en + tw）
	var rows []entity.CopywritingConfig
	if err := db.Mysql().
		Where("`key` IN ? AND language IN ?", pageKeys, []string{"en", "tw"}).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_BI_FAILED", "message": err.Error()})
		return
	}

	// 4) 按 key 聚合成 bilingualRow
	merged := make(map[string]*bilingualRow, len(pageKeys))
	for _, k := range pageKeys {
		merged[k] = &bilingualRow{Key: k}
	}
	for _, r := range rows {
		row := merged[r.Key]
		if row == nil {
			continue
		}
		switch r.Language {
		case "en":
			row.EN = r.Value
		case "tw":
			row.TW = r.Value
		}
		if ts := r.UpdatedAt.Format("2006-01-02 15:04:05"); ts > row.UpdatedAt {
			row.UpdatedAt = ts
		}
	}

	// 5) 按 keys 顺序输出
	items := make([]bilingualRow, 0, len(pageKeys))
	for _, k := range pageKeys {
		items = append(items, *merged[k])
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": items, "total": total, "page": page, "page_size": pageSize,
	}})
}

type bilingualUpsertReq struct {
	Key string `json:"key" binding:"required"`
	EN  string `json:"en"`
	TW  string `json:"tw"`
}

// UpsertCopywritingBilingual POST /admin/v1/copywriting/bilingual
//   按 key 同时 upsert en + tw 两行。空值也写入（允许显式清空）。
func UpsertCopywritingBilingual(c *gin.Context) {
	var req bilingualUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_REQUEST", "message": err.Error()})
		return
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_REQUEST", "message": "key required"})
		return
	}

	type pair struct {
		lang string
		val  string
	}
	upserts := []pair{{"en", req.EN}, {"tw", req.TW}}

	for _, p := range upserts {
		var existing entity.CopywritingConfig
		err := db.Mysql().Where("`key` = ? AND language = ?", key, p.lang).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Mysql().Create(&entity.CopywritingConfig{
				Key: key, Language: p.lang, Value: p.val,
			}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_UPSERT_FAILED", "message": err.Error()})
				return
			}
			continue
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_UPSERT_FAILED", "message": err.Error()})
			return
		}
		if err := db.Mysql().Model(&existing).Update("value", p.val).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_UPSERT_FAILED", "message": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"key": key}})
}

// ListCopywritingPrefixes GET /admin/v1/copywriting/prefixes
//   返回 distinct 页面前缀（key 中 "." 之前的部分），按字母排序。
//   admin UI 用作"页面"下拉的 options 源。
func ListCopywritingPrefixes(c *gin.Context) {
	var allKeys []string
	if err := db.Mysql().Model(&entity.CopywritingConfig{}).
		Select("DISTINCT `key`").
		Pluck("key", &allKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_PREFIXES_FAILED", "message": err.Error()})
		return
	}
	prefixSet := make(map[string]int, 16)
	for _, k := range allKeys {
		dot := strings.Index(k, ".")
		if dot < 0 {
			continue
		}
		prefixSet[k[:dot]]++
	}
	type prefixItem struct {
		Prefix string `json:"prefix"`
		Count  int    `json:"count"`
	}
	items := make([]prefixItem, 0, len(prefixSet))
	for p, n := range prefixSet {
		items = append(items, prefixItem{Prefix: p, Count: n})
	}
	// 按字母排序
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Prefix < items[i].Prefix {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": items}})
}

// DeleteCopywritingByKey DELETE /admin/v1/copywriting/by-key?key=xxx
//   按 key 删除所有语言的行
func DeleteCopywritingByKey(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_CW_BAD_REQUEST", "message": "key required"})
		return
	}
	if err := db.Mysql().Where("`key` = ?", key).Delete(&entity.CopywritingConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_CW_DELETE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"key": key}})
}
