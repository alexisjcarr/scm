package inventory

import (
	"context"
	"errors"
	"sort"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
)

// Repository stores registered agent inventory data.
type Repository interface {
	UpsertAgent(context.Context, cpdomain.Agent) error
	UpdateHeartbeat(context.Context, string, bool, string, time.Time) error
	ListAgents(context.Context) ([]cpdomain.Agent, error)
}

// RegisterInput contains the agent attributes accepted during registration.
type RegisterInput struct {
	AgentID      string
	HostID       string
	Version      string
	Labels       map[string]string
	Capabilities []string
}

// Service owns host registration, heartbeat tracking, and target resolution.
type Service struct {
	repo    Repository
	clock   clock.Clock
	metrics *platformmetrics.ControlPlaneMetrics
}

// NewService constructs an inventory service.
func NewService(repo Repository, clk clock.Clock, metrics *platformmetrics.ControlPlaneMetrics) *Service {
	return &Service{repo: repo, clock: clk, metrics: metrics}
}

// Register creates or refreshes an agent record.
func (s *Service) Register(ctx context.Context, input RegisterInput) (cpdomain.Agent, error) {
	if input.AgentID == "" || input.HostID == "" {
		return cpdomain.Agent{}, errors.New("agent_id and host_id are required")
	}

	agent := cpdomain.Agent{
		AgentID:      input.AgentID,
		HostID:       input.HostID,
		Version:      input.Version,
		Labels:       cloneMap(input.Labels),
		Capabilities: append([]string(nil), input.Capabilities...),
		Idle:         true,
		LastSeenAt:   s.clock.Now(),
	}
	if err := s.repo.UpsertAgent(ctx, agent); err != nil {
		return cpdomain.Agent{}, err
	}
	if s.metrics != nil {
		s.metrics.InventoryRegistrations.Inc()
	}
	return agent, nil
}

// Heartbeat updates an agent liveness record.
func (s *Service) Heartbeat(ctx context.Context, agentID string, idle bool, workItemID string) error {
	if agentID == "" {
		return errors.New("agent_id is required")
	}
	if err := s.repo.UpdateHeartbeat(ctx, agentID, idle, workItemID, s.clock.Now()); err != nil {
		return err
	}
	if s.metrics != nil {
		s.metrics.InventoryHeartbeats.Inc()
	}
	return nil
}

// ListAgents returns the current registered agent inventory.
func (s *Service) ListAgents(ctx context.Context) ([]cpdomain.Agent, error) {
	return s.repo.ListAgents(ctx)
}

// ResolveTargetHosts resolves explicit hosts and selector matches into one set.
func ResolveTargetHosts(target manifestdomain.TargetSpec, agents []cpdomain.Agent) []string {
	seen := make(map[string]struct{})
	var hosts []string

	for _, host := range target.Hosts {
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}

	if len(target.SelectorLabels) > 0 {
		for _, agent := range agents {
			if matchesSelector(agent.Labels, target.SelectorLabels) {
				if _, ok := seen[agent.HostID]; ok {
					continue
				}
				seen[agent.HostID] = struct{}{}
				hosts = append(hosts, agent.HostID)
			}
		}
	}

	sort.Strings(hosts)
	return hosts
}

func matchesSelector(agentLabels map[string]string, selector map[string]string) bool {
	for key, value := range selector {
		if agentLabels[key] != value {
			return false
		}
	}
	return true
}

func cloneMap[K comparable, V any](input map[K]V) map[K]V {
	if len(input) == 0 {
		return nil
	}
	out := make(map[K]V, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
