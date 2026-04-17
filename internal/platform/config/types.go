package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// CLIConfig configures scmctl.
type CLIConfig struct {
	ServerAddress string        `yaml:"server_address"`
	Timeout       time.Duration `yaml:"timeout"`
}

func DefaultCLIConfig() CLIConfig {
	return CLIConfig{
		ServerAddress: "127.0.0.1:8443",
		Timeout:       15 * time.Second,
	}
}

func LoadCLIConfig(path string) (CLIConfig, error) {
	cfg := DefaultCLIConfig()
	if path == "" {
		return cfg, nil
	}
	resolved, err := ResolveUserPath(path)
	if err != nil {
		return CLIConfig{}, err
	}
	if err := Load(resolved, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, cfg.Validate()
		}
		return CLIConfig{}, err
	}
	return cfg, cfg.Validate()
}

// Validate checks the CLI config for required fields.
func (c CLIConfig) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("server_address is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	return nil
}

// ControlPlaneConfig configures scmctld.
type ControlPlaneConfig struct {
	GRPCListenAddress string            `yaml:"grpc_listen_address"`
	HTTPListenAddress string            `yaml:"http_listen_address"`
	DatabasePath      string            `yaml:"database_path"`
	AgentAuthTokens   map[string]string `yaml:"agent_auth_tokens"`
	LogLevel          string            `yaml:"log_level"`
	LogJSON           bool              `yaml:"log_json"`
	LeaseDuration     time.Duration     `yaml:"lease_duration"`
}

func DefaultControlPlaneConfig() ControlPlaneConfig {
	return ControlPlaneConfig{
		GRPCListenAddress: ":8443",
		HTTPListenAddress: ":8080",
		DatabasePath:      "/var/lib/scm/scmctld.db",
		AgentAuthTokens:   map[string]string{"localhost-agent": "dev-agent-token"},
		LogLevel:          "info",
		LogJSON:           false,
		LeaseDuration:     2 * time.Minute,
	}
}

func LoadControlPlaneConfig(path string) (ControlPlaneConfig, error) {
	cfg := DefaultControlPlaneConfig()
	if path == "" {
		return cfg, nil
	}
	if err := Load(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, cfg.Validate()
		}
		return ControlPlaneConfig{}, err
	}
	return cfg, cfg.Validate()
}

// Validate checks the control plane config for required fields.
func (c ControlPlaneConfig) Validate() error {
	if c.GRPCListenAddress == "" {
		return fmt.Errorf("grpc_listen_address is required")
	}
	if c.HTTPListenAddress == "" {
		return fmt.Errorf("http_listen_address is required")
	}
	if c.DatabasePath == "" {
		return fmt.Errorf("database_path is required")
	}
	if len(c.AgentAuthTokens) == 0 {
		return fmt.Errorf("agent_auth_tokens must contain at least one agent token")
	}
	for agentID, token := range c.AgentAuthTokens {
		if agentID == "" || token == "" {
			return fmt.Errorf("agent_auth_tokens must not contain empty agent ids or tokens")
		}
	}
	if c.LeaseDuration <= 0 {
		return fmt.Errorf("lease_duration must be greater than zero")
	}
	return nil
}

// AgentConfig configures scmctld-agent.
type AgentConfig struct {
	ControlPlaneAddress  string            `yaml:"control_plane_address"`
	StateDir             string            `yaml:"state_dir"`
	ManifestCacheDir     string            `yaml:"manifest_cache_dir"`
	MetricsListenAddress string            `yaml:"metrics_listen_address"`
	HostID               string            `yaml:"host_id"`
	AgentID              string            `yaml:"agent_id"`
	AuthToken            string            `yaml:"auth_token"`
	Labels               map[string]string `yaml:"labels"`
	LogLevel             string            `yaml:"log_level"`
	LogJSON              bool              `yaml:"log_json"`
	PollInterval         time.Duration     `yaml:"poll_interval"`
	RunTimeout           time.Duration     `yaml:"run_timeout"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		ControlPlaneAddress:  "127.0.0.1:8443",
		StateDir:             "/var/lib/scm/scmctld-agent/state",
		ManifestCacheDir:     "/var/lib/scm/scmctld-agent/manifests",
		MetricsListenAddress: ":9108",
		HostID:               "localhost",
		AgentID:              "localhost-agent",
		AuthToken:            "dev-agent-token",
		Labels:               map[string]string{"env": "dev"},
		LogLevel:             "info",
		LogJSON:              false,
		PollInterval:         5 * time.Second,
		RunTimeout:           5 * time.Minute,
	}
}

func LoadAgentConfig(path string) (AgentConfig, error) {
	cfg := DefaultAgentConfig()
	if path == "" {
		return cfg, nil
	}
	if err := Load(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, cfg.Validate()
		}
		return AgentConfig{}, err
	}
	return cfg, cfg.Validate()
}

// Validate checks the agent config for required fields.
func (c AgentConfig) Validate() error {
	if c.ControlPlaneAddress == "" {
		return fmt.Errorf("control_plane_address is required")
	}
	if c.StateDir == "" {
		return fmt.Errorf("state_dir is required")
	}
	if c.ManifestCacheDir == "" {
		return fmt.Errorf("manifest_cache_dir is required")
	}
	if c.MetricsListenAddress == "" {
		return fmt.Errorf("metrics_listen_address is required")
	}
	if c.HostID == "" {
		return fmt.Errorf("host_id is required")
	}
	if c.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if c.AuthToken == "" {
		return fmt.Errorf("auth_token is required")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("poll_interval must be greater than zero")
	}
	if c.RunTimeout <= 0 {
		return fmt.Errorf("run_timeout must be greater than zero")
	}
	return nil
}
