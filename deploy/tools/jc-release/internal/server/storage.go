package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

const (
	manifestFile = "manifest.json"
	agentsFile   = "agents.json"
	commandsFile = "commands.json"
	commandsCap  = 500
	// JSON escaping can expand one logical byte to six encoded bytes. This permits
	// the maximum admitted fields plus a small envelope while still bounding reads.
	maxAgentReportBodyBytes int64 = 6*(shared.MaxCommandLogBytes+shared.MaxCommandErrorBytes) + 4096
	maxHeartbeatBodyBytes   int64 = 4096
	maxServiceLogBodyBytes  int64 = 2048
)

// manifestData is what we persist to manifest.json.
type manifestData struct {
	Releases []shared.Release `json:"releases"`
}

type agentsData struct {
	Agents       []shared.Agent       `json:"agents"`
	EnrollTokens []shared.EnrollToken `json:"enroll_tokens"`
}

type commandsData struct {
	Commands []shared.Command `json:"commands"`
}

// Storage is a JSON-file backed store with per-file locks.
type Storage struct {
	Root string

	muManifest sync.RWMutex
	muArtifact sync.Mutex
	muAgents   sync.RWMutex
	muCommands sync.RWMutex
}

// NewStorage prepares data_dir subdirectories and returns a Storage.
func NewStorage(root string) (*Storage, error) {
	for _, sub := range []string{"", "artifacts", "logs"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, err
		}
	}
	s := &Storage{Root: root}
	if err := s.initFileIfMissing(manifestFile, manifestData{Releases: []shared.Release{}}); err != nil {
		return nil, err
	}
	if err := s.initFileIfMissing(agentsFile, agentsData{Agents: []shared.Agent{}, EnrollTokens: []shared.EnrollToken{}}); err != nil {
		return nil, err
	}
	if err := s.initFileIfMissing(commandsFile, commandsData{Commands: []shared.Command{}}); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) initFileIfMissing(name string, v interface{}) error {
	p := filepath.Join(s.Root, name)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return writeJSONAtomic(p, v)
}

func writeJSONAtomic(path string, v interface{}) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ---------- Releases ----------

// ListReleases returns releases filtered by component (empty for all), newest first.
func (s *Storage) ListReleases(comp shared.Component, limit int) ([]shared.Release, error) {
	s.muManifest.RLock()
	defer s.muManifest.RUnlock()
	var m manifestData
	if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
		return nil, err
	}
	out := make([]shared.Release, 0, len(m.Releases))
	for _, r := range m.Releases {
		if comp != "" && r.Component != comp {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UploadedAt.After(out[j].UploadedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetRelease returns the release with id, or an error if not found.
func (s *Storage) GetRelease(id string) (shared.Release, error) {
	s.muManifest.RLock()
	defer s.muManifest.RUnlock()
	var m manifestData
	if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
		return shared.Release{}, err
	}
	for _, r := range m.Releases {
		if r.ID == id {
			return r, nil
		}
	}
	return shared.Release{}, errors.New("release not found")
}

// LatestPerComponent returns the newest release for every component (may be nil).
func (s *Storage) LatestPerComponent() (shared.Manifest, error) {
	s.muManifest.RLock()
	defer s.muManifest.RUnlock()
	var m manifestData
	if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
		return shared.Manifest{}, err
	}
	sorted := append([]shared.Release(nil), m.Releases...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].UploadedAt.After(sorted[j].UploadedAt) })
	var man shared.Manifest
	seen := map[shared.Component]bool{}
	for i := range sorted {
		r := sorted[i]
		if seen[r.Component] {
			continue
		}
		seen[r.Component] = true
		man.Set(r.Component, &r)
	}
	return man, nil
}

// ArtifactPath returns the on-disk path for a release's tarball.
func (s *Storage) ArtifactPath(releaseID string) string {
	return filepath.Join(s.Root, "artifacts", releaseID+".tar.gz")
}

