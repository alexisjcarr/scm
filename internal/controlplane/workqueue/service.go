package workqueue

import (
	"context"
	"errors"
	"fmt"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
)

// Store persists work queue claims and status transitions.
type Store interface {
	ClaimNextWork(context.Context, string, time.Duration, time.Time) (*cpdomain.WorkItem, error)
	UpdateWork(context.Context, string, string, string, string, string, []cpdomain.ApplyEvent, time.Time) error
}

// Service owns work claiming and agent-reported state transitions.
type Service struct {
	store         Store
	clock         clock.Clock
	leaseDuration time.Duration
	metrics       *platformmetrics.ControlPlaneMetrics
}

// NewService constructs a work queue service.
func NewService(store Store, clk clock.Clock, leaseDuration time.Duration, metrics *platformmetrics.ControlPlaneMetrics) *Service {
	return &Service{store: store, clock: clk, leaseDuration: leaseDuration, metrics: metrics}
}

// Fetch claims one pending work item for an idle agent.
func (s *Service) Fetch(ctx context.Context, agentID string) (*cpdomain.WorkItem, error) {
	if agentID == "" {
		return nil, errors.New("agent_id is required")
	}
	work, err := s.store.ClaimNextWork(ctx, agentID, s.leaseDuration, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if work != nil && s.metrics != nil {
		s.metrics.WorkClaims.Inc()
	}
	return work, nil
}

// Report applies an agent-authenticated state transition to a work item.
func (s *Service) Report(ctx context.Context, agentID string, workItemID string, leaseToken string, state string, summary string, events []cpdomain.ApplyEvent) error {
	if agentID == "" || workItemID == "" || leaseToken == "" {
		return errors.New("agent_id, work_item_id, and lease_token are required")
	}
	switch state {
	case cpdomain.WorkStateRunning, cpdomain.WorkStateCompleted, cpdomain.WorkStateFailed:
	default:
		return fmt.Errorf("invalid work state %q", state)
	}
	if err := s.store.UpdateWork(ctx, agentID, workItemID, leaseToken, state, summary, events, s.clock.Now()); err != nil {
		return err
	}
	if s.metrics != nil {
		s.metrics.WorkReports.WithLabelValues(state).Inc()
	}
	return nil
}
