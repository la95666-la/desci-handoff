package strategies

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joincare/jc-release/internal/shared"
)

// jobStrategy only handles the dedicated IDO Job service. Agent admission
// supplies a checksum-bound image identity; no Backend credentials are copied.
type jobStrategy struct {
	root                string
	imageID             string
	loadedID            string // Docker may expose the OCI manifest digest instead of the config digest.
	previousID          string
	previousContainerID string
	firstDeploy         bool
	applyAttempted      bool
	firstDeployToken    string
	retainedOriginal    bool
	command             func(context.Context, io.Writer, string, ...string) (string, error)
	shell               func(context.Context, io.Writer, string) (string, error)
}

func (s *jobStrategy) cmd(ctx context.Context, log io.Writer, name string, args ...string) (string, error) {
	if s.command != nil {
		return s.command(ctx, log, name, args...)
	}
	return runCmd(ctx, log, name, args...)
}
func (s *jobStrategy) pipe(ctx context.Context, log io.Writer, line string) (string, error) {
	if s.shell != nil {
		return s.shell(ctx, log, line)
	}
	return runShell(ctx, log, line)
}

func (*jobStrategy) Component() shared.Component { return shared.CompJob }

func (s *jobStrategy) existingImage(ctx context.Context) (string, error) {
	out, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Image}}", "jc-job")
	if err != nil || !validImageID(strings.TrimSpace(out)) {
		return "", errors.New("job_existing_rollback_target_unavailable")
	}
	return strings.TrimSpace(out), nil
}

// An image and a reusable container name do not prove that an existing Job
// belongs to this deployment. Refuse to replace a foreign or legacy container.
func (s *jobStrategy) existingBinding(ctx context.Context) error {
	out, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{index .Config.Labels \"com.docker.compose.service\"}} {{index .Config.Labels \"com.docker.compose.project.working_dir\"}}", "jc-job")
	if err != nil || strings.TrimSpace(out) != "job "+filepath.Clean(s.root) {
		return errors.New("job_existing_compose_binding_unavailable")
	}
	return nil
}

func (s *jobStrategy) Backup(ctx context.Context, dir string, log io.Writer) (string, error) {
	s.applyAttempted = false
	s.firstDeployToken = ""
	s.firstDeploy = false
	s.previousID = ""
	s.previousContainerID = ""
	s.retainedOriginal = false
	s.loadedID = ""
	if err := CheckIDOJobLayout(s.root); err != nil {
		return "", err
	}
	id, err := s.existingImage(ctx)
	if err != nil {
		// Only an independently confirmed ABSENT container qualifies. Docker
		// daemon errors and malformed inspect output must not bootstrap Job.
		out, listErr := s.cmd(ctx, io.Discard, "docker", "container", "ls", "-a", "--filter", "name=^/jc-job$", "--format", "{{.ID}}")
		if listErr != nil || strings.TrimSpace(out) != "" {
			return "", errors.New("job_existing_rollback_target_unavailable")
		}
		id, err = s.existingImage(ctx)
		if err == nil {
			return "", errors.New("job_rollback_target_changed")
		}
		out, listErr = s.cmd(ctx, io.Discard, "docker", "container", "ls", "-a", "--filter", "name=^/jc-job$", "--format", "{{.ID}}")
		if listErr != nil || strings.TrimSpace(out) != "" {
			return "", errors.New("job_rollback_target_changed")
		}
		s.previousID = ""
		s.firstDeploy = true
		return "job-absent", nil
	}
	s.firstDeploy = false
	if err := s.existingBinding(ctx); err != nil {
		return "", err
	}
	container, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Id}}", "jc-job")
	if err != nil || strings.TrimSpace(container) == "" {
		return "", errors.New("job_existing_container_identity_unavailable")
	}
	s.previousContainerID = strings.TrimSpace(container)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("job-%s-%d.tar", id[7:], time.Now().UTC().UnixNano()))
	if _, err := s.cmd(ctx, log, "docker", "save", "-o", path, id); err != nil {
		os.Remove(path)
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		os.Remove(path)
		return "", errors.New("job_backup_empty")
	}
	s.previousID = id
	return path, nil
}

