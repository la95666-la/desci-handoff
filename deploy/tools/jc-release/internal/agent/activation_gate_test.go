package agent

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/joincare/jc-release/internal/shared"
	"github.com/joincare/jc-release/internal/strategies"
)

type sentinelStrategy struct {
	component shared.Component
	calls     int
}

type failingApplyStrategy struct{ sentinelStrategy }

func (s *failingApplyStrategy) Apply(context.Context, string, io.Writer) error {
	s.calls++
	return io.ErrUnexpectedEOF
}

func (s *sentinelStrategy) Component() shared.Component                    { return s.component }
func (s *sentinelStrategy) Apply(context.Context, string, io.Writer) error { s.calls++; return nil }
func (s *sentinelStrategy) Backup(context.Context, string, io.Writer) (string, error) {
	s.calls++
	return "", nil
}
func (s *sentinelStrategy) Rollback(context.Context, string, io.Writer) error { s.calls++; return nil }
func (s *sentinelStrategy) HealthCheck(context.Context) error                 { s.calls++; return nil }

func TestJobReleaseCommandsRemainUnavailableBeforeStrategyIO(t *testing.T) {
	s := &sentinelStrategy{component: shared.CompJob}
	u := &Upgrader{Registry: map[shared.Component]strategies.Strategy{shared.CompJob: s}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: t.TempDir()}
	if err := os.Chmod(u.DataDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{shared.CmdUpgrade, shared.CmdRollback, shared.CmdRestart} {
		cmd := shared.Command{ID: "gate", Kind: kind, Component: shared.CompJob, ReleaseID: "r"}
		rel := &shared.Release{ID: "r", Component: shared.CompJob, Size: 1, SHA256: strings.Repeat("a", 64)}
		status, _, msg := u.ExecuteCommand(context.Background(), cmd, rel)
		if status != shared.StatusFailed || (kind == shared.CmdUpgrade && !strings.HasPrefix(msg, "job_runtime_admission_unavailable:")) || (kind != shared.CmdUpgrade && msg != "job_upgrade_only") || s.calls != 0 {
			t.Fatalf("%s status=%s msg=%s calls=%d", kind, status, msg, s.calls)
		}
	}
}

func TestAuxiliaryReleaseCommandsRemainUnavailableBeforeStrategyIO(t *testing.T) {
	for _, component := range []shared.Component{shared.CompTGBot, shared.CompLang} {
		s := &sentinelStrategy{component: component}
		u := &Upgrader{Registry: map[shared.Component]strategies.Strategy{component: s}}
		rel := &shared.Release{ID: "r", Component: component, Size: 1, SHA256: strings.Repeat("a", 64)}
		status, _, msg := u.ExecuteCommand(context.Background(), shared.Command{ID: "gate", Kind: shared.CmdUpgrade, Component: component, ReleaseID: "r"}, rel)
		if status != shared.StatusFailed || msg != "component_release_admission_unavailable" || s.calls != 0 {
			t.Fatalf("%s status=%s msg=%s calls=%d", component, status, msg, s.calls)
		}
	}
}
func TestReleaseComponentMismatchRejectedBeforeStrategy(t *testing.T) {
	s := &sentinelStrategy{component: shared.CompAdmin}
	u := &Upgrader{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	status, _, msg := u.ExecuteCommand(context.Background(), shared.Command{ID: "m", Kind: shared.CmdUpgrade, Component: shared.CompAdmin, ReleaseID: "front"}, &shared.Release{ID: "front", Component: shared.CompFrontend})
	if status != shared.StatusFailed || msg != "release_component_mismatch" || s.calls != 0 {
		t.Fatalf("status=%s msg=%s calls=%d", status, msg, s.calls)
	}
}
func TestLookupReleaseBindsComponentBothDirections(t *testing.T) {
	u := &Upgrader{}
	m := &shared.Manifest{}
	front := &shared.Release{ID: "front", Component: shared.CompFrontend}
	admin := &shared.Release{ID: "admin", Component: shared.CompAdmin}
	m.Set(shared.CompFrontend, front)
	m.Set(shared.CompAdmin, admin)
	if u.LookupReleaseForComponent("front", shared.CompAdmin, m) != nil {
		t.Fatal("frontend release matched admin")
	}
	if u.LookupReleaseForComponent("admin", shared.CompFrontend, m) != nil {
		t.Fatal("admin release matched frontend")
	}
	if u.LookupReleaseForComponent("front", shared.CompFrontend, m) != front {
		t.Fatal("exact frontend release not found")
	}
}

func TestSuccessfulAutomaticRollbackReportsFailedTerminalStatus(t *testing.T) {
	s := &failingApplyStrategy{sentinelStrategy: sentinelStrategy{component: shared.CompBackend}}
	u := &Upgrader{Registry: map[shared.Component]strategies.Strategy{shared.CompBackend: s}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	var buf bytes.Buffer
	status, logText, errText := u.rollbackOrFail(context.Background(), shared.Command{Component: shared.CompBackend}, s, "backup.tar.gz", &buf, u.Logger, "apply", io.ErrUnexpectedEOF)
	if status != shared.StatusFailed || !strings.Contains(logText, "previous version restored") || !strings.Contains(errText, "rolled back") {
		t.Fatalf("status=%s log=%q err=%q", status, logText, errText)
	}
}
