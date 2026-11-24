package agent

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/joincare/jc-release/internal/shared"
)

func BackupDir(dataDir string, component shared.Component) string {
	return filepath.Join(dataDir, "backups", string(component))
}

func EnsureBackupDir(dataDir string, component shared.Component) error {
	return os.MkdirAll(BackupDir(dataDir, component), 0o700)
}

func backupFilesNewestFirst(dataDir string, component shared.Component) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(BackupDir(dataDir, component))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	files := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			files = append(files, entry)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		left, _ := files[i].Info()
		right, _ := files[j].Info()
		return left.ModTime().After(right.ModTime())
	})
	return files, nil
}

func LatestBackup(dataDir string, component shared.Component) (string, error) {
	files, err := backupFilesNewestFirst(dataDir, component)
	if err != nil || len(files) == 0 {
		return "", err
	}
	return filepath.Join(BackupDir(dataDir, component), files[0].Name()), nil
}

func PruneOldBackups(dataDir string, component shared.Component, keep int) error {
	files, err := backupFilesNewestFirst(dataDir, component)
	if err != nil {
		return err
	}
	if keep < 0 {
		keep = 0
	}
	if len(files) <= keep {
		return nil
	}
	for _, entry := range files[keep:] {
		_ = os.Remove(filepath.Join(BackupDir(dataDir, component), entry.Name()))
	}
	return nil
}
