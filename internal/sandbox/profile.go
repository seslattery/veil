// Package sandbox provides macOS seatbelt sandbox functionality.
package sandbox

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed profile.sbpl.tmpl
var profileTemplate string

// profileData holds the data for rendering the seatbelt profile.
type profileData struct {
	HomeDir           string
	ProxyPort         int
	AllowedWritePaths []string
	DangerousPatterns []string
	EnablePTY         bool
}

// dangerousPatterns returns seatbelt regex patterns for files that should
// never be writable, even in allowed paths.
func dangerousPatterns() []string {
	return []string{
		`.*/\.env$`,
		`.*/\.env\..*`,
		`^\.env$`,
		`.*/\.npmrc$`,
		`^\.npmrc$`,
		`.*/\.pypirc$`,
		`^\.pypirc$`,
		`.*/\.gem/credentials$`,
		`.*/\.git/hooks$`,
		`.*/\.git/hooks/.*`,
		`.*/\.git/config$`,
		`.*/\.docker/config\.json$`,
		`.*/\.aws/credentials$`,
		`.*/\.azure/credentials$`,
		`.*/\.config/gcloud/credentials\.db$`,
	}
}

// GenerateProfile renders the seatbelt profile with the given configuration.
func GenerateProfile(proxyPort int, allowedWritePaths []string, enablePTY bool) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Expand paths, make absolute, and resolve symlinks
	expandedPaths := make([]string, 0, len(allowedWritePaths))
	for _, p := range allowedWritePaths {
		expanded := expandPath(p, homeDir)
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return "", err
		}
		// Resolve symlinks (e.g., /tmp -> /private/tmp on macOS)
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			// Path may not exist yet; use the absolute path as-is
			resolved = abs
		}
		expandedPaths = append(expandedPaths, resolved)
	}

	data := profileData{
		HomeDir:           homeDir,
		ProxyPort:         proxyPort,
		AllowedWritePaths: expandedPaths,
		DangerousPatterns: dangerousPatterns(),
		EnablePTY:         enablePTY,
	}

	tmpl, err := template.New("profile").Parse(profileTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// expandPath expands ~ to home directory.
func expandPath(path, homeDir string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(homeDir, path[2:])
	}
	if path == "~" {
		return homeDir
	}
	return path
}
