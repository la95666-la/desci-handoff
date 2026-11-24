//go:build docker_integration

package strategies

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in only: this test creates a temporary jc-job on the LOCAL
// Docker daemon. It never touches an existing jc-job, even if one is stopped.
func TestJobLocalDockerFirstDeployAndFailedUpgradeRollback(t *testing.T) {
	if os.Getenv("JC_JOB_DOCKER_INTEGRATION") != "1" {
		t.Skip("requires explicit local Docker opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	docker := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "docker", args...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		return strings.TrimSpace(out.String()), err
	}
	if _, err := docker("info", "--format", "{{.ServerVersion}}"); err != nil {
		t.Fatalf("local Docker unavailable: %v", err)
	}
	checkAbsent := func() bool {
		out, err := docker("container", "ls", "-a", "--filter", "name=^/jc-job$", "--format", "{{.ID}}")
		return err == nil && out == ""
	}
	if !checkAbsent() {
		t.Skip("jc-job already exists; never adopt or remove an unrelated container")
	}
	root := jobFixture(t)
	var images []string
	defer func() {
		// Only remove a container whose Compose binding points to this fixture.
		out, err := docker("inspect", "--format", "{{.Id}} {{index .Config.Labels \"com.docker.compose.service\"}} {{index .Config.Labels \"com.docker.compose.project.working_dir\"}}", "jc-job")
		fields := strings.Fields(out)
		if err == nil && len(fields) == 3 && fields[1] == "job" && fields[2] == root {
			_, _ = docker("rm", "-f", fields[0])
		}
		for _, tag := range images {
			_, _ = docker("image", "rm", tag)
		}
	}()
	build := func(name, script string) (string, string) {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM nginx:alpine\nCOPY job /app/joincare-job\nCOPY ready.conf /etc/nginx/conf.d/default.conf\nRUN chmod 755 /app/joincare-job\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "job"), []byte(script), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ready.conf"), []byte("server { listen 13007; location = /health/job-ready { return 200 'ready'; } }\n"), 0600); err != nil {
			t.Fatal(err)
		}
		tag := fmt.Sprintf("jc-job-local-integration-%d:%s", time.Now().UnixNano(), name)
		if out, err := docker("build", "--pull=false", "-q", "-t", tag, dir); err != nil {
			t.Fatalf("build %s: %v %s", name, err, out)
		}
		images = append(images, tag)
		id, err := docker("image", "inspect", "--format", "{{.Id}}", tag)
		if err != nil || !validImageID(id) {
			t.Fatalf("built image identity: %q %v", id, err)
		}
		archive := filepath.Join(t.TempDir(), name+".tar")
		if out, err := docker("save", "-o", archive, tag); err != nil {
			t.Fatalf("save %s: %v %s", name, err, out)
		}
		return id, archive
	}
	goodID, goodArchive := build("healthy", "#!/bin/sh\nexec nginx -g 'daemon off;'\n")
	badID, badArchive := build("broken", "#!/bin/sh\nexit 17\n")
	s := &jobStrategy{root: root}
	if err := s.SetJobImageID(goodID); err != nil {
		t.Fatal(err)
	}
	backup, err := s.Backup(ctx, filepath.Join(t.TempDir(), "backup"), io.Discard)
	if err != nil || backup != "job-absent" {
		t.Fatalf("initial state: %q %v", backup, err)
	}
	if err := s.Apply(ctx, goodArchive, io.Discard); err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	if err := s.HealthCheck(ctx); err != nil {
		t.Fatalf("first readiness: %v", err)
	}
	if err := s.SetJobImageID(badID); err != nil {
		t.Fatal(err)
	}
	backup, err = s.Backup(ctx, filepath.Join(t.TempDir(), "backup"), io.Discard)
	if err != nil || backup == "" || backup == "job-absent" {
		t.Fatalf("image backup: %q %v", backup, err)
	}
	if err := s.Apply(ctx, badArchive, io.Discard); err != nil {
		t.Fatalf("broken image failed before health verification: %v", err)
	}
	failedCtx, stop := context.WithTimeout(ctx, 4*time.Second)
	defer stop()
	if err := s.HealthCheck(failedCtx); err == nil {
		t.Fatal("broken image reported ready")
	}
	if err := s.Rollback(ctx, backup, io.Discard); err != nil {
		t.Fatalf("restore image: %v", err)
	}
	if err := s.RollbackHealthCheck(ctx); err != nil {
		t.Fatalf("restored readiness: %v", err)
	}
	if current, err := s.existingImage(ctx); err != nil || current != goodID {
		t.Fatalf("wrong image after rollback: %s %v", current, err)
	}
	if err := s.existingBinding(ctx); err != nil {
		t.Fatalf("rollback lost Compose identity: %v", err)
	}
	// Exercise the first-deploy failure path with the real daemon as well:
	// only this attempt's container may be removed, ending with no jc-job.
	container, err := docker("inspect", "--format", "{{.Id}} {{index .Config.Labels \"com.docker.compose.project.working_dir\"}}", "jc-job")
	fields := strings.Fields(container)
	if err != nil || len(fields) != 2 || fields[1] != root {
		t.Fatalf("cannot safely reset isolated fixture: %q %v", container, err)
	}
	if out, err := docker("rm", "-f", fields[0]); err != nil {
		t.Fatalf("reset fixture container: %v %s", err, out)
	}
	if err := s.SetJobImageID(badID); err != nil {
		t.Fatal(err)
	}
	backup, err = s.Backup(ctx, filepath.Join(t.TempDir(), "backup"), io.Discard)
	if err != nil || backup != "job-absent" {
		t.Fatalf("second first-deploy precondition: %q %v", backup, err)
	}
	if err := s.Apply(ctx, badArchive, io.Discard); err != nil {
		t.Fatalf("broken first deploy failed before health check: %v", err)
	}
	firstFailedCtx, stopFirst := context.WithTimeout(ctx, 4*time.Second)
	defer stopFirst()
	if err := s.HealthCheck(firstFailedCtx); err == nil {
		t.Fatal("broken first deployment reported ready")
	}
	if err := s.Rollback(ctx, backup, io.Discard); err != nil {
		t.Fatalf("first deploy cleanup: %v", err)
	}
	if err := s.RollbackHealthCheck(ctx); err != nil || !checkAbsent() {
		t.Fatalf("first deploy cleanup left a container: %v", err)
	}
}
