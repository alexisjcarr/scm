package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

// Repository defines the control-plane persistence contract.
type Repository interface {
	UpsertAgent(context.Context, cpdomain.Agent) error
	UpdateHeartbeat(context.Context, string, bool, string, time.Time) error
	ListAgents(context.Context) ([]cpdomain.Agent, error)
	ListApplies(context.Context) ([]cpdomain.Apply, error)
	CreateApply(context.Context, cpdomain.Apply, []cpdomain.WorkItem, []cpdomain.ApplyEvent) error
	GetApply(context.Context, string) (cpdomain.Apply, []cpdomain.WorkItem, error)
	ListEvents(context.Context, string, int64) ([]cpdomain.ApplyEvent, error)
	ClaimNextWork(context.Context, string, time.Duration, time.Time) (*cpdomain.WorkItem, error)
	UpdateWork(context.Context, string, string, string, string, string, []cpdomain.ApplyEvent, time.Time) error
}

// Service contains the core control-plane use cases.
type Service struct {
	repo          Repository
	clock         clock.Clock
	leaseDuration time.Duration
}

// NewService builds a control-plane service.
func NewService(repo Repository, clk clock.Clock, leaseDuration time.Duration) *Service {
	return &Service{repo: repo, clock: clk, leaseDuration: leaseDuration}
}

func (s *Service) RegisterAgent(ctx context.Context, req scmv1.RegisterAgentRequest) (cpdomain.Agent, error) {
	if req.AgentID == "" || req.HostID == "" {
		return cpdomain.Agent{}, errors.New("agent_id and host_id are required")
	}

	agent := cpdomain.Agent{
		AgentID:      req.AgentID,
		HostID:       req.HostID,
		Version:      req.Version,
		Labels:       cloneMap(req.Labels),
		Capabilities: append([]string(nil), req.Capabilities...),
		Idle:         true,
		LastSeenAt:   s.clock.Now(),
	}

	if err := s.repo.UpsertAgent(ctx, agent); err != nil {
		return cpdomain.Agent{}, err
	}
	return agent, nil
}

func (s *Service) Heartbeat(ctx context.Context, agentID string, idle bool, workItemID string) error {
	if agentID == "" {
		return errors.New("agent_id is required")
	}
	return s.repo.UpdateHeartbeat(ctx, agentID, idle, workItemID, s.clock.Now())
}

func (s *Service) GetAgents(ctx context.Context) ([]cpdomain.Agent, error) {
	return s.repo.ListAgents(ctx)
}

func (s *Service) ListApplies(ctx context.Context) ([]cpdomain.Apply, error) {
	return s.repo.ListApplies(ctx)
}

func (s *Service) SubmitApply(ctx context.Context, compiled manifestdomain.CompiledManifest, rawManifest string, submittedBy string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	agents, err := s.repo.ListAgents(ctx)
	if err != nil {
		return cpdomain.Apply{}, nil, err
	}

	targetHosts := resolveTargetHosts(compiled.Target, agents)
	if len(targetHosts) == 0 {
		return cpdomain.Apply{}, nil, errors.New("manifest target resolved to zero registered hosts")
	}

	manifestJSON, err := json.Marshal(compiled.ToAPI())
	if err != nil {
		return cpdomain.Apply{}, nil, fmt.Errorf("marshal compiled manifest: %w", err)
	}

	now := s.clock.Now()
	apply := cpdomain.Apply{
		ApplyID:      newID("apply"),
		Name:         compiled.Name,
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  submittedBy,
		RawManifest:  rawManifest,
		ManifestJSON: string(manifestJSON),
		CreatedAt:    now,
	}

	workItems := make([]cpdomain.WorkItem, 0, len(targetHosts))
	events := make([]cpdomain.ApplyEvent, 0, len(targetHosts))
	for _, hostID := range targetHosts {
		workItemID := newID("work")
		workItems = append(workItems, cpdomain.WorkItem{
			WorkItemID:   workItemID,
			ApplyID:      apply.ApplyID,
			HostID:       hostID,
			State:        cpdomain.WorkStatePending,
			ManifestJSON: string(manifestJSON),
			UpdatedAt:    now,
		})
		events = append(events, cpdomain.ApplyEvent{
			ID:         newID("evt"),
			ApplyID:    apply.ApplyID,
			HostID:     hostID,
			WorkItemID: workItemID,
			Level:      "info",
			Phase:      "submitted",
			Message:    "pending work item created",
			CreatedAt:  now,
		})
	}

	if err := s.repo.CreateApply(ctx, apply, workItems, events); err != nil {
		return cpdomain.Apply{}, nil, err
	}
	return apply, workItems, nil
}

func (s *Service) GetApply(ctx context.Context, applyID string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	return s.repo.GetApply(ctx, applyID)
}

func (s *Service) ListEvents(ctx context.Context, applyID string, fromOffset int64) ([]cpdomain.ApplyEvent, error) {
	return s.repo.ListEvents(ctx, applyID, fromOffset)
}

func (s *Service) FetchWork(ctx context.Context, agentID string) (*cpdomain.WorkItem, error) {
	if agentID == "" {
		return nil, errors.New("agent_id is required")
	}
	return s.repo.ClaimNextWork(ctx, agentID, s.leaseDuration, s.clock.Now())
}

func (s *Service) ReportWorkStatus(ctx context.Context, agentID string, workItemID string, leaseToken string, state string, summary string, events []cpdomain.ApplyEvent) error {
	if agentID == "" || workItemID == "" || leaseToken == "" {
		return errors.New("agent_id, work_item_id, and lease_token are required")
	}
	switch state {
	case cpdomain.WorkStateRunning, cpdomain.WorkStateCompleted, cpdomain.WorkStateFailed:
	default:
		return fmt.Errorf("invalid work state %q", state)
	}
	return s.repo.UpdateWork(ctx, agentID, workItemID, leaseToken, state, summary, events, s.clock.Now())
}

func resolveTargetHosts(target manifestdomain.TargetSpec, agents []cpdomain.Agent) []string {
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
