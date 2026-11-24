package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/joincare/jc-release/internal/shared"
	"github.com/joincare/jc-release/internal/strategies"
)

const backupsToKeep = 3

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Upgrader coordinates the per-command lifecycle:
// download → verify → backup → apply → healthcheck → rollback (on failure) → report.
type Upgrader struct {
	Client           *Client
	Store            *Store
	Registry         strategies.Registry
	DataDir          string
	AgentID          string
	Logger           *slog.Logger
	Root             string
	JobEnvironment   string
	JobTrustedKeys   map[string]string
	JobReadinessFile string
	JobEvidenceDir   string
	JobReadinessKeys map[string]string
	JobHostKeySHA256 string
	// Test-only override; production configuration has no corresponding key.
	jobHostKeyFile string
	jobRecovery    *jobV3Recovery
}

// ExecuteCommand runs one shared.Command end-to-end and returns final status + log.
// The returned log is the concatenated stdout/stderr of all commands invoked;
// caller is responsible for POST /report.
func (u *Upgrader) ExecuteCommand(ctx context.Context, cmd shared.Command, rel *shared.Release) (status, logStr, errMsg string) {
	if cmd.Component == shared.CompJob {
		jobExecutionMu.Lock()
		defer jobExecutionMu.Unlock()
		unlock, err := lockJobExecution(u.DataDir)
		if err != nil {
			return shared.StatusFailed, "", "job_execution_lock_unavailable"
		}
		defer unlock()
	}
	var buf bytes.Buffer
	logger := u.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	logger = logger.With("cmd_id", cmd.ID, "kind", cmd.Kind, "component", cmd.Component)
	var jobGeneration uint64

	fmt.Fprintf(&buf, "=== jc-agent · command %s · %s %s ===\n", cmd.ID, cmd.Kind, cmd.Component)
	fmt.Fprintf(&buf, "started_at: %s (UTC)\n", time.Now().UTC().Format(time.RFC3339))
	if cmd.Kind == shared.CmdLogs {
		return u.doServiceLogs(ctx, cmd, &buf)
	}
	if cmd.Component == shared.CompJob {
		if cmd.Kind != shared.CmdUpgrade {
			return releaseFailure(&buf, "job_upgrade_only")
		}
		if u.Store == nil {
			return releaseFailure(&buf, "job_runtime_admission_unavailable: job_state_unavailable")
		}
		state, err := u.Store.Load()
		if err != nil {
			return releaseFailure(&buf, "job_runtime_admission_unavailable: job_state_unavailable")
		}
		if state.JobPending != nil {
			recovery, err := u.prepareJobV3Recovery(ctx, cmd, rel, state)
			if err != nil {
				return releaseFailure(&buf, "job_pending_recovery_required")
			}
			u.jobRecovery = recovery
			defer func() { u.jobRecovery = nil }()
		}
		if rel == nil || rel.ID != cmd.ReleaseID || rel.Component != shared.CompJob || rel.Size <= 0 || !sha256Pattern.MatchString(rel.SHA256) {
			return releaseFailure(&buf, "job_release_identity_invalid")
		}
		jobGeneration, err = u.admitJob(rel)
		if err != nil {
			return releaseFailure(&buf, "job_runtime_admission_unavailable: "+err.Error())
		}
	}
	if cmd.Component == shared.CompTGBot || cmd.Component == shared.CompLang {
		return releaseFailure(&buf, "component_release_admission_unavailable")
	}
	if cmd.Kind == shared.CmdUpgrade {
		if rel == nil || rel.ID != cmd.ReleaseID {
			return releaseFailure(&buf, "release lookup returned nil or wrong release")
		}
		if rel.Component != cmd.Component {
			return releaseFailure(&buf, "release_component_mismatch")
		}
		if rel.Size <= 0 || !sha256Pattern.MatchString(rel.SHA256) {
			return releaseFailure(&buf, "release_identity_invalid")
		}
	}

	strat, err := u.Registry.Get(cmd.Component)
	if err != nil {
		fmt.Fprintf(&buf, "ERROR: %v\n", err)
		return shared.StatusFailed, buf.String(), err.Error()
	}
	if u.jobRecovery != nil {
		strat = &jobV3RecoveryStrategy{Strategy: strat, recovery: u.jobRecovery}
		fmt.Fprintln(&buf, "RECOVERY: exact failed intent retained; continuing only authorized V3-only release; old V2 readiness is degraded, NOT healthy")
	}

	switch cmd.Kind {
	case shared.CmdUpgrade:
		return u.doUpgrade(ctx, cmd, rel, strat, &buf, logger, jobGeneration)
	case shared.CmdRollback:
		return u.doRollback(ctx, cmd, strat, &buf, logger)
	case shared.CmdRestart:
		return u.doRestart(ctx, cmd, strat, &buf, logger)
	default:
		msg := fmt.Sprintf("unknown command kind: %s", cmd.Kind)
		fmt.Fprintln(&buf, msg)
		return shared.StatusFailed, buf.String(), msg
	}
}

