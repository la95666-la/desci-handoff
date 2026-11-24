package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/joincare/jc-release/internal/shared"
)

func TestCollectServiceLogsUsesStaticAllowlistedContainer(t *testing.T) {
	for target, container := range map[string]string{"frontend": "jc-frontend", "admin": "jc-admin"} {
		var gotName string
		var gotArgs []string
		_, err := collectServiceLogs(context.Background(), target, 120, 900, func(_ context.Context, name string, args ...string) (string, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return "log\n", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotName != "docker" || !strings.Contains(strings.Join(gotArgs, "|"), container) {
			t.Fatalf("%s: %s %#v", target, gotName, gotArgs)
		}
	}
}
func TestCollectServiceLogsUsesJournalForAgent(t *testing.T) {
	var gotName string
	var gotArgs []string
	_, err := collectServiceLogs(context.Background(), "agent", 50, 60, func(_ context.Context, name string, args ...string) (string, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return "", nil
	})
	if err != nil || gotName != "journalctl" || !strings.Contains(strings.Join(gotArgs, " "), "jc-agent.service") {
		t.Fatalf("%v %s %#v", err, gotName, gotArgs)
	}
}
func TestCollectServiceLogsRejectsProtectedAndHostileTargets(t *testing.T) {
	for _, target := range []string{"backend", "job", "tgbot", "mysql", "hardhat", "redis", "caddy", "../../root"} {
		called := false
		_, err := collectServiceLogs(context.Background(), target, 10, 60, func(context.Context, string, ...string) (string, error) { called = true; return "", nil })
		if err == nil || called {
			t.Fatalf("target %q err=%v called=%v", target, err, called)
		}
	}
}
func TestCappedLogBufferTruncatesWithoutShortWrite(t *testing.T) {
	buf := &cappedLogBuffer{max: 64}
	n, err := buf.Write([]byte(strings.Repeat("x", 80)))
	if err != nil || n != 80 || !strings.Contains(buf.String(), "truncated") || len(buf.String()) > 64 {
		t.Fatalf("n=%d err=%v len=%d out=%q", n, err, len(buf.String()), buf.String())
	}
}

func TestExecuteLargeServiceLogProducesAdmissibleReport(t *testing.T) {
	old := runCappedLogCommand
	runCappedLogCommand = func(context.Context, string, ...string) (string, error) {
		return strings.Repeat("界", shared.MaxCommandLogBytes), nil
	}
	defer func() { runCappedLogCommand = old }()
	u := &Upgrader{Logger: slog.Default()}
	status, logStr, errMsg := u.ExecuteCommand(context.Background(), shared.Command{
		ID: "large-log", Kind: shared.CmdLogs, Component: shared.CompFrontend, LogLines: 10, LogSince: 60,
	}, nil)
	logStr, errMsg = shared.CapCommandReport(logStr, errMsg)
	if status != shared.StatusSuccess || len(logStr) > shared.MaxCommandLogBytes || len(errMsg) > shared.MaxCommandErrorBytes {
		t.Fatalf("status=%s log=%d error=%d", status, len(logStr), len(errMsg))
	}
}