func (s *jobStrategy) Apply(ctx context.Context, src string, log io.Writer) error {
	if err := CheckIDOJobLayout(s.root); err != nil {
		return err
	}
	if !validImageID(s.imageID) || (!s.firstDeploy && !validImageID(s.previousID)) {
		return errors.New("job_image_identity_unverified")
	}
	if s.firstDeploy {
		out, err := s.cmd(ctx, io.Discard, "docker", "container", "ls", "-a", "--filter", "name=^/jc-job$", "--format", "{{.ID}}")
		if err != nil || strings.TrimSpace(out) != "" {
			return errors.New("job_first_deploy_target_changed")
		}
	} else if current, err := s.existingImage(ctx); err != nil || current != s.previousID {
		return errors.New("job_rollback_target_changed")
	}
	if !s.firstDeploy {
		if err := s.existingBinding(ctx); err != nil {
			return err
		}
		container, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Id}}", "jc-job")
		if err != nil || strings.TrimSpace(container) != s.previousContainerID {
			return errors.New("job_rollback_target_changed")
		}
	}
	loadOutput, err := s.pipe(ctx, log, fmt.Sprintf("docker load < %q", src))
	if err != nil {
		return err
	}
	loadedRef := parseLoadedImage(loadOutput)
	if loadedRef == "" {
		loadedRef = parseLoadedImageID(loadOutput)
	}
	if loadedRef == "" {
		return errors.New("job_loaded_image_identity_missing")
	}
	loaded, err := s.cmd(ctx, log, "docker", "image", "inspect", "--format", "{{.Id}}", loadedRef)
	if err != nil || !validImageID(strings.TrimSpace(loaded)) {
		return errors.New("job_loaded_image_mismatch")
	}
	if (strings.HasPrefix(loadedRef, "sha256:") && loadedRef != s.imageID) ||
		(!strings.HasPrefix(loadedRef, "sha256:") && !strings.HasPrefix(loadedRef, "joincare/job:")) {
		return errors.New("job_loaded_image_mismatch")
	}
	s.loadedID = strings.TrimSpace(loaded)
	if s.loadedID != s.imageID {
		// Docker with the containerd image store reports the OCI manifest
		// digest as .Id, while the server binds the image config digest.
		// Only accept a manifest whose bytes and config pointer are verified
		// against the checksum-bound archive; never trust the tag alone.
		manifestID, err := jobOCIManifestID(src, loadedRef, s.imageID)
		if err != nil || s.loadedID != manifestID {
			s.loadedID = ""
			return errors.New("job_loaded_image_mismatch")
		}
	}
	return s.recreate(ctx, s.loadedID, log)
}

func (s *jobStrategy) recreate(ctx context.Context, imageID string, log io.Writer) error {
	if err := CheckIDOJobLayout(s.root); err != nil {
		return err
	}
	if !validImageID(imageID) {
		return errors.New("job_image_identity_invalid")
	}
	// A temporary override pins the exact immutable image ID, rather than
	// mutating an existing tag. Only the job service is recreated.
	override, err := os.CreateTemp(s.root, ".jc-job-image-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(override.Name())
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		override.Close()
		return err
	}
	s.firstDeployToken = hex.EncodeToString(nonce[:])
	if _, err := fmt.Fprintf(override, "services:\n  job:\n    image: %s\n", imageID); err != nil {
		override.Close()
		return err
	}
	if _, err := fmt.Fprintf(override, "    labels:\n      joincare.release.attempt: %q\n", s.firstDeployToken); err != nil {
		override.Close()
		return err
	}
	if err := override.Close(); err != nil {
		return err
	}
	// YAML-only checks cannot see Docker Compose interpolation, includes or
	// effective mounts. Check the resolved Job service with the exact override
	// that the subsequent up will use, before touching a container.
	resolved, err := s.cmd(ctx, io.Discard, "docker", "compose", "-f", filepath.Join(s.root, "docker-compose.yml"), "-f", override.Name(), "config", "--format", "json")
	if err != nil || verifyResolvedJob(resolved, s.root, imageID, s.firstDeployToken) != nil {
		return errors.New("job_effective_compose_unavailable")
	}
	s.applyAttempted = true
	_, err = s.cmd(ctx, log, "docker", "compose", "-f", filepath.Join(s.root, "docker-compose.yml"), "-f", override.Name(), "up", "-d", "--no-deps", "--force-recreate", "job")
	return err
}