func releaseFailure(buf *bytes.Buffer, message string) (string, string, string) {
	fmt.Fprintf(buf, "ERROR: %s\n", message)
	return shared.StatusFailed, buf.String(), message
}

func (u *Upgrader) doServiceLogs(ctx context.Context, cmd shared.Command, buf *bytes.Buffer) (string, string, string) {
	target := string(cmd.Component)
	fmt.Fprintf(buf, "target: %s, lines: %d, since: %ds\n\n", target, cmd.LogLines, cmd.LogSince)
	output, err := collectServiceLogs(ctx, target, cmd.LogLines, cmd.LogSince, runCappedLogCommand)
	if output == "" {
		output = "(no log lines returned)\n"
	}
	buf.WriteString(output)
	if err != nil {
		msg := fmt.Sprintf("collect service logs: %v", err)
		fmt.Fprintf(buf, "\n=== FAILED: %s ===\n", msg)
		return shared.StatusFailed, buf.String(), msg
	}
	fmt.Fprintln(buf, "\n=== SUCCESS ===")
	return shared.StatusSuccess, buf.String(), ""
}

func (u *Upgrader) doUpgrade(ctx context.Context, cmd shared.Command, rel *shared.Release, strat strategies.Strategy, buf *bytes.Buffer, logger *slog.Logger, jobGeneration uint64) (string, string, string) {
	fmt.Fprintf(buf, "release: %s v%s size=%d sha256=%s\n", rel.ID, rel.Version, rel.Size, rel.SHA256)

	// 1. download
	incoming := filepath.Join(u.DataDir, "incoming", rel.ID+".tar.gz")
	fmt.Fprintf(buf, "\n[1/5] downloading to %s\n", incoming)
	if _, err := u.Client.DownloadArtifact(rel.ID, rel.SHA256, incoming); err != nil {
		return u.failWith(buf, "download", err)
	}
	defer os.Remove(incoming)

	// 2. backup (only if a live version exists — HealthCheck should NOT gate this because first-time deploys have no prior state, but that's rare)
	backupDir := BackupDir(u.DataDir, cmd.Component)
	if err := EnsureBackupDir(u.DataDir, cmd.Component); err != nil {
		return u.failWith(buf, "backup mkdir", err)
	}
	fmt.Fprintf(buf, "\n[2/5] backing up current state into %s\n", backupDir)
	backupPath, bkerr := strat.Backup(ctx, backupDir, buf)
	if bkerr != nil {
		return u.failWith(buf, "backup", fmt.Errorf("refusing apply without rollback artifact: %w", bkerr))
	}
	fmt.Fprintf(buf, "[backup] saved %s\n", backupPath)
	if cmd.Component == shared.CompJob {
		// Download and docker save can take minutes. Recheck the immutable
		// descriptor (or legacy signed readiness), layout, signer and generation
		// immediately before persisting the durable intent.
		if len(rel.JobAdmission) != 0 {
			confirmed, err := u.admitJob(rel)
			if err != nil || confirmed != jobGeneration {
				return u.failWith(buf, "job pre-apply admission", errors.New("job_readiness_changed_before_apply"))
			}
		}
		if err := u.Store.Update(func(st *State) error {
			if u.jobRecovery != nil {
				if !u.jobRecovery.matchesState(st) || jobGeneration != 40 {
					return errors.New("job_recovery_state_changed")
				}
				st.JobRecovery = &JobRecoveryJournal{CommandID: cmd.ID, PreviousIntent: *st.JobPending, TargetRelease: rel.ID, BackupDir: u.jobRecovery.backupDir, Result: "attempting", PreviousRecovery: st.JobRecovery}
			} else if st.JobPending != nil || jobGeneration <= st.JobGeneration {
				return errors.New("job_generation_or_pending_changed")
			}
			st.JobPending = &JobIntent{ReleaseID: rel.ID, Generation: jobGeneration}
			return nil
		}); err != nil {
			return u.failWith(buf, "job intent persist", err)
		}
	}

	// 3. apply
	fmt.Fprintf(buf, "\n[3/5] applying release\n")
	if err := strat.Apply(ctx, incoming, buf); err != nil {
		return u.rollbackOrFail(ctx, cmd, strat, backupPath, buf, logger, "apply", err)
	}

	// 4. health
	fmt.Fprintf(buf, "\n[4/5] health check\n")
	if err := strat.HealthCheck(ctx); err != nil {
		return u.rollbackOrFail(ctx, cmd, strat, backupPath, buf, logger, "healthcheck", err)
	}

	// 5. persist state
	fmt.Fprintf(buf, "\n[5/5] recording deployed version in state.json\n")
	err := u.Store.Update(func(st *State) error {
		if cmd.Component == shared.CompJob {
			if st.JobPending == nil || st.JobPending.ReleaseID != rel.ID || st.JobPending.Generation != jobGeneration || jobGeneration <= st.JobGeneration {
				return errors.New("job_generation_replayed")
			}
			st.JobGeneration = jobGeneration
			st.JobPending = nil
			if u.jobRecovery != nil {
				if st.JobRecovery == nil || st.JobRecovery.CommandID != cmd.ID {
					return errors.New("job_recovery_journal_changed")
				}
				st.JobRecovery.Result = "success"
			}
		}
		st.Deployed[cmd.Component] = DeployedEntry{
			ReleaseID: rel.ID, Version: rel.Version, Component: cmd.Component,
			AppliedAt: time.Now().UTC(), BackupPath: backupPath,
		}
		return nil
	})
	if err != nil {
		return u.rollbackOrFail(ctx, cmd, strat, backupPath, buf, logger, "state persist", err)
	}
	if err := PruneOldBackups(u.DataDir, cmd.Component, backupsToKeep); err != nil {
		logger.Warn("backup prune failed", "err", err)
	}
	fmt.Fprintf(buf, "\n=== SUCCESS ===\n")
	return shared.StatusSuccess, buf.String(), ""
}

