package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

const serviceLogTimeout = 30 * time.Second

var serviceLogContainers = map[string]string{
	"frontend": "jc-frontend",
	"admin":    "jc-admin",
}

type serviceLogRunner func(context.Context, string, ...string) (string, error)

func collectServiceLogs(ctx context.Context, target string, lines, sinceSeconds int, runner serviceLogRunner) (string, error) {
	if !shared.ValidServiceLogTarget(target) {
		return "", fmt.Errorf("service log target %q is not allowed", target)
	}
	if lines < 1 || lines > shared.MaxServiceLogLines {
		return "", fmt.Errorf("log lines must be between 1 and %d", shared.MaxServiceLogLines)
	}
	if sinceSeconds < 1 || sinceSeconds > shared.MaxServiceLogSince {
		return "", fmt.Errorf("log since must be between 1 and %d seconds", shared.MaxServiceLogSince)
	}

	queryCtx, cancel := context.WithTimeout(ctx, serviceLogTimeout)
	defer cancel()

	if target == "agent" {
		return runner(queryCtx, "journalctl",
			"-u", "jc-agent.service",
			"--no-pager",
			"-n", strconv.Itoa(lines),
			"--since", fmt.Sprintf("%d seconds ago", sinceSeconds),
			"-o", "short-iso",
		)
	}

	container := serviceLogContainers[target]
	containerOutput, containerErr := runner(queryCtx, "docker", "logs",
		"--tail", strconv.Itoa(lines),
		"--since", fmt.Sprintf("%ds", sinceSeconds),
		"--timestamps",
		container,
	)
	return containerOutput, containerErr
}

var runCappedLogCommand serviceLogRunner = func(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	buf := &cappedLogBuffer{max: shared.MaxCommandLogBytes}
	cmd.Stdout = buf
	cmd.Stderr = buf
	err := cmd.Run()
	return buf.String(), err
}

type cappedLogBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (b *cappedLogBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return originalLen, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return originalLen, nil
}

func (b *cappedLogBuffer) String() string {
	value := b.buf.String()
	if b.truncated {
		value += "\n[output truncated at 256 KiB]\n"
	}
	return shared.CapUTF8Bytes(value, b.max)
}

func capServiceLogString(value string) string {
	return shared.CapUTF8Bytes(value, shared.MaxCommandLogBytes)
}
