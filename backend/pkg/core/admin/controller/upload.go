package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/storage"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// AdminUpload POST /admin/v1/upload  (multipart/form-data: file)
// 单文件上传，返回 { id, url, size, mime }。前端 TipTap 图片 / Documents PDF 用同一入口。
func AdminUpload(c *gin.Context) {
	if storage.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "UPLOAD_NOT_CONFIGURED", "message": "storage not initialized"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "UPLOAD_BAD_REQUEST", "message": err.Error()})
		return
	}
	if fh.Size > storage.MaxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"status": false, "code": "UPLOAD_TOO_BIG", "message": "file exceeds 20MB"})
		return
	}

	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "UPLOAD_OPEN_FAILED", "message": err.Error()})
		return
	}
	defer src.Close()

	mime := fh.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	storagePath, publicURL, err := storage.Default.Save(fh.Filename, mime, src)
	if err != nil {
		code := "UPLOAD_FAILED"
		if errors.Is(err, storage.ErrUnsupportedMime) {
			code = "UPLOAD_MIME_NOT_ALLOWED"
		} else if errors.Is(err, storage.ErrUploadTooBig) {
			code = "UPLOAD_TOO_BIG"
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": code, "message": err.Error()})
		return
	}

	uploaderID := uint64(0)
	if v, ok := c.Get(middleware.CtxAdminID); ok {
		if i, ok := v.(int64); ok {
			uploaderID = uint64(i)
		}
	}

	record := entity.UploadFile{
		UploaderAdminID: uploaderID,
		OriginalName:    fh.Filename,
		StoragePath:     storagePath,
		PublicURL:       publicURL,
		Mime:            mime,
		SizeBytes:       fh.Size,
	}
	if err := db.Mysql().Create(&record).Error; err != nil {
		// 已上传文件，DB 挂了 → 尝试回滚删文件；失败也无害
		_ = storage.Default.Delete(storagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "UPLOAD_DB_FAILED", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"id":   record.ID,
		"url":  publicURL,
		"size": fh.Size,
		"mime": mime,
		"name": fh.Filename,
	}})
}
