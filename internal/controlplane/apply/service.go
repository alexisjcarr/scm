package apply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
)

// Store persists apply records and event history.
type Store interface {
	ListApplies(context.Context) ([]cpdomain.Apply, error)
	CreateApply(context.Context, cpdomain.Apply, []cpdomain.WorkItem, []cpdomain.ApplyEvent) error
	GetApply(context.Context, string) (cpdomain.Apply, []cpdomain.WorkItem, error)
	ListEvents(context.Context, string, int64) ([]cpdomain.ApplyEvent, error)
}

// IDGenerator returns stable prefixed identifiers for persisted objects.
type IDGenerator func(prefix string) string

// Service owns apply submission and apply read models.
type Service struct {
	store   Store
	clock   clock.Clock
	newID   IDGenerator
	metrics *platformmetrics.ControlPlaneMetrics
}

// NewService constructs an apply service.
func NewService(store Store, clk clock.Clock, idGen IDGenerator, metrics *platformmetrics.ControlPlaneMetrics) *Service {
	return &Service{store: store, clock: clk, newID: idGen, metrics: metrics}
}

// Submit materializes an apply and one pending work item per target host.
func (s *Service) Submit(ctx context.Context, compiled manifestdomain.CompiledManifest, rawManifest string, submittedBy string, targetHosts []string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	if len(targetHosts) == 0 {
		return cpdomain.Apply{}, nil, errors.New("manifest target resolved to zero registered hosts")
	}

	manifestJSON, err := json.Marshal(compiled.ToAPI())
	if err != nil {
		return cpdomain.Apply{}, nil, fmt.Errorf("marshal compiled manifest: %w", err)
	}

	now := s.clock.Now()
	apply := cpdomain.Apply{
		ApplyID:      s.newID("apply"),
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
		workItemID := s.newID("work")
		workItems = append(workItems, cpdomain.WorkItem{
			WorkItemID:   workItemID,
			ApplyID:      apply.ApplyID,
			HostID:       hostID,
			State:        cpdomain.WorkStatePending,
			ManifestJSON: string(manifestJSON),
			UpdatedAt:    now,
		})
		events = append(events, cpdomain.ApplyEvent{
			ID:         s.newID("evt"),
			ApplyID:    apply.ApplyID,
			HostID:     hostID,
			WorkItemID: workItemID,
			Level:      "info",
			Phase:      "submitted",
			Message:    "pending work item created",
			CreatedAt:  now,
		})
	}

	if err := s.store.CreateApply(ctx, apply, workItems, events); err != nil {
		return cpdomain.Apply{}, nil, err
	}
	if s.metrics != nil {
		s.metrics.ApplySubmissions.Inc()
	}
	return apply, workItems, nil
}

// ListApplies returns recent applies.
func (s *Service) ListApplies(ctx context.Context) ([]cpdomain.Apply, error) {
	return s.store.ListApplies(ctx)
}

// Get returns an apply detail view.
func (s *Service) Get(ctx context.Context, applyID string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	return s.store.GetApply(ctx, applyID)
}

// ListEvents returns apply events from a row offset.
func (s *Service) ListEvents(ctx context.Context, applyID string, fromOffset int64) ([]cpdomain.ApplyEvent, error) {
	return s.store.ListEvents(ctx, applyID, fromOffset)
}

// AggregateStatus derives the apply state from per-host work item states.
func AggregateStatus(states []string) string {
	allCompleted := true
	anyRunning := false
	anyFailed := false
	for _, state := range states {
		switch state {
		case cpdomain.WorkStateFailed:
			anyFailed = true
			allCompleted = false
		case cpdomain.WorkStateCompleted:
		case cpdomain.WorkStateRunning, cpdomain.WorkStateAssigned:
			anyRunning = true
			allCompleted = false
		default:
			allCompleted = false
		}
	}
	if anyFailed {
		return cpdomain.ApplyStatusFailed
	}
	if allCompleted && len(states) > 0 {
		return cpdomain.ApplyStatusCompleted
	}
	if anyRunning {
		return cpdomain.ApplyStatusRunning
	}
	return cpdomain.ApplyStatusPending
}
