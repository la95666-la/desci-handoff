package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

// State is the persistent agent identity + deployed versions.
type State struct {
	AgentID       string                             `json:"agent_id"`
	AgentToken    string                             `json:"agent_token"`
	Deployed      map[shared.Component]DeployedEntry `json:"deployed"` // component → last successful upgrade info
	JobGeneration uint64                             `json:"job_generation,omitempty"`
	// JobPending is written before any Job container mutation. After a crash or
	// an unverified rollback, admission stays closed until operator recovery.
	JobPending  *JobIntent          `json:"job_pending,omitempty"`
	JobRecovery *JobRecoveryJournal `json:"job_recovery,omitempty"`
}

type JobRecoveryJournal struct {
	CommandID        string              `json:"command_id"`
	PreviousIntent   JobIntent           `json:"previous_intent"`
	TargetRelease    string              `json:"target_release"`
	BackupDir        string              `json:"backup_dir"`
	Result           string              `json:"result"`
	PreviousRecovery *JobRecoveryJournal `json:"previous_recovery,omitempty"`
}

type JobIntent struct {
	ReleaseID  string `json:"release_id"`
	Generation uint64 `json:"generation"`
}

// DeployedEntry records what release is currently deployed for a component.
type DeployedEntry struct {
	ReleaseID  string           `json:"release_id"`
	Version    string           `json:"version"`
	Component  shared.Component `json:"component"`
	AppliedAt  time.Time        `json:"applied_at"`
	BackupPath string           `json:"backup_path"` // path to backup made just before this upgrade
}

// Store persists State to state.json under DataDir.
type Store struct {
	Path string
	mu   sync.RWMutex
}

// NewStore returns a Store; DataDir must exist.
func NewStore(dataDir string) *Store {
	return &Store{Path: filepath.Join(dataDir, "state.json")}
}

// Load returns the current State (or empty if state.json doesn't exist).
func (s *Store) Load() (*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{Deployed: map[shared.Component]DeployedEntry{}}, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(f, &st); err != nil {
		return nil, err
	}
	if st.Deployed == nil {
		st.Deployed = map[shared.Component]DeployedEntry{}
	}
	return &st, nil
}

// Save persists State atomically.
func (s *Store) Save(st *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.Deployed == nil {
		st.Deployed = map[shared.Component]DeployedEntry{}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := durableStateWrite(tmp, data); err != nil {
		return err
	}
	return durableStateRename(tmp, s.Path)
}

// Update loads, mutates, saves atomically under lock.
func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st State
	if data, err := os.ReadFile(s.Path); err == nil {
		if err := json.Unmarshal(data, &st); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if st.Deployed == nil {
		st.Deployed = map[shared.Component]DeployedEntry{}
	}
	if err := fn(&st); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := durableStateWrite(tmp, data); err != nil {
		return err
	}
	return durableStateRename(tmp, s.Path)
}

func durableStateWrite(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func durableStateRename(tmp, path string) error {
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// On POSIX, persist the directory entry as well as the file contents.
	if runtime.GOOS != "windows" {
		dir, err := os.Open(filepath.Dir(path))
		if err != nil {
			return err
		}
		defer dir.Close()
		return dir.Sync()
	}
	return nil
}