func verifyResolvedJob(data, root, imageID, token string) error {
	// Refuse unknown effective Job fields instead of trusting the typed subset;
	// a new Compose version must be reviewed before it can mutate Job.
	var document struct {
		Name     string                                `json:"name"`
		Services map[string]map[string]json.RawMessage `json:"services"`
		Networks map[string]json.RawMessage            `json:"networks"`
		Volumes  map[string]json.RawMessage            `json:"volumes"`
	}
	if json.Unmarshal([]byte(data), &document) != nil {
		return errors.New("invalid effective compose")
	}
	job := document.Services["job"]
	// Docker Compose JSON may grow nested options independently of the YAML
	// source. A typed decode alone would silently discard dangerous options.
	if len(job) == 0 || !onlyJSONFields(job["healthcheck"], "test", "interval", "timeout", "retries", "start_period") {
		return errors.New("unsafe effective Job healthcheck")
	}
	var rawVolumes []map[string]json.RawMessage
	if json.Unmarshal(job["volumes"], &rawVolumes) != nil || len(rawVolumes) == 0 {
		return errors.New("unsafe effective Job volumes")
	}
	for _, raw := range rawVolumes {
		// Compose v5 includes an empty "volume" object for named volumes.
		// The source is still checked against the project-scoped declaration below.
		if !onlyJSONMapFields(raw, "type", "source", "target", "read_only", "bind", "volume") {
			return errors.New("unsafe effective Job volume option")
		}
		if bind, ok := raw["bind"]; ok && !onlyJSONFields(bind, "create_host_path") {
			return errors.New("unsafe effective Job bind option")
		}
		if volume, ok := raw["volume"]; ok && !jsonNullOrEmptyObject(volume) {
			return errors.New("unsafe effective Job volume option")
		}
	}
	var rawNetworks map[string]json.RawMessage
	if json.Unmarshal(job["networks"], &rawNetworks) != nil || len(rawNetworks) != 1 ||
		!jsonNullOrEmptyObject(rawNetworks["default"]) {
		return errors.New("unsafe effective Job network options")
	}
	if !onlyJSONFields(document.Networks["default"], "name", "ipam") {
		return errors.New("unsafe effective default network")
	}
	var network struct {
		Name string          `json:"name"`
		IPAM json.RawMessage `json:"ipam"`
	}
	if json.Unmarshal(document.Networks["default"], &network) != nil || document.Name == "" ||
		network.Name != document.Name+"_default" ||
		len(network.IPAM) != 0 && !jsonNullOrEmptyObject(network.IPAM) {
		return errors.New("unsafe effective default network identity")
	}
	allowed := map[string]bool{"image": true, "container_name": true, "entrypoint": true, "command": true,
		"volumes": true, "healthcheck": true, "networks": true, "labels": true}
	for key := range job {
		if !allowed[key] {
			return errors.New("unsafe effective Job field")
		}
	}
	var service struct {
		Image         string            `json:"image"`
		ContainerName string            `json:"container_name"`
		Entrypoint    []string          `json:"entrypoint"`
		Command       any               `json:"command"`
		Labels        map[string]string `json:"labels"`
		Healthcheck   struct {
			Test        []string `json:"test"`
			Interval    string   `json:"interval"`
			Timeout     string   `json:"timeout"`
			Retries     *int     `json:"retries"`
			StartPeriod string   `json:"start_period"`
		} `json:"healthcheck"`
		Networks map[string]any `json:"networks"`
		Volumes  []struct {
			Type     string `json:"type"`
			Source   string `json:"source"`
			Target   string `json:"target"`
			ReadOnly bool   `json:"read_only"`
			Bind     struct {
				CreateHostPath bool `json:"create_host_path"`
			} `json:"bind"`
		} `json:"volumes"`
	}
	jobData, err := json.Marshal(job)
	if err != nil || len(job) == 0 || json.Unmarshal(jobData, &service) != nil ||
		service.Image != imageID || service.ContainerName != "jc-job" || service.Command != nil ||
		len(service.Entrypoint) != 3 || service.Entrypoint[0] != "/app/joincare-job" ||
		service.Entrypoint[1] != "--path" || service.Entrypoint[2] != "/app/config/" ||
		!equalJobHealthCommand(service.Healthcheck.Test) || service.Healthcheck.Retries != nil && *service.Healthcheck.Retries < 1 ||
		len(service.Labels) != 1 || service.Labels["joincare.release.attempt"] != token || token == "" ||
		len(service.Networks) != 1 || len(document.Networks) != 1 ||
		len(service.Volumes) == 0 || len(service.Volumes) > 2 {
		return errors.New("effective Job identity mismatch")
	}
	if _, ok := service.Networks["default"]; !ok {
		return errors.New("effective Job network not isolated")
	}
	if _, ok := document.Networks["default"]; !ok {
		return errors.New("effective Job default network unavailable")
	}
	if _, ok := document.Volumes["job-logs"]; ok {
		var declared struct {
			Name string `json:"name"`
		}
		if !onlyJSONFields(document.Volumes["job-logs"], "name") ||
			json.Unmarshal(document.Volumes["job-logs"], &declared) != nil ||
			declared.Name != document.Name+"_job-logs" {
			return errors.New("unsafe effective Job log volume declaration")
		}
	}
	seenConfig, seenLogs := false, false
	for _, volume := range service.Volumes {
		switch {
		case volume.Type == "bind" && volume.Source == filepath.Join(root, "job-runtime", "config") &&
			volume.Target == "/app/config" && volume.ReadOnly && !seenConfig:
			seenConfig = true
		case volume.Type == "volume" && volume.Target == "/app/logs" && !seenLogs &&
			(volume.Source == "job-logs" && document.Volumes["job-logs"] != nil ||
				volume.Source == document.Name+"_job-logs" && document.Volumes["job-logs"] == nil):
			seenLogs = true
		default:
			return errors.New("unsafe effective Job volume")
		}
	}
	if !seenConfig {
		return errors.New("effective Job config missing")
	}
	return nil
}

