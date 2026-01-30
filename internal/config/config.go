// Package config handles loading and validation of veil configuration.
package config

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