// SaveArtifact writes a bounded, hashed archive before publishing its manifest.
func (s *Storage) SaveArtifact(id string, src io.Reader) (int64, string, error) {
	s.muArtifact.Lock()
	defer s.muArtifact.Unlock()
	if !validReleaseID(id) {
		return 0, "", errors.New("invalid_release_id")
	}
	dst := s.ArtifactPath(id)
	f, err := os.OpenFile(dst+".tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, "", err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(dst + ".tmp")
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(src, maxArtifactBytes+1))
	if err != nil {
		return 0, "", err
	}
	if n == 0 || n > maxArtifactBytes {
		return 0, "", errors.New("artifact_size_invalid")
	}
	if err = f.Sync(); err != nil {
		return 0, "", err
	}
	if err = f.Close(); err != nil {
		return 0, "", err
	}
	if err = os.Rename(dst+".tmp", dst); err != nil {
		return 0, "", err
	}
	ok = true
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

const maxArtifactBytes int64 = 1024 * 1024 * 1024

func validReleaseID(id string) bool {
	return len(id) > 0 && len(id) <= shared.MaxReleaseIDBytes && !strings.ContainsAny(id, "/\\.") && releaseIDChars(id)
}
func releaseIDChars(id string) bool {
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func (s *Storage) AddRelease(r shared.Release) error {
	if !shared.ValidComponent(r.Component) || !validReleaseID(r.ID) || r.Size <= 0 || r.Size > maxArtifactBytes || len(r.SHA256) != 64 || len(r.Version) == 0 || len(r.Version) > 128 {
		return errors.New("invalid_release")
	}
	if r.Component == shared.CompJob {
		if len(r.JobAdmission) == 0 || len(r.JobAdmission) > 64*1024 || !json.Valid(r.JobAdmission) {
			return errors.New("job_admission_invalid")
		}
	} else if len(r.JobAdmission) != 0 {
		return errors.New("unexpected_job_admission")
	}
	s.muArtifact.Lock()
	defer s.muArtifact.Unlock()
	f, err := os.Open(s.ArtifactPath(r.ID))
	if err != nil {
		return err
	}
	h := sha256.New()
	n, readErr := io.Copy(h, io.LimitReader(f, maxArtifactBytes+1))
	closeErr := f.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n != r.Size || subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h.Sum(nil))), []byte(r.SHA256)) != 1 {
		return errors.New("release_artifact_mismatch")
	}
	s.muManifest.Lock()
	defer s.muManifest.Unlock()
	var m manifestData
	if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
		return err
	}
	for _, existing := range m.Releases {
		if existing.ID == r.ID {
			return errors.New("duplicate_release")
		}
	}
	m.Releases = append(m.Releases, r)
	return writeJSONAtomic(filepath.Join(s.Root, manifestFile), m)
}

// addReleaseFixture seeds historical release identity for same-package tests only.
func (s *Storage) addReleaseFixture(r shared.Release) error {
	s.muManifest.Lock()
	defer s.muManifest.Unlock()
	var m manifestData
	if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
		return err
	}
	m.Releases = append(m.Releases, r)
	return writeJSONAtomic(filepath.Join(s.Root, manifestFile), m)
}

// DeleteRelease removes the release entry and its artifact file.
func (s *Storage) DeleteRelease(id string) error {
	if !validReleaseID(id) {
		return errors.New("invalid_release_id")
	}
	s.muManifest.Lock()
	defer s.muManifest.Unlock()
	var m manifestData
	if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
		return err
	}
	out := m.Releases[:0]
	found := false
	for _, r := range m.Releases {
		if r.ID == id {
			found = true
			continue
		}
		out = append(out, r)
	}
	if !found {
		return errors.New("release not found")
	}
	m.Releases = out
	if err := writeJSONAtomic(filepath.Join(s.Root, manifestFile), m); err != nil {
		return err
	}
	_ = os.Remove(s.ArtifactPath(id))
	return nil
}

// ---------- Agents ----------

// ListAgents returns all registered agents.
func (s *Storage) ListAgents() ([]shared.Agent, error) {
	s.muAgents.RLock()
	defer s.muAgents.RUnlock()
	var a agentsData
	if err := readJSON(filepath.Join(s.Root, agentsFile), &a); err != nil {
		return nil, err
	}
	return a.Agents, nil
}

// GetAgent returns the agent with id.
func (s *Storage) GetAgent(id string) (shared.Agent, error) {
	agents, err := s.ListAgents()
	if err != nil {
		return shared.Agent{}, err
	}
	for _, a := range agents {
		if a.ID == id {
			return a, nil
		}
	}
	return shared.Agent{}, errors.New("agent not found")
}

