//go:build windows

package agent

import "os"

// Windows test fixtures have no POSIX uid; production admission requires Linux.
func requireRootOwnedReadiness(os.FileInfo) error { return nil }

func requireRootAgent() error { return nil }

func requireTrustedReadinessParents(string) error { return nil }

func verifySSHHostKeyFile(string, string) error { return nil }
