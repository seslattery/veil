// Package config handles loading and validation of veil configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Sandbox SandboxConfig `yaml:"sandbox"`
	Policy  PolicyConfig  `yaml:"policy"`
}

// SandboxConfig defines filesystem isolation settings.
type SandboxConfig struct {
	AllowedWritePaths []string `yaml:"allowed_write_paths"`
}

// PolicyConfig defines network policy settings.
type PolicyConfig struct {
	Allowlist []string `yaml:"allowlist"` // Host glob patterns
}

// DefaultPath returns the default config file path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".veilwarden", "config.yaml"), nil
}

// Load reads and parses the config file.
func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config %s: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if len(c.Policy.Allowlist) == 0 {
		return fmt.Errorf("policy.allowlist cannot be empty")
	}
	return nil
}
