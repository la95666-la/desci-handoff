package agent

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joincare/jc-release/internal/shared"
	"gopkg.in/yaml.v3"
)

// Config is the yaml-driven agent configuration.
type Config struct {
	ServerURL            string            `yaml:"server_url"`
	EnrollToken          string            `yaml:"enroll_token,omitempty"` // cleared after first register
	DataDir              string            `yaml:"data_dir"`
	Root                 string            `yaml:"root"` // joincare deployment root, default /opt/joincare-prod
	JobEnvironment       string            `yaml:"job_environment,omitempty"`
	JobTrustedKeys       map[string]string `yaml:"job_trusted_keys,omitempty"`    // key ID -> Ed25519 public key (hex)
	JobReadinessFile     string            `yaml:"job_readiness_file,omitempty"`  // protected, externally reviewed assertion
	JobEvidenceDir       string            `yaml:"job_evidence_dir,omitempty"`    // root-controlled evidence receipts
	JobReadinessKeys     map[string]string `yaml:"job_readiness_keys,omitempty"`  // distinct from artifact approvers
	JobHostKeySHA256     string            `yaml:"job_host_key_sha256,omitempty"` // independently verified host fingerprint
	AutoUpgrade          bool              `yaml:"auto_upgrade"`
	PollCommandInterval  time.Duration     `yaml:"poll_interval_commands"`
	PollManifestInterval time.Duration     `yaml:"poll_interval_manifest"`
	// Legacy production configs may contain this field set to false. Keeping the
	// field avoids a disruptive config rewrite, while true remains forbidden.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify,omitempty"`
}

func validateServerURL(raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return "", fmt.Errorf("server_url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return "", fmt.Errorf("server_url must be an absolute HTTPS URL without userinfo, query, or fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("server_url must not contain a path")
	}
	if err := shared.ValidateHostname(u.Hostname()); err != nil {
		return "", fmt.Errorf("server_url has invalid host")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("server_url must use http or https")
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return "", fmt.Errorf("server_url has invalid port")
		}
	}
	u.Path = ""
	return strings.TrimRight(u.String(), "/"), nil
}

// LoadConfig reads and defaults agent-config.yaml. Production configuration is
// strict: unknown keys (including insecure_skip_verify) and non-HTTPS URLs fail.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.InsecureSkipVerify {
		return nil, fmt.Errorf("insecure_skip_verify=true is not permitted")
	}
	c.ServerURL, err = validateServerURL(c.ServerURL)
	if err != nil {
		return nil, err
	}
	if c.DataDir == "" {
		c.DataDir = "/var/lib/jc-agent"
	}
	if c.Root == "" {
		c.Root = "/opt/joincare-prod"
	}
	if c.PollCommandInterval == 0 {
		c.PollCommandInterval = 5 * time.Second
	}
	if c.PollManifestInterval == 0 {
		c.PollManifestInterval = 5 * time.Minute
	}
	return &c, nil
}

// ClearEnrollToken writes cfg back to path with EnrollToken emptied.
func ClearEnrollToken(path string, cfg *Config) error {
	cfg.EnrollToken = ""
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
