package strategies

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// CheckIDOJobLayout checks only the static part of a future Job
// release. It is NOT runtime admission: signer separation, chain identity,
// schema, historical RPC, backup and readiness must also be verified before
// an agent is ever allowed to mutate the container. In particular, never
// bootstrap a Job by copying the Backend container's binds/environment.
func CheckIDOJobLayout(root string) error {
	// The Backend mounts root/config, including its Core operator secret. A Job
	// cannot mount a subdirectory of that tree, and must use the same three
	// paths that cmd/job actually reads: job, common and secret/config.yaml.
	configRoot := filepath.Join(root, "job-runtime", "config")
	for _, dir := range []string{root, filepath.Join(root, "job-runtime"), configRoot} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0022 != 0) {
			return errors.New("ido_job_config_unavailable")
		}
	}
	for _, component := range []string{"job", "common", "secret"} {
		dir := filepath.Join(configRoot, component)
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return errors.New("ido_job_config_unavailable")
		}
		file, err := os.Lstat(filepath.Join(dir, "config.yaml"))
		if err != nil || !file.Mode().IsRegular() || (runtime.GOOS != "windows" && file.Mode().Perm()&0077 != 0) {
			return errors.New("ido_job_config_unavailable")
		}
	}
	// The source configuration tree itself must not be writable by the
	// deployment group: a read-only container mount does not protect the host
	// from a later change between preflight and `docker compose up`.
	if runtime.GOOS != "windows" {
		for _, dir := range []string{filepath.Join(root, "job-runtime"), configRoot,
			filepath.Join(configRoot, "job"), filepath.Join(configRoot, "common"), filepath.Join(configRoot, "secret")} {
			info, err := os.Lstat(dir)
			if err != nil || info.Mode().Perm()&0022 != 0 {
				return errors.New("ido_job_config_permissions_unsafe")
			}
		}
	}
	configPath := filepath.Join(configRoot, "job", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return errors.New("ido_job_config_unavailable")
	}
	// Only inspect YAML keys, never include the contents of either secret file
	// in diagnostics. Explicitly refuse the Core relayer key in the Job tree.
	secret, err := os.ReadFile(filepath.Join(configRoot, "secret", "config.yaml"))
	if err != nil {
		return errors.New("ido_job_secret_unavailable")
	}
	var secretConfig struct {
		Chain struct {
			Project struct {
				JobPrivate string `yaml:"jobPrivate"`
				Private    string `yaml:"private"`
			} `yaml:"project"`
		} `yaml:"chain"`
	}
	if err := yaml.Unmarshal(secret, &secretConfig); err != nil ||
		secretConfig.Chain.Project.JobPrivate == "" || secretConfig.Chain.Project.Private != "" {
		return errors.New("ido_job_signer_not_isolated")
	}
	// cmd/job merges job, then common, then secret via Viper. A later file can
	// override a safe job role even when job/config.yaml itself looks safe.
	// Refuse identity keys outside job/config.yaml and signer keys outside the
	// isolated secret, rather than trusting a single file's apparent values.
	if err := rejectJobIdentityOverrides(filepath.Join(configRoot, "common", "config.yaml"), true); err != nil {
		return err
	}
	if err := rejectJobIdentityOverrides(filepath.Join(configRoot, "secret", "config.yaml"), false); err != nil {
		return err
	}
	if err := rejectJobSignerKeys(filepath.Join(configRoot, "job", "config.yaml")); err != nil {
		return err
	}
	// Viper folds casing across all three files. A typed YAML decode may see
	// `application` but miss a second `APPLICATION` that Viper merges over it.
	// Require one exact spelling for each security-critical key path.
	jobMap, err := readJobConfigMap(configPath)
	if err != nil {
		return err
	}
	for _, keys := range [][]string{
		{"application", "environment"}, {"application", "jobMode"},
		{"chain", "project", "v1", "enabled"},
		{"chain", "project", "v2", "coreOperatorAddress"},
		{"chain", "project", "v2", "contractAddress"},
		{"chain", "project", "chainID"},
		{"chain", "project", "url"},
		{"fairLaunchMainnet", "scanner", "enabled"},
		{"fairLaunchMainnet", "scanner", "exclusiveJobMode"},
	} {
		if !hasOnlyCanonicalJobPath(jobMap, keys...) {
			return errors.New("ido_job_identity_ambiguous")
		}
	}
	var config struct {
		Application struct {
			Environment string `yaml:"environment"`
			JobMode     string `yaml:"jobMode"`
		} `yaml:"application"`
		Chain struct {
			Project struct {
				ChainID uint64 `yaml:"chainID"`
				URL     string `yaml:"url"`
				V1      struct {
					Enabled string `yaml:"enabled"`
				} `yaml:"v1"`
				V2 struct {
					ContractAddress string `yaml:"contractAddress"`
				} `yaml:"v2"`
			} `yaml:"project"`
		} `yaml:"chain"`
		FairLaunch struct {
			Scanner struct {
				Enabled   bool `yaml:"enabled"`
				Exclusive bool `yaml:"exclusiveJobMode"`
			} `yaml:"scanner"`
		} `yaml:"fairLaunchMainnet"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return errors.New("ido_job_config_invalid")
	}
	if config.Application.Environment != "prod" || (config.Application.JobMode != "ido" && config.Application.JobMode != "ido-v3") ||
		config.Chain.Project.ChainID != 56 || strings.TrimSpace(config.Chain.Project.URL) == "" ||
		!validJobAddress(config.Chain.Project.V2.ContractAddress) ||
		config.Chain.Project.V1.Enabled != "false" || config.FairLaunch.Scanner.Enabled || config.FairLaunch.Scanner.Exclusive {
		return errors.New("ido_job_role_not_admitted")
	}

	composePath := filepath.Join(root, "docker-compose.yml")
	info, err := os.Lstat(composePath)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0022 != 0) {
		return errors.New("ido_job_compose_unavailable")
	}
	data, err = os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("read ido job compose: %w", err)
	}
	var compose struct {
		Services struct {
			Job struct {
				Image         string   `yaml:"image"`
				ContainerName string   `yaml:"container_name"`
				Entrypoint    []string `yaml:"entrypoint"`
				Volumes       []string `yaml:"volumes"`
				Environment   any      `yaml:"environment"`
				EnvFile       any      `yaml:"env_file"`
				Healthcheck   struct {
					Test        []string `yaml:"test"`
					Interval    string   `yaml:"interval"`
					Timeout     string   `yaml:"timeout"`
					Retries     *int     `yaml:"retries"`
					StartPeriod string   `yaml:"start_period"`
				} `yaml:"healthcheck"`
			} `yaml:"job"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return errors.New("ido_job_compose_invalid")
	}
	// Decode the effective YAML mapping as well: yaml.v3 expands merge keys
	// and aliases here. A permissive typed struct silently ignores e.g.
	// privileged, network_mode, build, extra mounts and container labels.
	var effective struct {
		Services map[string]map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &effective); err != nil {
		return errors.New("ido_job_compose_invalid")
	}
	allowedJobFields := map[string]bool{
		"image": true, "container_name": true, "entrypoint": true,
		"volumes": true, "healthcheck": true,
	}
	for field := range effective.Services["job"] {
		if !allowedJobFields[field] {
			return errors.New("ido_job_compose_field_not_admitted")
		}
	}
	// A permissive healthcheck mapping could hide test_disable or a custom
	// start_interval while the typed struct silently ignores it.
	if raw, ok := effective.Services["job"]["healthcheck"]; ok {
		fields, valid := raw.(map[string]any)
		if !valid {
			return errors.New("ido_job_readiness_unavailable")
		}
		for field := range fields {
			switch field {
			case "test", "interval", "timeout", "retries", "start_period":
			default:
				return errors.New("ido_job_readiness_unavailable")
			}
		}
	}
	// Only Job has this deliberately narrow service shape. Sibling services
	// may use valid Compose long mounts or scalar entrypoints; they neither
	// grant Job authority nor need to fit the Job-only typed decoder.
	job := compose.Services.Job
	_, ok := effective.Services["job"]
	if !ok || job.Image == "" || job.ContainerName != "jc-job" || len(job.Entrypoint) != 3 || job.Entrypoint[0] != "/app/joincare-job" ||
		job.Entrypoint[1] != "--path" || job.Entrypoint[2] != "/app/config/" ||
		job.Environment != nil || job.EnvFile != nil {
		return errors.New("ido_job_service_unavailable")
	}
	// A shared Backend config mount would expose the Core operator key. The
	// Job needs an independent, read-only config path and its own health gate.
	configMount := false
	for _, volume := range job.Volumes {
		parts := strings.Split(volume, ":")
		if len(parts) == 3 && parts[0] == "./job-runtime/config" && parts[1] == "/app/config" && parts[2] == "ro" && !configMount {
			configMount = true
			continue
		}
		// Refuse other host paths, including Backend secrets mounted at a
		// different destination. Only the dedicated named log volume is safe.
		if len(parts) != 2 || parts[0] != "job-logs" || parts[1] != "/app/logs" {
			return errors.New("ido_job_config_mount_not_isolated")
		}
	}
	if !configMount || !equalJobHealthCommand(job.Healthcheck.Test) ||
		job.Healthcheck.Retries != nil && *job.Healthcheck.Retries < 1 {
		return errors.New("ido_job_readiness_unavailable")
	}
	return nil
}

