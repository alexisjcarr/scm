package config

import "testing"

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
}

func TestAgentConfigValidate(t *testing.T) {
	t.Parallel()

	cfg := DefaultAgentConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	cfg.PollInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected poll interval validation error")
	}
}
