package strategies

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

// imageStrategy handles docker-save gzip artifacts for Backend and TGBot.
type imageStrategy struct {
	name    string
	service string
	root    string
	comp    shared.Component
}

func (s *imageStrategy) Component() shared.Component { return s.comp }

func (s *imageStrategy) svcName() string {
	if s.service != "" {
		return s.service
	}
	return strings.TrimPrefix(s.name, "jc-")
}

func (s *imageStrategy) currentTag(ctx context.Context, log io.Writer) (string, error) {
	out, err := runCmd(ctx, log, "docker", "inspect", "--format", "{{.Config.Image}}", s.name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *imageStrategy) Apply(ctx context.Context, src string, log io.Writer) error {
	currentTag, err := s.currentTag(ctx, log)
	if err != nil {
		return fmt.Errorf("inspect current: %w", err)
	}
	loadOut, err := runShell(ctx, log, fmt.Sprintf("docker load < %q", src))
	if err != nil {
		return err
	}
	newImage := parseLoadedImage(loadOut)
	if newImage == "" {
		return fmt.Errorf("could not parse loaded image name from: %s", loadOut)
	}
	fmt.Fprintf(log, "\n[apply] loaded image = %s\n", newImage)
	fmt.Fprintf(log, "[apply] retagging as %s\n", currentTag)
	if _, err := runCmd(ctx, log, "docker", "tag", newImage, currentTag); err != nil {
		return err
	}
	compose := filepath.Join(s.root, "docker-compose.yml")
	if _, err := runCmd(ctx, log, "docker", "compose", "-f", compose, "up", "-d", "--force-recreate", s.svcName()); err != nil {
		return err
	}
	return nil
}

func (s *imageStrategy) Backup(ctx context.Context, dstDir string, log io.Writer) (string, error) {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return "", err
	}
	tag, err := s.currentTag(ctx, log)
	if err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	dst := filepath.Join(dstDir, fmt.Sprintf("%s-%s.tar.gz", string(s.comp), stamp))
	if _, err := runShell(ctx, log, fmt.Sprintf("docker save %q | gzip > %q", tag, dst)); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *imageStrategy) Rollback(ctx context.Context, backup string, log io.Writer) error {
	if backup == "" {
		return ErrNoBackup
	}
	if _, err := os.Stat(backup); err != nil {
		return fmt.Errorf("backup missing: %w", err)
	}
	return s.Apply(ctx, backup, log)
}

func (s *imageStrategy) HealthCheck(ctx context.Context) error {
	deadline := time.Now().Add(120 * time.Second)
	graceDeadline := time.Now().Add(45 * time.Second)
	initialRestarts := -1
	stableWithoutHealth := 0
	for time.Now().Before(deadline) {
		out, err := runCmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.State.Status}}", s.name)
		if err != nil {
			return err
		}
		state := strings.TrimSpace(out)
		if state != "running" {
			if time.Now().Before(graceDeadline) {
				time.Sleep(2 * time.Second)
				continue
			}
			return fmt.Errorf("%s state = %s, expected running", s.name, state)
		}
		hcOut, hcErr := runCmd(ctx, io.Discard, "docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", s.name)
		if hcErr == nil {
			health := strings.TrimSpace(hcOut)
			switch health {
			case "healthy":
				return nil
			case "unhealthy":
				return fmt.Errorf("%s healthcheck reports unhealthy", s.name)
			case "":
				stableWithoutHealth++
			default:
				stableWithoutHealth = 0
			}
		}
		restartOut, err := runCmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.RestartCount}}", s.name)
		if err != nil {
			return err
		}
		restarts := 0
		_, _ = fmt.Sscanf(strings.TrimSpace(restartOut), "%d", &restarts)
		if initialRestarts < 0 {
			initialRestarts = restarts
		} else if restarts > initialRestarts {
			return fmt.Errorf("%s restarted %d times during healthcheck", s.name, restarts-initialRestarts)
		}
		if stableWithoutHealth >= 8 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("%s did not become healthy before timeout", s.name)
}

func parseLoadedImage(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Loaded image:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Loaded image:"))
		}
	}
	return ""
}
