//go:build !windows

package chainevent

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func readFairLaunchRuntimeFile(path string, maximum int64, secret bool) ([]byte, error) {
	if path == "" || maximum <= 0 {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil || resolved != abs {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	fd, err := unix.Open(abs, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	file := os.NewFile(uintptr(fd), filepath.Base(abs))
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Size < 0 || stat.Size > maximum || stat.Uid != uint32(os.Geteuid()) {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	permissions := os.FileMode(stat.Mode).Perm()
	if secret {
		if permissions != 0o600 {
			return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
		}
	} else if permissions&0o022 != 0 || permissions&0o400 == 0 {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	value, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(value)) > maximum {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	return value, nil
}
