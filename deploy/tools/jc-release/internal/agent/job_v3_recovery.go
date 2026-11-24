package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/joincare/jc-release/internal/shared"
	"github.com/joincare/jc-release/internal/strategies"
)

const v3RecoveryTarget = "c3d6fbb6-7a23-4fba-8433-cb7a0ede2255"
const v3RecoverySHA = "9c84d34939b28eddacdc42d374e4f100c34f50739e75d7b20c106d8f837c89a0"
const v3RecoveryOldSHA = "373d3103265227b88cb2210b875840ae114f25850b8c612c221d78aaea150231"
const v3RecoveryNewSHA = "b2d389a534d6187c278961a8e18d785a72ee313f2c2ba6f63c83032ece862a2b"
const v3RecoveryImage = "sha256:e05db0d0944eefda70dac93f02164e723ac8a408ebe3947192837512fc4b4673"
const v3RecoveryContainer = "6f0ae239243b71ac34e6db8467e1936566ebf807a356358ecdeeef7292a250fe"

type jobV3Recovery struct {
	commandID, backupDir, configPath string
	oldConfig, newConfig             []byte
}
type jobV3Permit struct {
	CommandID string    `json:"command_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func digestV3Recovery(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func (r *jobV3Recovery) matchesState(s *State) bool {
	return s != nil && s.AgentID == "401afd6a-a6ec-4a61-8a57-860a51f8a691" && s.JobGeneration == 39 && s.JobPending != nil && s.JobPending.ReleaseID == v3RecoveryTarget && s.JobPending.Generation == 40 && s.Deployed[shared.CompJob].ReleaseID == "2ea068a5-c8ea-4df3-a0be-247f72b73e8a" && s.JobRecovery != nil && s.JobRecovery.CommandID == "fc21cac3-3d2b-4099-949e-fda582b60375" && s.JobRecovery.Result == "attempting" && s.JobRecovery.PreviousIntent.ReleaseID == "d17301c7-7871-49ed-afe6-4f8afe04efdf" && s.JobRecovery.PreviousIntent.Generation == 40
}
func recoveryFile(path string) ([]byte, error) {
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || requireRootOwnedReadiness(st) != nil {
		return nil, errors.New("recovery_file_untrusted")
	}
	return os.ReadFile(path)
}
func (u *Upgrader) prepareJobV3Recovery(ctx context.Context, cmd shared.Command, rel *shared.Release, s *State) (*jobV3Recovery, error) {
	invalid := errors.New("exact_v3_recovery_not_authorized")
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || u.Root != "/opt/joincare-prod" || u.DataDir != "/var/lib/jc-agent" || cmd.Kind != shared.CmdUpgrade || cmd.Component != shared.CompJob || rel == nil || rel.ID != v3RecoveryTarget || cmd.ReleaseID != v3RecoveryTarget || rel.SHA256 != v3RecoverySHA || rel.Size != 48967218 {
		return nil, invalid
	}
	r := &jobV3Recovery{commandID: cmd.ID, configPath: filepath.Join(u.Root, "job-runtime/config/job/config.yaml")}
	if !r.matchesState(s) {
		return nil, invalid
	}
	permitDir := filepath.Join(u.DataDir, "job-v3-recovery")
	st, e := os.Lstat(permitDir)
	if e != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 || requireRootOwnedReadiness(st) != nil {
		return nil, invalid
	}
	raw, e := recoveryFile(filepath.Join(permitDir, "permit.json"))
	if e != nil {
		return nil, invalid
	}
	var permit jobV3Permit
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&permit) != nil || permit.CommandID != cmd.ID || !permit.ExpiresAt.After(time.Now()) || permit.ExpiresAt.After(time.Now().Add(30*time.Minute)) {
		return nil, invalid
	}
	config, e := os.ReadFile(r.configPath)
	if e != nil || digestV3Recovery(config) != v3RecoveryOldSHA {
		return nil, invalid
	}
	re := regexp.MustCompile(`(?m)^(\s*)jobMode:\s*ido\s*$`)
	next := re.ReplaceAll(config, []byte("${1}jobMode: ido-v3"))
	if digestV3Recovery(next) != v3RecoveryNewSHA {
		return nil, invalid
	}
	r.oldConfig = config
	r.newConfig = next
	if e = r.checkOldRuntime(ctx); e != nil {
		return nil, e
	}
	// Consume by rename. A crash cannot replay this permit or clear the intent.
	r.backupDir = filepath.Join(permitDir, "attempt-"+cmd.ID)
	if e = os.Mkdir(r.backupDir, 0700); e != nil {
		return nil, invalid
	}
	if e = os.Rename(filepath.Join(permitDir, "permit.json"), filepath.Join(r.backupDir, "permit.consumed.json")); e != nil {
		return nil, invalid
	}
	state, e := os.ReadFile(u.Store.Path)
	if e != nil {
		return nil, invalid
	}
	if durableStateWrite(filepath.Join(r.backupDir, "state.before.json"), state) != nil || durableStateWrite(filepath.Join(r.backupDir, "job.config.before.yaml"), config) != nil {
		return nil, invalid
	}
	return r, nil
}

func (r *jobV3Recovery) checkOldRuntime(ctx context.Context) error {
	invalid := errors.New("recovery_old_runtime_mismatch")
	out, e := exec.CommandContext(ctx, "docker", "inspect", "jc-job").Output()
	if e != nil {
		return invalid
	}
	var ds []struct {
		ID     string `json:"Id"`
		Image  string
		State  struct{ Running bool }
		Config struct{ Labels map[string]string }
	}
	if json.Unmarshal(out, &ds) != nil || len(ds) != 1 {
		return invalid
	}
	d := ds[0]
	if d.ID != v3RecoveryContainer || d.Image != v3RecoveryImage || !d.State.Running || d.Config.Labels["com.docker.compose.service"] != "job" || d.Config.Labels["com.docker.compose.project.working_dir"] != "/opt/joincare-prod" {
		return invalid
	}
	out, e = exec.CommandContext(ctx, "docker", "exec", "jc-job", "curl", "-sS", "--max-time", "8", "http://127.0.0.1:13007/health/job-ready").Output()
	if e != nil {
		return invalid
	}
	var v struct {
		Status bool
		Data   struct {
			DeadLetters int `json:"dead_letters"`
			Blocked     int `json:"blocked_chain_transactions"`
			Reorgs      int `json:"active_reorg_incidents"`
			Sync        struct {
				Error string `json:"last_error"`
			}
		}
	}
	if json.Unmarshal(out, &v) != nil || v.Status || v.Data.DeadLetters != 0 || v.Data.Blocked != 0 || v.Data.Reorgs != 0 || !bytes.Contains([]byte(v.Data.Sync.Error), []byte("keyless requests are served for the most recent 100 blocks only")) {
		return invalid
	}
	out, e = exec.CommandContext(ctx, "docker", "exec", "jc-job", "curl", "-sS", "--max-time", "8", "http://127.0.0.1:13007/health/ido-v3-stream").Output()
	if e != nil {
		return invalid
	}
	var p struct {
		CaughtUp bool `json:"caught_up"`
		Progress struct {
			Producer struct {
				State string
				Epoch uint64
			}
			Lag       uint64 `json:"producer_lag"`
			Consumers []struct {
				ProjectID   uint64
				State       string
				ProducerGap uint64
			}
		}
	}
	if json.Unmarshal(out, &p) != nil || !p.CaughtUp || p.Progress.Producer.State != "running" || p.Progress.Producer.Epoch != 3 || p.Progress.Lag != 0 {
		return invalid
	}
	found := false
	for _, c := range p.Progress.Consumers {
		if c.ProjectID == 43879281503240193 {
			found = c.State == "running" && c.ProducerGap <= 120
		}
	}
	if !found {
		return invalid
	}
	return nil
}

func (r *jobV3Recovery) replaceConfig(want string, data []byte) error {
	lstat, e := os.Lstat(r.configPath)
	if e != nil || !lstat.Mode().IsRegular() {
		return errors.New("recovery_config_not_regular")
	}
	current, e := os.ReadFile(r.configPath)
	if e != nil || digestV3Recovery(current) != want {
		return errors.New("recovery_config_changed")
	}
	st, e := os.Stat(r.configPath)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(r.configPath), ".jc-v3-recovery-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Chmod(tmp, st.Mode().Perm()); e != nil {
		return e
	}
	return durableStateRename(tmp, r.configPath)
}

type jobV3RecoveryStrategy struct {
	strategies.Strategy
	recovery *jobV3Recovery
}

func (s *jobV3RecoveryStrategy) Apply(ctx context.Context, path string, log io.Writer) error {
	r := s.recovery
	if e := r.checkOldRuntime(ctx); e != nil {
		return e
	}
	if e := r.replaceConfig(v3RecoveryOldSHA, r.newConfig); e != nil {
		return e
	}
	return s.Strategy.Apply(ctx, path, log)
}
func (s *jobV3RecoveryStrategy) Rollback(ctx context.Context, path string, log io.Writer) error {
	r := s.recovery
	raw, e := os.ReadFile(r.configPath)
	if e != nil {
		return e
	}
	if digestV3Recovery(raw) == v3RecoveryNewSHA {
		if e = r.replaceConfig(v3RecoveryNewSHA, r.oldConfig); e != nil {
			return e
		}
	} else if digestV3Recovery(raw) != v3RecoveryOldSHA {
		return errors.New("recovery_rollback_config_changed")
	}
	return s.Strategy.Rollback(ctx, path, log)
}
func (s *jobV3RecoveryStrategy) RollbackHealthCheck(ctx context.Context) error {
	// Preserve the failed journal if restored legacy readiness remains unhealthy.
	if v, ok := s.Strategy.(interface{ RollbackHealthCheck(context.Context) error }); ok {
		return v.RollbackHealthCheck(ctx)
	}
	return s.Strategy.HealthCheck(ctx)
}
