package strategies

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// runCmd executes name+args, piping stdout+stderr into log while retaining
// stdout for callers that need to parse a Docker response.
func runCmd(ctx context.Context, log io.Writer, name string, args ...string) (string, error) {
	fmt.Fprintf(log, "\n$ %s %s\n", name, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, log)
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s failed: %w", name, err)
	}
	return stdout.String(), nil
}

// runShell is restricted to the two required streaming pipelines: docker
// load from an archive and docker save through gzip. Paths and image names are
// quoted by their callers and never originate from a shell command request.
func runShell(ctx context.Context, log io.Writer, cmdline string) (string, error) {
	fmt.Fprintf(log, "\n$ sh -c %q\n", cmdline)
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdline)
	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, log)
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("sh -c failed: %w", err)
	}
	return stdout.String(), nil
}
