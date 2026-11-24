package server

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TLSConfig configures optional server-side TLS termination.
type TLSConfig struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

// Config is the yaml-driven server configuration.
type Config struct {
	ListenAddr          string     `yaml:"listen_addr"`
	DataDir             string     `yaml:"data_dir"`
	AdminUsername       string     `yaml:"admin_username"`
	AdminPasswordBcrypt string     `yaml:"admin_password_bcrypt"`
	SessionSecret       string     `yaml:"session_secret"`
	ReadAPIToken        string     `yaml:"read_api_token,omitempty"`
	TLS                 *TLSConfig `yaml:"tls,omitempty"`
}

// LoadConfig reads and validates a server-config.yaml file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:8090"
	}
	if c.DataDir == "" {
		return nil, fmt.Errorf("data_dir is required")
	}
	if c.AdminUsername == "" || c.AdminPasswordBcrypt == "" {
		return nil, fmt.Errorf("admin_username and admin_password_bcrypt are required")
	}
	if len(c.SessionSecret) < 16 {
		return nil, fmt.Errorf("session_secret must be at least 16 chars")
	}
	if c.ReadAPIToken != "" && len(c.ReadAPIToken) < 32 {
		return nil, fmt.Errorf("read_api_token must be at least 32 chars when configured")
	}
	return &c, nil
}
