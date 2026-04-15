package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDecodesYAMLAndRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte("server_address: 127.0.0.1:9000\ntimeout: 30s\n"), 0o644); err != nil {
		t.Fatalf("write valid config: %v", err)
	}

	var cfg CLIConfig
	if err := Load(validPath, &cfg); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ServerAddress != "127.0.0.1:9000" || cfg.Timeout != 30*time.Second {
		t.Fatalf("unexpected decoded config: %+v", cfg)
	}

	invalidPath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte("server_address: ["), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	if err := Load(invalidPath, &cfg); err == nil || !strings.Contains(err.Error(), "decode config") {
		t.Fatalf("expected decode config error, got %v", err)
	}
}

func TestEnsureParentDirCreatesDirectory(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "nested", "state.db")
	if err := EnsureParentDir(target); err != nil {
		t.Fatalf("EnsureParentDir returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(target)); err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}
}

func TestResolveUserPathExpandsHomeRelativePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	resolved, err := ResolveUserPath("~/.config/scm/scmctl.yaml")
	if err != nil {
		t.Fatalf("ResolveUserPath returned error: %v", err)
	}
	if resolved != filepath.Join(home, ".config/scm/scmctl.yaml") {
		t.Fatalf("unexpected resolved path %q", resolved)
	}
}
