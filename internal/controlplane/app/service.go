package app

import (
	"context"
	"time"

	applysvc "github.com/alexisjcarr/scm/internal/controlplane/apply"
	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	"github.com/alexisjcarr/scm/internal/controlplane/inventory"
	"github.com/alexisjcarr/scm/internal/controlplane/workqueue"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
)

// Repository is the combined persistence contract used by the control plane facade.
type Repository interface {
	inventory.Repository
	applysvc.Store
	workqueue.Store
}

// Service coordinates inventory, apply, and work queue behavior behind one facade.
type Service struct {
	inventory *inventory.Service
	applies   *applysvc.Service
	workqueue *workqueue.Service
}

// NewService builds a control-plane service facade from the smaller subdomains.
func NewService(repo Repository, clk clock.Clock, leaseDuration time.Duration, metrics *platformmetrics.ControlPlaneMetrics) *Service {
	return &Service{
		inventory: inventory.NewService(repo, clk, metrics),
		applies:   applysvc.NewService(repo, clk, newID, metrics),
		workqueue: workqueue.NewService(repo, clk, leaseDuration, metrics),
	}
}

func (s *Service) RegisterAgent(ctx context.Context, input inventory.RegisterInput) (cpdomain.Agent, error) {
	return s.inventory.Register(ctx, input)
}

func (s *Service) Heartbeat(ctx context.Context, agentID string, idle bool, workItemID string) error {
	return s.inventory.Heartbeat(ctx, agentID, idle, workItemID)
}

func (s *Service) GetAgents(ctx context.Context) ([]cpdomain.Agent, error) {
	return s.inventory.ListAgents(ctx)
}

func (s *Service) ListApplies(ctx context.Context) ([]cpdomain.Apply, error) {
	if err := s.workqueue.ReconcileStalled(ctx); err != nil {
		return nil, err
	}
	return s.applies.ListApplies(ctx)
}

func (s *Service) SubmitApply(ctx context.Context, compiled manifestdomain.CompiledManifest, rawManifest string, submittedBy string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	agents, err := s.inventory.ListAgents(ctx)
	if err != nil {
		return cpdomain.Apply{}, nil, err
	}
	targetHosts := inventory.ResolveTargetHosts(compiled.Target, agents)
	return s.applies.Submit(ctx, compiled, rawManifest, submittedBy, targetHosts)
}

func (s *Service) GetApply(ctx context.Context, applyID string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	if err := s.workqueue.ReconcileStalled(ctx); err != nil {
		return cpdomain.Apply{}, nil, err
	}
	return s.applies.Get(ctx, applyID)
}

func (s *Service) ListEvents(ctx context.Context, applyID string, fromOffset int64) ([]cpdomain.ApplyEvent, error) {
	return s.applies.ListEvents(ctx, applyID, fromOffset)
}

func (s *Service) FetchWork(ctx context.Context, agentID string) (*cpdomain.WorkItem, error) {
	return s.workqueue.Fetch(ctx, agentID)
}

func (s *Service) ReportWorkStatus(ctx context.Context, agentID string, workItemID string, leaseToken string, state string, summary string, events []cpdomain.ApplyEvent) error {
	return s.workqueue.Report(ctx, agentID, workItemID, leaseToken, state, summary, events)
}
