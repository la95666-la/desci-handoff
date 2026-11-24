//go:build !windows

package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProtectedJobFileRejectsSymlinkAndUnsafeInode(t *testing.T) {
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		t.Skip("positive fixture requires root-owned inode")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "evidence")
	if err := os.WriteFile(file, []byte("verified"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, err := readProtectedJobFile(file, 8); err != nil || string(body) != "verified" {
		t.Fatalf("safe inode rejected: %q %v", body, err)
	}
	if _, err := readProtectedJobFile(file, 7); err == nil {
		t.Fatal("oversized receipt admitted")
	}
	link := filepath.Join(dir, "swapped")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readProtectedJobFile(link, 8); err == nil {
		t.Fatal("symlink redirected evidence read")
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProtectedJobFile(file, 8); err == nil {
		t.Fatal("world-readable evidence admitted")
	}
}
