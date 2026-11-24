package strategies

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joincare/jc-release/internal/shared"
	"gopkg.in/yaml.v3"
)

func TestJobStrategyRegisteredButNoUnreviewedMutation(t *testing.T) {
	s, err := New(t.TempDir()).Get(shared.CompJob)
	if err != nil {
		t.Fatal(err)
	}
	job, ok := s.(*jobStrategy)
	if !ok {
		t.Fatalf("missing dedicated job strategy: %T", s)
	}
	if err := job.Apply(context.Background(), "nonexistent", io.Discard); err == nil {
		t.Fatal("missing layout applied")
	}
	if err := job.SetJobImageID("joincare/backend:latest"); err == nil {
		t.Fatal("mutable tag admitted")
	}
	if err := job.SetJobImageID("sha256:" + strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
}

func jobFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, part := range []string{"job", "common", "secret"} {
		if err := os.MkdirAll(filepath.Join(root, "job-runtime", "config", part), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range map[string]string{
		"job-runtime/config/job/config.yaml":    "application:\n  environment: prod\n  jobMode: ido\nchain:\n  project:\n    chainID: 56\n    url: https://rpc.example.invalid\n    v1:\n      enabled: 'false'\n    v2:\n      contractAddress: 0x0000000000000000000000000000000000000002\n      coreOperatorAddress: 0x0000000000000000000000000000000000000001\nfairLaunchMainnet:\n  scanner:\n    enabled: false\n    exclusiveJobMode: false\n",
		"job-runtime/config/common/config.yaml": "application: {}\n",
		"job-runtime/config/secret/config.yaml": "chain:\n  project:\n    jobPrivate: isolated-placeholder\n",
		"docker-compose.yml":                    "services:\n  job:\n    container_name: jc-job\n    image: joincare/job:reviewed\n    entrypoint: ['/app/joincare-job', '--path', '/app/config/']\n    volumes: ['./job-runtime/config:/app/config:ro']\n    healthcheck:\n      test: ['CMD', 'curl', '--fail', 'http://127.0.0.1:13007/health/job-ready']\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func resolvedJobFixture(t *testing.T, root, image, token string) string {
	t.Helper()
	fixture := map[string]any{
		"name":     "fixture",
		"networks": map[string]any{"default": map[string]any{"name": "fixture_default"}},
		"services": map[string]any{"job": map[string]any{
			"image": image, "container_name": "jc-job", "command": nil,
			"entrypoint":  []string{"/app/joincare-job", "--path", "/app/config/"},
			"labels":      map[string]string{"joincare.release.attempt": token},
			"healthcheck": map[string]any{"test": []string{"CMD", "curl", "--fail", "http://127.0.0.1:13007/health/job-ready"}},
			"networks":    map[string]any{"default": nil},
			"volumes":     []any{map[string]any{"type": "bind", "source": filepath.Join(root, "job-runtime", "config"), "target": "/app/config", "read_only": true}},
		}},
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func resolvedJobOverrideFixture(t *testing.T, root, token string, args []string) string {
	t.Helper()
	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) && strings.Contains(args[i+1], ".jc-job-image-") {
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				t.Fatal(err)
			}
			var parsed struct {
				Services struct {
					Job struct {
						Image string `yaml:"image"`
					} `yaml:"job"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				t.Fatal(err)
			}
			return resolvedJobFixture(t, root, parsed.Services.Job.Image, token)
		}
	}
	t.Fatal("missing Job image override")
	return ""
}

func TestResolvedJobRejectsInterpolatedPrivilegesAndMounts(t *testing.T) {
	root := jobFixture(t)
	image := "sha256:" + strings.Repeat("a", 64)
	token := "attempt-only"
	valid := resolvedJobFixture(t, root, image, token)
	if err := verifyResolvedJob(valid, root, image, token); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"backend bind":       strings.Replace(valid, `"target":"/app/config"`, `"target":"/app/core-config"`, 1),
		"writable bind":      strings.Replace(valid, `"read_only":true`, `"read_only":false`, 1),
		"extra bind":         strings.Replace(valid, `"volumes":[`, `"volumes":[{"type":"bind","source":"/opt/secrets","target":"/secrets","read_only":true},`, 1),
		"extra network":      strings.Replace(valid, `"networks":{"default":null}`, `"networks":{"default":null,"host":null}`, 1),
		"network alias":      strings.Replace(valid, `"networks":{"default":null}`, `"networks":{"default":{"aliases":["backend"]}}`, 1),
		"external network":   strings.Replace(valid, `"networks":{"default":{"name":"fixture_default"}}`, `"networks":{"default":{"name":"shared","external":true}}`, 1),
		"volume propagation": strings.Replace(valid, `"read_only":true`, `"read_only":true,"bind":{"propagation":"rshared"}`, 1),
		"health extension":   strings.Replace(valid, `"healthcheck":{"test":`, `"healthcheck":{"disable":true,"test":`, 1),
		"privileged":         strings.Replace(valid, `"container_name":"jc-job"`, `"container_name":"jc-job","privileged":true`, 1),
		"environment":        strings.Replace(valid, `"container_name":"jc-job"`, `"container_name":"jc-job","environment":{"CORE_KEY":"leak"}`, 1),
		"wrong image":        strings.Replace(valid, image, "sha256:"+strings.Repeat("b", 64), 1),
		"wrong label":        strings.Replace(valid, token, "other-attempt", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bad == valid || verifyResolvedJob(bad, root, image, token) == nil {
				t.Fatal("unsafe effective Compose accepted")
			}
		})
	}
}

func TestResolvedJobComposeV5NamedLogVolume(t *testing.T) {
	root := jobFixture(t)
	image := "sha256:" + strings.Repeat("a", 64)
	token := "attempt-only"
	var resolved map[string]any
	if err := json.Unmarshal([]byte(resolvedJobFixture(t, root, image, token)), &resolved); err != nil {
		t.Fatal(err)
	}
	resolved["name"] = "joincare-prod"
	resolved["networks"] = map[string]any{"default": map[string]any{"name": "joincare-prod_default", "ipam": map[string]any{}}}
	resolved["volumes"] = map[string]any{"job-logs": map[string]any{"name": "joincare-prod_job-logs"}}
	job := resolved["services"].(map[string]any)["job"].(map[string]any)
	job["volumes"] = []any{
		map[string]any{"type": "bind", "source": filepath.Join(root, "job-runtime", "config"), "target": "/app/config", "read_only": true, "bind": map[string]any{}},
		map[string]any{"type": "volume", "source": "job-logs", "target": "/app/logs", "volume": map[string]any{}},
	}
	check := func(want bool) {
		t.Helper()
		data, err := json.Marshal(resolved)
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyResolvedJob(string(data), root, image, token); (err == nil) != want {
			t.Fatalf("accepted=%t want=%t: %v", err == nil, want, err)
		}
	}
	check(true)
	declared := resolved["volumes"].(map[string]any)["job-logs"].(map[string]any)
	for _, bad := range []map[string]any{
		{"name": "shared_job-logs"},
		{"name": "joincare-prod_job-logs", "external": true},
	} {
		resolved["volumes"].(map[string]any)["job-logs"] = bad
		check(false)
	}
	resolved["volumes"].(map[string]any)["job-logs"] = declared
	logVolume := job["volumes"].([]any)[1].(map[string]any)
	logVolume["volume"] = map[string]any{"nocopy": true}
	check(false)
	logVolume["volume"] = map[string]any{}
	logVolume["source"] = "other-logs"
	check(false)
	logVolume["source"] = "job-logs"
	delete(resolved["volumes"].(map[string]any), "job-logs")
	check(false)
}

func TestJobInitialCatchupWaitsButKeepsHealthAndIdentity(t *testing.T) {
	if got := jobHealthTimeout(true); got != 30*time.Minute {
		t.Fatalf("initial catch-up timeout: %s", got)
	}
	if got := jobHealthTimeout(false); got != 90*time.Second {
		t.Fatalf("upgrade timeout: %s", got)
	}
	root := t.TempDir()
	image := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, inspect, wantErr string
	}{
		{"wrong image", "running starting sha256:" + strings.Repeat("b", 64) + " job " + root, "job_health_identity_changed"},
		{"wrong service", "running starting " + image + " backend " + root, "job_health_identity_changed"},
		{"wrong root", "running starting " + image + " job /another-root", "job_health_identity_changed"},
		{"exited", "exited unhealthy " + image + " job " + root, "job_health_container_stopped"},
		{"healthy", "running healthy " + image + " job " + root, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &jobStrategy{root: root, firstDeploy: true}
			s.command = func(_ context.Context, _ io.Writer, _ string, _ ...string) (string, error) {
				return tc.inspect, nil
			}
			err := s.healthCheckImage(context.Background(), image)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr) {
				t.Fatalf("health result: %v, want %q", err, tc.wantErr)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	s := &jobStrategy{root: root, firstDeploy: true}
	s.command = func(_ context.Context, _ io.Writer, _ string, _ ...string) (string, error) {
		return "running unhealthy " + image + " job " + root, nil
	}
	if err := s.healthCheckImage(ctx, image); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unhealthy Job must not pass readiness: %v", err)
	}
}

func TestJobFirstDeployStopsOnlyItsOwnContainerOnFailure(t *testing.T) {
	root := jobFixture(t)
	image := "sha256:" + strings.Repeat("a", 64)
	s := &jobStrategy{root: root}
	if err := s.SetJobImageID(image); err != nil {
		t.Fatal(err)
	}
	exists := false
	var mutations []string
	s.command = func(_ context.Context, _ io.Writer, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		if strings.Contains(line, " config --format json") {
			return resolvedJobOverrideFixture(t, root, s.firstDeployToken, args), nil
		}
		if strings.Contains(line, "container ls") {
			if exists {
				return "container-id\n", nil
			}
			return "", nil
		}
		if strings.Contains(line, "inspect --format {{.Image}} {{index") {
			if !exists {
				return "", errors.New("absent")
			}
			return image + " job " + root + " " + s.firstDeployToken + "\n", nil
		}
		if strings.Contains(line, "inspect --format {{.Image}} jc-job") {
			if !exists {
				return "", errors.New("absent")
			}
			return image + "\n", nil
		}
		if strings.Contains(line, "com.docker.compose.project.working_dir") && !strings.Contains(line, "State.Status") {
			return "job " + root + "\n", nil
		}
		if strings.Contains(line, "image inspect") {
			return image + "\n", nil
		}
		if strings.Contains(line, "inspect --format {{.Id}} jc-job") {
			return "container-id\n", nil
		}
		if strings.Contains(line, "compose") || strings.Contains(line, "docker rm -f container-id") {
			mutations = append(mutations, line)
			if strings.Contains(line, " up ") {
				exists = true
				return "", errors.New("simulated unhealthy container")
			}
			if strings.Contains(line, " rm ") || strings.Contains(line, "docker rm -f container-id") {
				exists = false
			}
		}
		return "", nil
	}
	s.shell = func(_ context.Context, _ io.Writer, line string) (string, error) {
		if strings.Contains(line, "docker load") {
			return "Loaded image ID: " + image + "\n", nil
		}
		t.Fatal("unexpected shell:", line)
		return "", nil
	}
	backup, err := s.Backup(context.Background(), filepath.Join(t.TempDir(), "backups"), io.Discard)
	if err != nil || backup != "job-absent" {
		t.Fatalf("first backup %q %v", backup, err)
	}
	if err := s.Apply(context.Background(), "signed.tar.gz", io.Discard); err == nil {
		t.Fatal("expected apply failure")
	}
	if err := s.Rollback(context.Background(), backup, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := s.RollbackHealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if exists || len(mutations) != 2 || !strings.Contains(mutations[0], "--no-deps --force-recreate job") || !strings.HasSuffix(mutations[1], "docker rm -f container-id") {
		t.Fatalf("job not safely stopped: exists=%v commands=%v", exists, mutations)
	}
}

func TestJobRefusesExistingUnidentifiableTarget(t *testing.T) {
	s := &jobStrategy{root: jobFixture(t)}
	s.command = func(_ context.Context, _ io.Writer, _ string, args ...string) (string, error) {
		line := strings.Join(args, " ")
		if strings.Contains(line, "container ls") {
			return "existing\n", nil
		}
		return "", errors.New("unavailable")
	}
	if _, err := s.Backup(context.Background(), t.TempDir(), io.Discard); err == nil {
		t.Fatal("invalid existing target treated as first deploy")
	}
}

func TestJobRefusesExistingForeignComposeBindingBeforeBackup(t *testing.T) {
	s := &jobStrategy{root: jobFixture(t)}
	image := "sha256:" + strings.Repeat("a", 64)
	saved := false
	s.command = func(_ context.Context, _ io.Writer, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(line, "inspect --format {{.Image}} jc-job"):
			return image + "\n", nil
		case strings.Contains(line, "com.docker.compose.project.working_dir"):
			return "job /opt/other-deployment\n", nil
		case strings.Contains(line, "docker save"):
			saved = true
		}
		return "", nil
	}
	if _, err := s.Backup(context.Background(), t.TempDir(), io.Discard); err == nil || saved {
		t.Fatalf("foreign Job backed up or admitted: %v, saved=%v", err, saved)
	}
}

func TestJobFirstDeployRefusesToRemoveContainerWithoutAttemptLabel(t *testing.T) {
	s := &jobStrategy{root: jobFixture(t), imageID: "sha256:" + strings.Repeat("a", 64), firstDeploy: true, applyAttempted: true, firstDeployToken: "this-attempt"}
	removed := false
	s.command = func(_ context.Context, _ io.Writer, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(line, "inspect --format {{.Image}} {{index"):
			return s.imageID + " job another-attempt\n", nil
		case strings.Contains(line, "docker rm"):
			removed = true
		}
		return "", nil
	}
	if err := s.Rollback(context.Background(), "job-absent", io.Discard); err == nil || removed {
		t.Fatalf("foreign container removed or accepted: %v, removed=%v", err, removed)
	}
}

func TestJobFirstDeployRefusesForeignProjectEvenWithMatchingAttempt(t *testing.T) {
	s := &jobStrategy{root: jobFixture(t), imageID: "sha256:" + strings.Repeat("a", 64), firstDeploy: true, applyAttempted: true, firstDeployToken: "this-attempt"}
	removed := false
	s.command = func(_ context.Context, _ io.Writer, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(line, "inspect --format {{.Image}} {{index"):
			return s.imageID + " job /opt/another-project " + s.firstDeployToken + "\n", nil
		case strings.Contains(line, "docker rm"):
			removed = true
		}
		return "", nil
	}
	if err := s.Rollback(context.Background(), "job-absent", io.Discard); err == nil || removed {
		t.Fatalf("foreign Compose project removed or accepted: %v removed=%v", err, removed)
	}
}

func TestJobFirstSuccessThenUpgradeFailureRestoresPreviousImage(t *testing.T) {
	root := jobFixture(t)
	first := "sha256:" + strings.Repeat("a", 64)
	second := "sha256:" + strings.Repeat("b", 64)
	current, loaded, attempted := "", "", ""
	failNext := false
	s := &jobStrategy{root: root}
	s.command = func(_ context.Context, _ io.Writer, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(line, " config --format json"):
			return resolvedJobOverrideFixture(t, root, s.firstDeployToken, args), nil
		case strings.Contains(line, "container ls"):
			if current == "" {
				return "", nil
			}
			return "existing-container\n", nil
		case strings.Contains(line, "inspect --format {{.Image}} jc-job"):
			if current == "" {
				return "", errors.New("absent")
			}
			return current + "\n", nil
		case strings.Contains(line, "inspect --format {{.Id}} {{.Image}} jc-job"):
			return "existing-container " + current + "\n", nil
		case strings.Contains(line, "inspect --format {{.Id}} jc-job"):
			return "existing-container\n", nil
		case strings.Contains(line, "inspect --format {{.Image}} {{index"):
			if current == "" {
				return "", errors.New("absent")
			}
			return current + " job " + root + " " + s.firstDeployToken + "\n", nil
		case strings.Contains(line, "com.docker.compose.project.working_dir") && !strings.Contains(line, "State.Status"):
			return "job " + root + "\n", nil
		case strings.Contains(line, "image inspect"):
			return loaded + "\n", nil
		case strings.Contains(line, "docker save -o"):
			if current != first || len(args) != 4 || args[3] != first {
				t.Fatalf("backup wrong image: %s", line)
			}
			return "", os.WriteFile(args[2], []byte("saved prior image"), 0600)
		case strings.Contains(line, "compose") && strings.Contains(line, " up "):
			if strings.Contains(line, "-f "+root+string(os.PathSeparator)+"docker-compose.yml") == false || !strings.Contains(line, "--no-deps --force-recreate job") {
				t.Fatalf("unsafe compose invocation: %s", line)
			}
			// Read the actual override: a rollback must pin the saved digest.
			// YAML override is inspected without relying on an attempted-image variable.
			for i, arg := range args {
				if arg == "-f" && i+1 < len(args) && strings.Contains(args[i+1], ".jc-job-image-") {
					data, err := os.ReadFile(args[i+1])
					if err != nil {
						t.Fatal(err)
					}
					var parsed map[string]any
					if err := yaml.Unmarshal(data, &parsed); err != nil {
						t.Fatal(err)
					}
					services := parsed["services"].(map[string]any)
					job := services["job"].(map[string]any)
					attempted = job["image"].(string)
				}
			}
			if failNext {
				failNext = false
				current = attempted // compose can have replaced a running Job before returning an error
				return "", errors.New("unhealthy upgrade")
			}
			current = attempted
			return "", nil
		case strings.Contains(line, "inspect --format {{.State.Status}}"):
			return "running healthy " + current + " job " + root + "\n", nil
		}
		return "", errors.New("unexpected Docker command: " + line)
	}
	s.shell = func(_ context.Context, _ io.Writer, line string) (string, error) {
		if strings.Contains(line, "docker load") {
			if strings.Contains(line, "job-"+first[7:]) {
				loaded = first
			} else {
				loaded = attempted
			}
			return "Loaded image ID: " + loaded + "\n", nil
		}
		return "", errors.New("unexpected shell command: " + line)
	}
	if err := s.SetJobImageID(first); err != nil {
		t.Fatal(err)
	}
	backup, err := s.Backup(context.Background(), t.TempDir(), io.Discard)
	if err != nil || backup != "job-absent" {
		t.Fatalf("first backup: %s %v", backup, err)
	}
	attempted = first
	if err := s.Apply(context.Background(), "first.tar", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := s.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.SetJobImageID(second); err != nil {
		t.Fatal(err)
	}
	backup, err = s.Backup(context.Background(), t.TempDir(), io.Discard)
	if err != nil || backup == "" || backup == "job-absent" {
		t.Fatalf("upgrade backup: %s %v", backup, err)
	}
	attempted = second
	failNext = true
	if err := s.Apply(context.Background(), "second.tar", io.Discard); err == nil {
		t.Fatal("expected upgrade failure")
	}
	if err := s.Rollback(context.Background(), backup, io.Discard); err != nil {
		t.Fatal(err)
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.RollbackHealthCheck(checkCtx); err != nil || current != first {
		t.Fatalf("old Job not restored: image=%s err=%v", current, err)
	}
}
