package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads YAML config into out. Missing files are returned as os.ErrNotExist
// so callers can decide whether defaults should apply.
func Load(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode config %s: %w", path, err)
	}

	return nil
}

// EnsureParentDir creates the parent directory for file-backed state paths.
func EnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// EnsureDir creates a directory-backed state path when needed.
func EnsureDir(path string) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

// ResolveUserPath expands a leading tilde for user config locations.
func ResolveUserPath(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("resolve home directory")
	}

	if path == "~" {
		return home, nil
	}

	if len(path) > 1 && path[1] == '/' {
		return filepath.Join(home, path[2:]), nil
	}

	return "", fmt.Errorf("unsupported home-relative path %q", path)
}