func onlyJSONFields(raw json.RawMessage, fields ...string) bool {
	var mapping map[string]json.RawMessage
	return json.Unmarshal(raw, &mapping) == nil && mapping != nil && onlyJSONMapFields(mapping, fields...)
}

func onlyJSONMapFields(mapping map[string]json.RawMessage, fields ...string) bool {
	for key := range mapping {
		found := false
		for _, allowed := range fields {
			if key == allowed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func jsonNullOrEmptyObject(raw json.RawMessage) bool {
	return string(raw) == "null" || string(raw) == "{}"
}

func (s *jobStrategy) Rollback(ctx context.Context, backup string, log io.Writer) error {
	if err := CheckIDOJobLayout(s.root); err != nil {
		return err
	}
	if s.firstDeploy && backup == "job-absent" {
		// A new Job has no prior image. Remove only a container produced by
		// this attempt, never a container placed there by another actor.
		if !s.applyAttempted {
			return nil
		}
		out, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Image}} {{index .Config.Labels \"com.docker.compose.service\"}} {{index .Config.Labels \"com.docker.compose.project.working_dir\"}} {{index .Config.Labels \"joincare.release.attempt\"}}", "jc-job")
		if err != nil {
			absent, listErr := s.cmd(ctx, io.Discard, "docker", "container", "ls", "-a", "--filter", "name=^/jc-job$", "--format", "{{.ID}}")
			if listErr != nil || strings.TrimSpace(absent) != "" {
				return errors.New("job_first_deploy_stop_target_unverified")
			}
			return nil
		}
		if s.firstDeployToken == "" || strings.TrimSpace(out) != s.loadedID+" job "+filepath.Clean(s.root)+" "+s.firstDeployToken {
			return errors.New("job_first_deploy_stop_target_changed")
		}
		// Confirm the runtime binding, not just an image that happens to match.
		container, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Id}}", "jc-job")
		if err != nil || strings.TrimSpace(container) == "" {
			return errors.New("job_first_deploy_stop_target_unverified")
		}
		// Remove the inspected immutable container ID, never the reusable name.
		_, err = s.cmd(ctx, log, "docker", "rm", "-f", strings.TrimSpace(container))
		return err
	}
	if !validImageID(s.previousID) || backup == "" || !strings.HasPrefix(filepath.Base(backup), "job-"+s.previousID[7:]+"-") {
		return ErrNoBackup
	}
	if s.applyAttempted {
		s.retainedOriginal = false
		out, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Image}} {{index .Config.Labels \"com.docker.compose.service\"}} {{index .Config.Labels \"com.docker.compose.project.working_dir\"}} {{index .Config.Labels \"joincare.release.attempt\"}}", "jc-job")
		if err != nil || s.firstDeployToken == "" || strings.TrimSpace(out) != s.loadedID+" job "+filepath.Clean(s.root)+" "+s.firstDeployToken {
			// Compose might fail before replacing the original. It is safe to
			// retain only the exact original container, never an unknown Job.
			current, currentErr := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Id}} {{.Image}}", "jc-job")
			if currentErr == nil && strings.TrimSpace(current) == s.previousContainerID+" "+s.previousID {
				s.retainedOriginal = true
				return nil
			}
			return errors.New("job_rollback_target_changed")
		}
	}
	if !s.applyAttempted {
		if err := s.existingBinding(ctx); err != nil {
			return err
		}
	}
	info, err := os.Stat(backup)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ErrNoBackup
	}
	loadOutput, err := s.pipe(ctx, log, fmt.Sprintf("docker load < %q", backup))
	if err != nil {
		return err
	}
	loadedRef := parseLoadedImage(loadOutput)
	if loadedRef == "" {
		loadedRef = parseLoadedImageID(loadOutput)
	}
	if loadedRef == "" {
		return errors.New("job_backup_image_identity_missing")
	}
	loaded, err := s.cmd(ctx, log, "docker", "image", "inspect", "--format", "{{.Id}}", loadedRef)
	if err != nil || strings.TrimSpace(loaded) != s.previousID {
		return errors.New("job_backup_image_unavailable")
	}
	return s.recreate(ctx, s.previousID, log)
}

