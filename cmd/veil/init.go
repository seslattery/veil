package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file",
	Long: `Creates a default configuration file at ~/.veilwarden/config.yaml.

The default config uses a strict read-deny-by-default sandbox profile.
Reads are allowed for system paths and configured allowed_read_paths.
Writes are allowed for configured allowed_write_paths.
TMPDIR, XDG_CACHE_HOME, and XDG_CONFIG_HOME are automatically allowed
when explicitly set in the environment.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const defaultConfig = `# Veil configuration
# See: https://github.com/seslattery/veil

sandbox:
  # Read-only path exceptions (strict read-deny-by-default)
  # allowed_read_paths:
  #   - ~/.claude

  # Read+write path exceptions
  allowed_write_paths:
    - ./            # Current directory
    - /tmp          # Temporary files
    - ~/.claude     # Claude Code config directory
    - ~/.claude.json  # Claude Code config file

  # TMPDIR, XDG_CACHE_HOME, and XDG_CONFIG_HOME are automatically
  # allowed (read+write) when explicitly set in the environment.

policy:
  allowlist:
    # Package registries
    - "*.npmjs.org"
    - "registry.npmjs.org"
    - "*.yarnpkg.com"
    - "pypi.org"
    - "*.pypi.org"
    - "files.pythonhosted.org"

    # Version control
    - "github.com"
    - "*.github.com"
    - "gitlab.com"
    - "*.gitlab.com"

    # Google Cloud Storage
    - "storage.googleapis.com"

    # AI services
    - "api.anthropic.com"
    - "*.anthropic.com"
    - "*.claude.ai"
    - "*.claude.com"
    - "api.openai.com"
`

func runInit(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(home, ".veilwarden")
	configPath := filepath.Join(configDir, "config.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config already exists at %s", configPath)
	}

	// Create directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created config at %s\n", configPath)
	return nil
}
