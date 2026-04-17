package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCLIConfigValidate(t *testing.T) {
	t.Parallel()

	cfg := DefaultCLIConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	cfg.Timeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected timeout validation error")
	}
}

func TestControlPlaneConfigValidate(t *testing.T) {
	t.Parallel()

	cfg := DefaultControlPlaneConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	cfg.DatabasePath = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected database path validation error")
	}
	cfg = DefaultControlPlaneConfig()
	cfg.AgentAuthTokens = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected agent auth tokens validation error")
	}
}

func TestAgentConfigValidate(t *testing.T) {
	t.Parallel()

	cfg := DefaultAgentConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	cfg.StateDir = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected state dir validation error")
	}
	cfg = DefaultAgentConfig()
	cfg.PollInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected poll interval validation error")
	}
	cfg = DefaultAgentConfig()
	cfg.RunTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected run timeout validation error")
	}
	cfg = DefaultAgentConfig()
	cfg.AuthToken = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected auth token validation error")
	}
}

func TestLoadCLIConfigResolvesTildeAndMergesValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "scm", "scmctl.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create cli config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("server_address: 10.0.0.5:8443\ntimeout: 42s\n"), 0o644); err != nil {
		t.Fatalf("write cli config: %v", err)
	}

	cfg, err := LoadCLIConfig("~/.config/scm/scmctl.yaml")
	if err != nil {
		t.Fatalf("LoadCLIConfig returned error: %v", err)
	}
	if cfg.ServerAddress != "10.0.0.5:8443" || cfg.Timeout != 42*time.Second {
		t.Fatalf("unexpected cli config: %+v", cfg)
	}
}

func TestLoadControlPlaneConfigMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadControlPlaneConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("LoadControlPlaneConfig returned error: %v", err)
	}
	if cfg.GRPCListenAddress != ":8443" || cfg.HTTPListenAddress != ":8080" {
		t.Fatalf("unexpected default control-plane config: %+v", cfg)
	}
}

func TestLoadAgentConfigReadsFileAndValidates(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "agent.yaml")
	data := []byte("" +
		"control_plane_address: 10.0.0.5:8443\n" +
		"state_dir: /tmp/state\n" +
		"manifest_cache_dir: /tmp/manifests\n" +
		"metrics_listen_address: :9108\n" +
		"host_id: web-1\n" +
		"agent_id: web-1-agent\n" +
		"auth_token: secret-token\n" +
		"poll_interval: 10s\n" +
		"run_timeout: 2m\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write agent config: %v", err)
	}

	cfg, err := LoadAgentConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig returned error: %v", err)
	}
	if cfg.ControlPlaneAddress != "10.0.0.5:8443" || cfg.PollInterval != 10*time.Second || cfg.RunTimeout != 2*time.Minute {
		t.Fatalf("unexpected agent config: %+v", cfg)
	}
}
