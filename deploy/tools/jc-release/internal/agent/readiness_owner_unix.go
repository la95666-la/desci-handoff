//go:build !windows

package agent

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func requireRootOwnedReadiness(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return errors.New("not root-owned")
	}
	return nil
}

func requireRootAgent() error {
	current, err := user.Current()
	if err != nil {
		return errors.New("job_readiness_owner_unavailable")
	}
	uid, err := strconv.Atoi(current.Uid)
	if err != nil || uid != 0 {
		return errors.New("job_readiness_requires_root_agent")
	}
	return nil
}

func requireTrustedReadinessParents(path string) error {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || requireRootOwnedReadiness(info) != nil {
			return errors.New("untrusted path ancestor")
		}
		if parent == string(os.PathSeparator) {
			return nil
		}
	}
}

// Match the actual SSH host public key, not just a value in agent config.
// Independent out-of-band confirmation of that fingerprint remains required.
func verifySSHHostKeyFile(path, expected string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("job_host_key_unavailable")
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 || fields[0] != "ssh-ed25519" {
		return errors.New("job_host_key_invalid")
	}
	key, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return errors.New("job_host_key_invalid")
	}
	digest := sha256.Sum256(key)
	if "SHA256:"+base64.RawStdEncoding.EncodeToString(digest[:]) != expected {
		return errors.New("job_host_key_mismatch")
	}
	return nil
}
