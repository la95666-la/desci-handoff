package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestLocalFsRejectsSVG(t *testing.T) {
	fs := &LocalFs{RootDir: t.TempDir(), PublicBase: "/uploads"}
	_, _, err := fs.Save("active.svg", "image/svg+xml", strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if !errors.Is(err, ErrUnsupportedMime) {
		t.Fatalf("expected ErrUnsupportedMime, got %v", err)
	}
}

func TestLocalFsAcceptsPNGMagicBytes(t *testing.T) {
	fs := &LocalFs{RootDir: t.TempDir(), PublicBase: "/uploads"}
	png := string([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0})
	path, publicURL, err := fs.Save("image.png", "image/png", strings.NewReader(png))
	if err != nil {
		t.Fatalf("save png: %v", err)
	}
	if path == "" || !strings.HasSuffix(publicURL, ".png") {
		t.Fatalf("unexpected upload result: path=%q url=%q", path, publicURL)
	}
}
