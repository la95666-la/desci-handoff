//go:build !windows

package agent

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// The kernel owns this lock, including after an Agent crash. Never delete the
// lock inode: a waiting process could otherwise lock an unlinked file.
func lockJobExecution(dataDir string) (func(), error) {
	if dataDir == "" {
		return nil, errors.New("missing data dir")
	}
	dir, err := os.Lstat(dataDir)
	if err != nil || !dir.IsDir() || dir.Mode().Perm()&0077 != 0 || !ownedByAgent(dir) {
		return nil, errors.New("unsafe data dir")
	}
	path := filepath.Join(dataDir, ".job-execution.lock")
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !ownedByAgent(info) {
		f.Close()
		return nil, errors.New("unsafe lock file")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}

func ownedByAgent(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
