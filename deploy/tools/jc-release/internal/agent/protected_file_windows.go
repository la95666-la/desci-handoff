//go:build windows

package agent

import (
	"errors"
	"io"
	"os"
)

// Windows fixtures do not model Linux inode ownership or O_NOFOLLOW.
func readProtectedJobFile(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("unsafe evidence inode")
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(data)) > max {
		return nil, errors.New("evidence read failed or oversized")
	}
	return data, nil
}