// GetAgentByToken returns the agent with the given permanent token.
func (s *Storage) GetAgentByToken(token string) (shared.Agent, error) {
	agents, err := s.ListAgents()
	if err != nil {
		return shared.Agent{}, err
	}
	for _, a := range agents {
		if a.Token == token {
			return a, nil
		}
	}
	return shared.Agent{}, errors.New("invalid agent token")
}

func validateAgentControlPlane(ag *shared.Agent) error {
	return shared.ValidateAgentRegistration(ag)
}

// UpdateAgent applies mutator inside the write lock, repeats control-plane
// validation on the complete candidate, and only then persists.
func (s *Storage) UpdateAgent(id string, mutator func(*shared.Agent) error) error {
	s.muAgents.Lock()
	defer s.muAgents.Unlock()
	var a agentsData
	p := filepath.Join(s.Root, agentsFile)
	if err := readJSON(p, &a); err != nil {
		return err
	}
	for i := range a.Agents {
		if a.Agents[i].ID == id {
			original := a.Agents[i]
			candidate := original
			if err := mutator(&candidate); err != nil {
				return err
			}
			if err := validateAgentControlPlane(&candidate); err != nil {
				return err
			}
			a.Agents[i] = candidate
			if err := writeJSONAtomic(p, a); err != nil {
				a.Agents[i] = original
				return err
			}
			return nil
		}
	}
	return errors.New("agent not found")
}

// AddAgent appends a new agent record.
func (s *Storage) AddAgent(ag shared.Agent) error {
	s.muAgents.Lock()
	defer s.muAgents.Unlock()
	if err := shared.ValidateAgentRegistration(&ag); err != nil {
		return err
	}
	var a agentsData
	p := filepath.Join(s.Root, agentsFile)
	if err := readJSON(p, &a); err != nil {
		return err
	}
	a.Agents = append(a.Agents, ag)
	return writeJSONAtomic(p, a)
}

// DeleteAgent removes an agent by id.
func (s *Storage) DeleteAgent(id string) error {
	s.muAgents.Lock()
	defer s.muAgents.Unlock()
	var a agentsData
	p := filepath.Join(s.Root, agentsFile)
	if err := readJSON(p, &a); err != nil {
		return err
	}
	out := a.Agents[:0]
	found := false
	for _, x := range a.Agents {
		if x.ID == id {
			found = true
			continue
		}
		out = append(out, x)
	}
	if !found {
		return errors.New("agent not found")
	}
	a.Agents = out
	return writeJSONAtomic(p, a)
}

// AddEnrollToken persists a new enroll token.
func (s *Storage) AddEnrollToken(t shared.EnrollToken) error {
	s.muAgents.Lock()
	defer s.muAgents.Unlock()
	var a agentsData
	p := filepath.Join(s.Root, agentsFile)
	if err := readJSON(p, &a); err != nil {
		return err
	}
	a.EnrollTokens = append(a.EnrollTokens, t)
	return writeJSONAtomic(p, a)
}

// RegisterAgent atomically validates registration data, consumes the one-time
// enrollment token and appends the agent. No token or agent state changes if any
// validation or persistence step fails.
func (s *Storage) RegisterAgent(enrollToken string, agent shared.Agent) error {
	s.muAgents.Lock()
	defer s.muAgents.Unlock()
	if err := shared.ValidateEnrollToken(enrollToken); err != nil {
		return err
	}
	if err := shared.ValidateAgentRegistration(&agent); err != nil {
		return err
	}
	var data agentsData
	p := filepath.Join(s.Root, agentsFile)
	if err := readJSON(p, &data); err != nil {
		return err
	}
	now := time.Now().UTC()
	found := false
	for i := range data.EnrollTokens {
		token := &data.EnrollTokens[i]
		if token.Token != enrollToken {
			continue
		}
		if token.UsedAt != nil {
			return errors.New("enroll token already used")
		}
		if now.After(token.ExpiresAt) {
			return errors.New("enroll token expired")
		}
		token.UsedAt = &now
		found = true
		break
	}
	if !found {
		return errors.New("unknown enroll token")
	}
	data.Agents = append(data.Agents, agent)
	return writeJSONAtomic(p, data)
}

// ---------- Commands ----------

