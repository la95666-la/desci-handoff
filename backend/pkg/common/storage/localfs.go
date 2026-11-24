package storage

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalFs 把文件存本地磁盘 rootDir 下，publicBase 是 nginx serve 该目录时的 URL 前缀。
// 例：LocalFs{RootDir: "/app/uploads", PublicBase: "/uploads"}
type LocalFs struct {
	RootDir    string
	PublicBase string
}

// Save 生成 uuid + ext，写 <RootDir>/YYYY-MM/<uuid>.<ext>，返回 <PublicBase>/YYYY-MM/<uuid>.<ext>
func (l *LocalFs) Save(originalName, mimeType string, reader io.Reader) (string, string, error) {
	if _, ok := AllowedMimes[mimeType]; !ok {
		return "", "", ErrUnsupportedMime
	}

	// 内容嗅探（magic bytes）：不信任客户端声明的 Content-Type，
	// 防可执行/脚本文件伪装成白名单类型上传后被同域 serve
	head := make([]byte, 512)
	nRead, _ := io.ReadFull(reader, head)
	head = head[:nRead]
	reader = io.MultiReader(bytes.NewReader(head), reader)
	if !sniffAllowed(http.DetectContentType(head), mimeType) {
		return "", "", ErrUnsupportedMime
	}

	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		if guesses, _ := mime.ExtensionsByType(mimeType); len(guesses) > 0 {
			ext = guesses[0]
		}
	}
	// 白名单扩展名，防路径穿透 / 二进制被当成 html serve
	if !isAllowedExt(ext) {
		return "", "", ErrUnsupportedMime
	}

	dateDir := time.Now().Format("2006-01")
	if err := os.MkdirAll(filepath.Join(l.RootDir, dateDir), 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir: %w", err)
	}

	uid := randomHex(16)
	name := uid + ext
	relPath := filepath.Join(dateDir, name)
	absPath := filepath.Join(l.RootDir, relPath)

	f, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	limited := &io.LimitedReader{R: reader, N: MaxUploadBytes + 1}
	n, err := io.Copy(f, limited)
	if err != nil {
		_ = os.Remove(absPath)
		return "", "", err
	}
	if n > MaxUploadBytes {
		_ = os.Remove(absPath)
		return "", "", ErrUploadTooBig
	}

	// publicBase 若已带尾斜杠去掉再拼
	pub := strings.TrimRight(l.PublicBase, "/") + "/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
	return absPath, pub, nil
}

func (l *LocalFs) Delete(storagePath string) error {
	// 只删 RootDir 下的（防越权删）
	abs, err := filepath.Abs(storagePath)
	if err != nil {
		return err
	}
	rootAbs, _ := filepath.Abs(l.RootDir)
	if !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) {
		return fmt.Errorf("path outside root")
	}
	err = os.Remove(abs)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var allowedExts = map[string]bool{
	".pdf": true, ".png": true, ".jpg": true, ".jpeg": true,
	".webp": true, ".gif": true, ".zip": true,
	".json": true, ".md": true, ".txt": true,
}

func isAllowedExt(ext string) bool {
	return allowedExts[strings.ToLower(ext)]
}

// sniffAllowed 判定嗅探到的真实类型是否符合白名单。
// DetectContentType 对文本类（md/txt/json/svg）统一报 text/plain，
// 此类回退到客户端声明类型（仍受 AllowedMimes 白名单约束）；
// 二进制类型（图/pdf/zip）以嗅探结果为准，声明类型不再可信。
func sniffAllowed(sniffed, declared string) bool {
	base := strings.TrimSpace(strings.SplitN(sniffed, ";", 2)[0])
	switch base {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "application/pdf", "application/zip":
		return AllowedMimes[base]
	case "text/plain":
		return AllowedMimes[declared]
	}
	return false
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