func (s *jobStrategy) HealthCheck(ctx context.Context) error {
	return s.healthCheckImage(ctx, s.loadedID)
}

func (s *jobStrategy) healthCheckImage(ctx context.Context, imageID string) error {
	// Initial catch-up is a durable scanner operation, not an instantaneous
	// startup probe. In particular, rate-limited RPC may require multiple
	// one-minute scanner runs before lag falls below the readiness threshold.
	// Upgrades of an already-running Job retain the short rollback window.
	deadline := time.NewTimer(jobHealthTimeout(s.firstDeploy))
	defer deadline.Stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		out, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.Image}} {{index .Config.Labels \"com.docker.compose.service\"}} {{index .Config.Labels \"com.docker.compose.project.working_dir\"}}", "jc-job")
		if err == nil {
			fields := strings.Fields(out)
			if len(fields) == 5 {
				if fields[2] != imageID || fields[3] != "job" || fields[4] != filepath.Clean(s.root) {
					return errors.New("job_health_identity_changed")
				}
				if fields[0] != "running" {
					return errors.New("job_health_container_stopped")
				}
				if fields[1] == "healthy" {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("job_readiness_unavailable")
		case <-tick.C:
		}
	}
}

func jobHealthTimeout(firstDeploy bool) time.Duration {
	if firstDeploy {
		return 30 * time.Minute
	}
	return 90 * time.Second
}

// RollbackHealthCheck verifies the safe, stopped terminal state for a first
// deployment. Upgrader uses this instead of requiring a now-absent Job healthy.
func (s *jobStrategy) RollbackHealthCheck(ctx context.Context) error {
	if !s.firstDeploy {
		if s.retainedOriginal {
			current, err := s.cmd(ctx, io.Discard, "docker", "inspect", "--format", "{{.Id}} {{.Image}}", "jc-job")
			if err != nil || strings.TrimSpace(current) != s.previousContainerID+" "+s.previousID {
				return errors.New("job_original_container_changed")
			}
		}
		return s.healthCheckImage(ctx, s.previousID)
	}
	out, err := s.cmd(ctx, io.Discard, "docker", "container", "ls", "-a", "--filter", "name=^/jc-job$", "--format", "{{.ID}}")
	if err != nil || strings.TrimSpace(out) != "" {
		return errors.New("job_first_deploy_stop_unverified")
	}
	return nil
}

func validImageID(id string) bool {
	if !strings.HasPrefix(id, "sha256:") || len(id) != 71 {
		return false
	}
	for _, c := range id[7:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func parseLoadedImageID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Loaded image ID: ") {
			id := strings.TrimSpace(strings.TrimPrefix(line, "Loaded image ID: "))
			if validImageID(id) {
				return id
			}
		}
	}
	return ""
}

// SetJobImageID is called only after signed artifact verification.
func (s *jobStrategy) SetJobImageID(id string) error {
	if !validImageID(id) {
		return errors.New("job_image_identity_invalid")
	}
	s.imageID = id
	return nil
}
