//go:build windows

package chainevent

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func readFairLaunchRuntimeFile(path string, maximum int64, secret bool) ([]byte, error) {
	if path == "" || maximum <= 0 {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maximum || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	// Windows ACLs are not faithfully represented by os.FileMode. Production
	// admission runs in the Linux container, where secure_file_unix.go enforces
	// owner-only mode 0600, uid ownership and O_NOFOLLOW atomically.
	file, err := os.Open(abs)
	if err != nil {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(value)) > maximum {
		return nil, errors.New(FairLaunchMainnetRuntimeUnavailable)
	}
	return value, nil
}