// ListCommands returns commands optionally filtered by agent, newest first.
func (s *Storage) ListCommands(agentID string, limit int) ([]shared.Command, error) {
	s.muCommands.RLock()
	defer s.muCommands.RUnlock()
	var c commandsData
	if err := readJSON(filepath.Join(s.Root, commandsFile), &c); err != nil {
		return nil, err
	}
	out := make([]shared.Command, 0, len(c.Commands))
	for _, x := range c.Commands {
		if agentID != "" && x.AgentID != agentID {
			continue
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetCommand returns a single command by id.
func (s *Storage) GetCommand(id string) (shared.Command, error) {
	s.muCommands.RLock()
	defer s.muCommands.RUnlock()
	var c commandsData
	if err := readJSON(filepath.Join(s.Root, commandsFile), &c); err != nil {
		return shared.Command{}, err
	}
	for _, x := range c.Commands {
		if x.ID == id {
			return x, nil
		}
	}
	return shared.Command{}, errors.New("command not found")
}

func validateCommandFieldSizes(cmd *shared.Command) error {
	if len([]byte(cmd.Status)) > shared.MaxCommandStatusBytes {
		return errors.New("command_status_too_large")
	}
	if len([]byte(cmd.Log)) > shared.MaxCommandLogBytes {
		return errors.New("command_log_too_large")
	}
	if len([]byte(cmd.Error)) > shared.MaxCommandErrorBytes {
		return errors.New("command_error_too_large")
	}
	return nil
}

// FinalizeCommand atomically enforces the agent ownership, in-progress state,
// terminal report vocabulary, field bounds and one-time finalization contract.
func (s *Storage) FinalizeCommand(id, agentID, status, log, errMsg string, finished time.Time) error {
	if !shared.ValidTerminalReportStatus(status) {
		return errors.New("invalid_terminal_report_status")
	}
	s.muCommands.Lock()
	defer s.muCommands.Unlock()
	var c commandsData
	p := filepath.Join(s.Root, commandsFile)
	if err := readJSON(p, &c); err != nil {
		return err
	}
	for i := range c.Commands {
		if c.Commands[i].ID != id {
			continue
		}
		original := c.Commands[i]
		if original.AgentID != agentID {
			return errors.New("command_belongs_to_different_agent")
		}
		if original.Status != shared.StatusInProgress || original.FinishedAt != nil {
			return errors.New("command_not_in_progress")
		}
		candidate := original
		candidate.Status = status
		candidate.Log = log
		candidate.Error = errMsg
		candidate.FinishedAt = &finished
		if err := validateCommandFieldSizes(&candidate); err != nil {
			return err
		}
		c.Commands[i] = candidate
		return writeJSONAtomic(p, c)
	}
	return errors.New("command not found")
}

// AddCommand appends a new command. Rotates oldest entries to logs/ if over cap.
func (s *Storage) AddCommand(cmd shared.Command) error {
	s.muCommands.Lock()
	defer s.muCommands.Unlock()
	if cmd.Kind == shared.CmdLogs {
		if err := shared.ValidateServiceLogCommand(&cmd); err != nil {
			return err
		}
	} else {
		if !shared.ValidComponent(cmd.Component) || cmd.AgentID == "" || cmd.ID == "" || cmd.Status != shared.StatusPending {
			return errors.New("invalid_release_command")
		}
		var agents agentsData
		if err := readJSON(filepath.Join(s.Root, agentsFile), &agents); err != nil {
			return err
		}
		knownAgent := false
		for _, ag := range agents.Agents {
			if ag.ID == cmd.AgentID {
				knownAgent = true
				break
			}
		}
		if !knownAgent {
			return errors.New("unknown_agent")
		}
		if cmd.LogLines != 0 || cmd.LogSince != 0 {
			return errors.New("release_command_log_options_forbidden")
		}
		switch cmd.Kind {
		case shared.CmdUpgrade:
			if !validReleaseID(cmd.ReleaseID) {
				return errors.New("release_id_required")
			}
			var m manifestData
			if err := readJSON(filepath.Join(s.Root, manifestFile), &m); err != nil {
				return err
			}
			found := false
			for _, r := range m.Releases {
				if r.ID == cmd.ReleaseID && r.Component == cmd.Component {
					found = true
					break
				}
			}
			if !found {
				return errors.New("release_component_mismatch")
			}
		case shared.CmdRollback:
			return errors.New("rollback_target_unverified")
		case shared.CmdRestart:
			if cmd.ReleaseID != "" {
				return errors.New("restart_release_id_forbidden")
			}
		default:
			return errors.New("invalid_release_kind")
		}
	}
	if err := validateCommandFieldSizes(&cmd); err != nil {
		return err
	}
	var c commandsData
	p := filepath.Join(s.Root, commandsFile)
	if err := readJSON(p, &c); err != nil {
		return err
	}
	if cmd.Kind != shared.CmdLogs {
		for _, existing := range c.Commands {
			if existing.AgentID == cmd.AgentID && existing.Component == cmd.Component && (existing.Status == shared.StatusPending || existing.Status == shared.StatusInProgress) && existing.Kind != shared.CmdLogs {
				return errors.New("release_command_already_active")
			}
		}
	}
	c.Commands = append(c.Commands, cmd)
	if len(c.Commands) > commandsCap {
		// Rotate oldest half to logs
		excess := len(c.Commands) - commandsCap
		sort.Slice(c.Commands, func(i, j int) bool { return c.Commands[i].CreatedAt.Before(c.Commands[j].CreatedAt) })
		toRotate := c.Commands[:excess]
		c.Commands = c.Commands[excess:]
		if err := s.rotateCommands(toRotate); err != nil {
			return err
		}
	}
	return writeJSONAtomic(p, c)
}

// UpdateCommand applies mutator to the command with id and persists.
func (s *Storage) UpdateCommand(id string, mutator func(*shared.Command) error) error {
	s.muCommands.Lock()
	defer s.muCommands.Unlock()
	var c commandsData
	p := filepath.Join(s.Root, commandsFile)
	if err := readJSON(p, &c); err != nil {
		return err
	}
	for i := range c.Commands {
		if c.Commands[i].ID == id {
			original := c.Commands[i]
			candidate := original
			if err := mutator(&candidate); err != nil {
				return err
			}
			if err := validateCommandFieldSizes(&candidate); err != nil {
				return err
			}
			c.Commands[i] = candidate
			if err := writeJSONAtomic(p, c); err != nil {
				c.Commands[i] = original
				return err
			}
			return nil
		}
	}
	return errors.New("command not found")
}

// ClaimPendingForAgent transitions all pending commands for agentID to in_progress and returns them.
func (s *Storage) ClaimPendingForAgent(agentID string) ([]shared.Command, error) {
	s.muCommands.Lock()
	defer s.muCommands.Unlock()
	var c commandsData
	p := filepath.Join(s.Root, commandsFile)
	if err := readJSON(p, &c); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var claimed []shared.Command
	for i := range c.Commands {
		if c.Commands[i].AgentID != agentID {
			continue
		}
		if c.Commands[i].Status != shared.StatusPending {
			continue
		}
		c.Commands[i].Status = shared.StatusInProgress
		started := now
		c.Commands[i].StartedAt = &started
		claimed = append(claimed, c.Commands[i])
	}
	if len(claimed) == 0 {
		return nil, nil
	}
	if err := writeJSONAtomic(p, c); err != nil {
		return nil, err
	}
	sort.Slice(claimed, func(i, j int) bool { return claimed[i].CreatedAt.Before(claimed[j].CreatedAt) })
	return claimed, nil
}

func (s *Storage) rotateCommands(cmds []shared.Command) error {
	if len(cmds) == 0 {
		return nil
	}
	now := time.Now().UTC()
	name := fmt.Sprintf("history-%04d%02d.jsonl", now.Year(), int(now.Month()))
	p := filepath.Join(s.Root, "logs", name)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, cmd := range cmds {
		entry := shared.LogEntry{Command: cmd, Rotated: now}
		if err := enc.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

// ReleaseInUse reports whether any agent has this release deployed.
func (s *Storage) ReleaseInUse(releaseID string) (bool, error) {
	// An accepted command may still be pending or executing even before the
	// agent reports this release as deployed. Do not delete its only archive.
	s.muCommands.RLock()
	var queued commandsData
	queueErr := readJSON(filepath.Join(s.Root, commandsFile), &queued)
	s.muCommands.RUnlock()
	if queueErr != nil {
		return false, queueErr
	}
	for _, cmd := range queued.Commands {
		if cmd.ReleaseID == releaseID && (cmd.Status == shared.StatusPending || cmd.Status == shared.StatusInProgress) {
			return true, nil
		}
	}
	agents, err := s.ListAgents()
	if err != nil {
		return false, err
	}
	for _, a := range agents {
		for _, id := range a.Deployed {
			if id == releaseID {
				return true, nil
			}
		}
	}
	return false, nil
}
