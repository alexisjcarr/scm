package configs_test

import (
	"testing"

	platformconfig "github.com/alexisjcarr/scm/internal/platform/config"
)

func TestComposeControlPlaneConfigIsValid(t *testing.T) {
	t.Parallel()

	cfg, err := platformconfig.LoadControlPlaneConfig("dev/scmctld-compose.yaml")
	if err != nil {
		t.Fatalf("LoadControlPlaneConfig returned error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("compose control-plane config should validate: %v", err)
	}
}

func TestRemoteAgentConfigIsValid(t *testing.T) {
	t.Parallel()

	cfg, err := platformconfig.LoadAgentConfig("examples/scmctld-agent-remote.yaml")
	if err != nil {
		t.Fatalf("LoadAgentConfig returned error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("remote agent config should validate: %v", err)
	}
}