func (u *Upgrader) doRollback(ctx context.Context, cmd shared.Command, strat strategies.Strategy, buf *bytes.Buffer, logger *slog.Logger) (string, string, string) {
	backup, err := LatestBackup(u.DataDir, cmd.Component)
	if err != nil || backup == "" {
		msg := "no backup available for rollback"
		if err != nil {
			msg = err.Error()
		}
		fmt.Fprintln(buf, msg)
		return shared.StatusFailed, buf.String(), msg
	}
	fmt.Fprintf(buf, "rolling back %s from %s\n", cmd.Component, backup)
	if err := strat.Rollback(ctx, backup, buf); err != nil {
		return u.failWith(buf, "rollback", err)
	}
	if err := strat.HealthCheck(ctx); err != nil {
		return u.failWith(buf, "healthcheck after rollback", err)
	}
	// Clear deployed entry so admin knows this component is on an older backup version.
	_ = u.Store.Update(func(st *State) error {
		delete(st.Deployed, cmd.Component)
		return nil
	})
	fmt.Fprintln(buf, "\n=== ROLLED_BACK ===")
	return shared.StatusSuccess, buf.String(), ""
}

func (u *Upgrader) doRestart(ctx context.Context, cmd shared.Command, strat strategies.Strategy, buf *bytes.Buffer, logger *slog.Logger) (string, string, string) {
	if err := strat.HealthCheck(ctx); err != nil {
		// try a docker restart of the primary container
		// We use the strategy's implementation details via the Apply path only for image-based
		// components — here just report health status.
		fmt.Fprintf(buf, "healthcheck failed: %v\n", err)
		return shared.StatusFailed, buf.String(), err.Error()
	}
	fmt.Fprintln(buf, "restart no-op: component reported healthy")
	return shared.StatusSuccess, buf.String(), ""
}

