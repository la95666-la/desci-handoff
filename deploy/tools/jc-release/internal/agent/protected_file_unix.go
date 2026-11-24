//go:build !windows

package agent

import (
	"errors"
	"io"
	"os"
	"runtime"
	"syscall"
)

// Validate the opened inode, not only the path checked by Lstat. An attacker
// swapping the last path component to a symlink must not redirect the read.
func readProtectedJobFile(path string, max int64) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("unsafe evidence inode")
	}
	if runtime.GOOS == "linux" {
		if err := requireRootOwnedReadiness(info); err != nil {
			return nil, err
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(data)) > max {
		return nil, errors.New("evidence read failed or oversized")
	}
	return data, nil
}
