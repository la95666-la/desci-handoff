//go:build !windows

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJobExecutionLockRejectsConcurrentProcessAndUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockJobExecution(dir)
	if err != nil {
		t.Fatal(err)
	}
	if next, err := lockJobExecution(dir); err == nil {
		next()
		t.Fatal("concurrent lock acquired")
	}
	unlock()
	unlock, err = lockJobExecution(dir)
	if err != nil {
		t.Fatalf("unlock did not release: %v", err)
	}
	unlock()
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if next, err := lockJobExecution(dir); err == nil {
		next()
		t.Fatal("world-writable lock directory admitted")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, ".job-execution.lock")
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "target"), lock); err != nil {
		t.Fatal(err)
	}
	if next, err := lockJobExecution(dir); err == nil {
		next()
		t.Fatal("symlink lock admitted")
	}
}
