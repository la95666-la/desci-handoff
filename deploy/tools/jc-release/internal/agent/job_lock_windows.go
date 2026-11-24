//go:build windows

package agent

import (
	"errors"
	"os"
	"path/filepath"
)

// Windows test-only lock: a crash leaves the marker and blocks new Job work.
// Production Job admission requires a Linux root Agent.
func lockJobExecution(dataDir string) (func(), error) {
	if dataDir == "" {
		return nil, errors.New("missing data dir")
	}
	f, err := os.OpenFile(filepath.Join(dataDir, ".job-execution.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return func() { _ = f.Close(); _ = os.Remove(f.Name()) }, nil
}
