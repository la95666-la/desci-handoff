// Package storage 提供文件上传抽象。
//
// Phase 1（Sprint A / C）：LocalFs 实现 —— 存 disk + nginx 静态 serve
// Phase 2 前：切 MinIO 实现（新增 minio.go，同 interface）—— code 不动只换 config
package storage

import (
	"errors"
	"io"
)

// Uploader 上传器接口。Save 返回 (storagePath, publicURL, error)。
//
//	storagePath: 用于将来 Delete 的定位标识（LocalFs 是绝对路径；MinIO 是 bucket 内 key）
//	publicURL:   前端可访问的完整 URL（LocalFs 是 /uploads/xxx，MinIO 是 CDN URL）
type Uploader interface {
	// Save 存原始字节流，返回 (storagePath, publicURL, err)。
	// originalName 用于保留扩展名 + 记 metadata；实际存名由实现自行生成（uuid + ext）。
	Save(originalName, mime string, reader io.Reader) (storagePath, publicURL string, err error)

	// Delete 移除文件；不存在也当作成功
	Delete(storagePath string) error
}

// ErrUploadTooBig 上传体积超限
var ErrUploadTooBig = errors.New("upload exceeds max size")

// ErrUnsupportedMime 类型不允许
var ErrUnsupportedMime = errors.New("unsupported mime type")

// AllowedMimes Phase 1 白名单：pdf + 常见图 + zip
var AllowedMimes = map[string]bool{
	"application/pdf":  true,
	"image/png":        true,
	"image/jpeg":       true,
	"image/webp":       true,
	"image/gif":        true,
	"application/zip":  true,
	"application/json": true,
	"text/markdown":    true,
	"text/plain":       true,
}

// MaxUploadBytes Phase 1 单文件上限 20 MB
const MaxUploadBytes int64 = 20 * 1024 * 1024
