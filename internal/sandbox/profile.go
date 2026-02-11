// Package sandbox provides macOS seatbelt sandbox functionality.
package sandbox

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

//go:embed profile.sbpl.tmpl
var profileTemplate string

type profileData struct {
	HomeDir           string
	ProxyPort         int
	AllowedReadDirs   []string
	AllowedReadFiles  []string
	AllowedWriteDirs  []string
	AllowedWriteFiles []string
	EnablePTY         bool
}

func collectPATHDirs() []string {
	pathVal := os.Getenv("PATH")
	if pathVal == "" {
		return nil
	}
	seen := make(map[string]bool)
	var dirs []string
	for _, dir := range strings.Split(pathVal, ":") {
		if dir == "" || !filepath.IsAbs(dir) || containsControlChars(dir) {
			continue
		}
		cleaned := filepath.Clean(dir)
		resolved, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			resolved = cleaned
		}
		if !seen[resolved] {
			seen[resolved] = true
			dirs = append(dirs, resolved)
		}
	}
	return dirs
}

func collectEnvPaths() []string {
	seen := make(map[string]bool)
	var paths []string

	for _, key := range []string{"TMPDIR", "XDG_CACHE_HOME", "XDG_CONFIG_HOME"} {
		val := os.Getenv(key)
		if val == "" {
			continue
		}
		if !filepath.IsAbs(val) {
			continue
		}
		if containsControlChars(val) {
			continue
		}
		cleaned := filepath.Clean(val)
		resolved, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			resolved = cleaned
		}
		if !seen[resolved] {
			seen[resolved] = true
			paths = append(paths, resolved)
		}
	}

	return paths
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != 0x09 {
			return true
		}
	}
	return false
}

func processPaths(paths []string, homeDir string) (dirs, files []string) {
	for _, p := range paths {
		expanded := expandPath(p, homeDir)
		abs, err := filepath.Abs(expanded)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			resolved = abs
		}

		info, err := os.Stat(resolved)
		isFile := err == nil && !info.IsDir()

		if isFile {
			files = append(files, resolved)
		} else {
			dirs = append(dirs, resolved)
		}
	}
	return
}

func dedup(s []string) []string {
	if len(s) == 0 {
		return s
	}
	slices.Sort(s)
	return slices.Compact(s)
}

func GenerateProfile(proxyPort int, allowedReadPaths, allowedWritePaths []string, enablePTY bool) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	canonicalCWD, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		canonicalCWD = cwd
	}

	envPaths := collectEnvPaths()
	pathDirs := collectPATHDirs()

	readDirs, readFiles := processPaths(allowedReadPaths, homeDir)
	writeDirs, writeFiles := processPaths(allowedWritePaths, homeDir)

	readDirs = append(readDirs, pathDirs...)
	writeDirs = append(writeDirs, canonicalCWD)
	writeDirs = append(writeDirs, envPaths...)

	data := profileData{
		HomeDir:           homeDir,
		ProxyPort:         proxyPort,
		AllowedReadDirs:   dedup(readDirs),
		AllowedReadFiles:  dedup(readFiles),
		AllowedWriteDirs:  dedup(writeDirs),
		AllowedWriteFiles: dedup(writeFiles),
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

func expandPath(path, homeDir string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(homeDir, path[2:])
	}
	if path == "~" {
		return homeDir
	}
	return path
}
