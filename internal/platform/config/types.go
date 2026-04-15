package config

import (
	"errors"
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
			return cfg, nil
		}
		return CLIConfig{}, err
	}
	return cfg, nil
}

// ControlPlaneConfig configures scmctld.
type ControlPlaneConfig struct {
	GRPCListenAddress string        `yaml:"grpc_listen_address"`
	HTTPListenAddress string        `yaml:"http_listen_address"`
	DatabasePath      string        `yaml:"database_path"`
	LogLevel          string        `yaml:"log_level"`
	LogJSON           bool          `yaml:"log_json"`
	LeaseDuration     time.Duration `yaml:"lease_duration"`
}

func DefaultControlPlaneConfig() ControlPlaneConfig {
	return ControlPlaneConfig{
		GRPCListenAddress: ":8443",
		HTTPListenAddress: ":8080",
		DatabasePath:      "/var/lib/scm/scmctld.db",
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
			return cfg, nil
		}
		return ControlPlaneConfig{}, err
	}
	return cfg, nil
}

// AgentConfig configures scmctld-agent.
type AgentConfig struct {
	ControlPlaneAddress  string            `yaml:"control_plane_address"`
	StateDatabasePath    string            `yaml:"state_database_path"`
	ManifestCacheDir     string            `yaml:"manifest_cache_dir"`
	MetricsListenAddress string            `yaml:"metrics_listen_address"`
	HostID               string            `yaml:"host_id"`
	AgentID              string            `yaml:"agent_id"`
	Labels               map[string]string `yaml:"labels"`
	LogLevel             string            `yaml:"log_level"`
	LogJSON              bool              `yaml:"log_json"`
	PollInterval         time.Duration     `yaml:"poll_interval"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		ControlPlaneAddress:  "127.0.0.1:8443",
		StateDatabasePath:    "/var/lib/scm/scmctld-agent/state.db",
		ManifestCacheDir:     "/var/lib/scm/scmctld-agent/manifests",
		MetricsListenAddress: ":9108",
		HostID:               "localhost",
		AgentID:              "localhost-agent",
		Labels:               map[string]string{"env": "dev"},
		LogLevel:             "info",
		LogJSON:              false,
		PollInterval:         5 * time.Second,
	}
}

func LoadAgentConfig(path string) (AgentConfig, error) {
	cfg := DefaultAgentConfig()
	if path == "" {
		return cfg, nil
	}
	if err := Load(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return AgentConfig{}, err
	}
	return cfg, nil
}