func (u *Upgrader) failWith(buf *bytes.Buffer, phase string, err error) (string, string, string) {
	fmt.Fprintf(buf, "\n=== FAILED @ %s: %v ===\n", phase, err)
	return shared.StatusFailed, buf.String(), fmt.Sprintf("%s: %v", phase, err)
}

// rollbackOrFail: apply/healthcheck failed → attempt rollback from backupPath.
// A failed upgrade remains failed even when the previous version is restored.
// The rollback result is preserved in the log and error text.
func (u *Upgrader) rollbackOrFail(ctx context.Context, cmd shared.Command, strat strategies.Strategy, backupPath string, buf *bytes.Buffer, logger *slog.Logger, phase string, applyErr error) (string, string, string) {
	fmt.Fprintf(buf, "\n=== %s FAILED: %v ===\n", phase, applyErr)
	if backupPath == "" {
		fmt.Fprintln(buf, "no backup available → cannot auto-rollback")
		return shared.StatusFailed, buf.String(), fmt.Sprintf("%s: %v (no backup)", phase, applyErr)
	}
	fmt.Fprintf(buf, "\n[rollback] restoring from %s\n", backupPath)
	// The command context may already be canceled (health timeout, poller
	// shutdown). A bounded independent recovery window is needed to attempt
	// restoring the old image; otherwise every Docker command fails at once.
	// An unverified recovery still retains job_pending and blocks retry.
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	defer cancel()
	if rbErr := strat.Rollback(recoveryCtx, backupPath, buf); rbErr != nil {
		fmt.Fprintf(buf, "\n=== ROLLBACK FAILED: %v ===\n", rbErr)
		return shared.StatusFailed, buf.String(), fmt.Sprintf("%s: %v; rollback also failed: %v", phase, applyErr, rbErr)
	}
	checkRollback := strat.HealthCheck
	if job, ok := strat.(interface{ RollbackHealthCheck(context.Context) error }); ok && cmd.Component == shared.CompJob {
		checkRollback = job.RollbackHealthCheck
	}
	if hcErr := checkRollback(recoveryCtx); hcErr != nil {
		fmt.Fprintf(buf, "\n=== ROLLBACK HEALTHCHECK FAILED: %v ===\n", hcErr)
		return shared.StatusFailed, buf.String(), fmt.Sprintf("%s: %v; rollback healthcheck failed: %v", phase, applyErr, hcErr)
	}
	if cmd.Component == shared.CompJob {
		if err := u.Store.Update(func(st *State) error {
			if st.JobPending == nil || st.JobPending.ReleaseID != cmd.ReleaseID {
				return errors.New("job_pending_intent_changed")
			}
			st.JobPending = nil
			return nil
		}); err != nil {
			return shared.StatusFailed, buf.String(), fmt.Sprintf("%s: %v; recovery journal retained: %v", phase, applyErr, err)
		}
	}
	fmt.Fprintln(buf, "\n=== ROLLED_BACK (previous version restored) ===")
	return shared.StatusFailed, buf.String(), fmt.Sprintf("%s: %v (rolled back; previous version restored)", phase, applyErr)
}

// LookupReleaseForComponent binds both directions so a release for one static
// surface cannot be dispatched as another component.
func (u *Upgrader) LookupReleaseForComponent(id string, component shared.Component, cachedManifest *shared.Manifest) *shared.Release {
	if cachedManifest == nil {
		return nil
	}
	r := cachedManifest.Get(component)
	if r != nil && r.ID == id && r.Component == component {
		return r
	}
	return nil
}

// LookupComponent returns Component of the strategy given a command.
func (u *Upgrader) getStrategyOrErr(c shared.Component) (strategies.Strategy, error) {
	return u.Registry.Get(c)
}

// helper: derive from writer used only in tests
var _ io.Writer = (*bytes.Buffer)(nil)
var _ = errors.New