func validJobAddress(value string) bool {
	if len(value) != 42 || !strings.HasPrefix(value, "0x") || strings.EqualFold(value, "0x"+strings.Repeat("0", 40)) {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil
}

func equalJobHealthCommand(command []string) bool {
	want := []string{"CMD", "curl", "--fail", "http://127.0.0.1:13007/health/job-ready"}
	if len(command) != len(want) {
		return false
	}
	for i := range want {
		if command[i] != want[i] {
			return false
		}
	}
	return true
}

func readJobConfigMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("ido_job_config_unavailable")
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, errors.New("ido_job_config_invalid")
	}
	return values, nil
}

func hasJobKey(values map[string]any, path ...string) bool {
	if len(path) == 0 {
		return false
	}
	// Viper normalizes key casing. Inspect *all* spellings: selecting the
	// first case-insensitive Go map entry can miss a malicious second value.
	for name, value := range values {
		if !strings.EqualFold(name, path[0]) {
			continue
		}
		if len(path) == 1 {
			return true
		}
		child, ok := value.(map[string]any)
		if !ok || hasJobKey(child, path[1:]...) {
			return true // malformed parent cannot be treated as safe
		}
	}
	return false
}

func hasOnlyCanonicalJobPath(values map[string]any, path ...string) bool {
	if len(path) == 0 {
		return true
	}
	matches := 0
	var child any
	for key, value := range values {
		if strings.EqualFold(key, path[0]) {
			if key != path[0] {
				return false
			}
			matches++
			child = value
		}
	}
	if matches != 1 {
		return false
	}
	if len(path) == 1 {
		return child != nil
	}
	inner, ok := child.(map[string]any)
	return ok && hasOnlyCanonicalJobPath(inner, path[1:]...)
}

func rejectJobSignerKeys(path string) error {
	values, err := readJobConfigMap(path)
	if err != nil {
		return err
	}
	if hasJobKey(values, "chain", "project", "private") || hasJobKey(values, "chain", "project", "jobPrivate") {
		return errors.New("ido_job_signer_not_isolated")
	}
	return nil
}

func rejectJobIdentityOverrides(path string, commonConfig bool) error {
	values, err := readJobConfigMap(path)
	if err != nil {
		return err
	}
	for _, keys := range [][]string{
		{"application", "environment"}, {"application", "jobMode"},
		{"chain", "project", "v1", "enabled"},
		{"chain", "project", "v2", "coreOperatorAddress"},
		{"chain", "project", "v2", "contractAddress"},
		{"chain", "project", "chainID"},
		{"chain", "project", "url"},
		{"fairLaunchMainnet", "scanner", "enabled"},
		{"fairLaunchMainnet", "scanner", "exclusiveJobMode"},
		{"chain", "project", "private"},
	} {
		if hasJobKey(values, keys...) {
			return errors.New("ido_job_identity_override")
		}
	}
	if commonConfig && hasJobKey(values, "chain", "project", "jobPrivate") {
		return errors.New("ido_job_signer_not_isolated")
	}
	return nil
}
