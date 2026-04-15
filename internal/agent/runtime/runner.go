package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

// Repository stores local work state persisted on the host.
type Repository interface {
	SaveWork(context.Context, agentdomain.LocalApply) error
	UpdateWork(context.Context, string, string, string, time.Time) error
}

// Backend reconciles host resources in an idempotent way.
type Backend interface {
	EnsurePackage(context.Context, manifestdomain.PackageResource) (bool, string, error)
	EnsureFile(context.Context, manifestdomain.FileResource) (bool, string, error)
	EnsureService(context.Context, manifestdomain.ServiceResource, bool) (bool, string, error)
}

// Runner owns local manifest persistence and resource reconciliation.
type Runner struct {
	repo    Repository
	backend Backend
	metrics *platformmetrics.AgentMetrics
}

// NewRunner constructs an agent runtime runner.
func NewRunner(repo Repository, backend Backend, metrics *platformmetrics.AgentMetrics) *Runner {
	return &Runner{repo: repo, backend: backend, metrics: metrics}
}

// Prepare persists work locally before execution begins.
func (r *Runner) Prepare(ctx context.Context, work *scmv1.WorkItem, manifestCacheDir string) ([]*scmv1.ApplyEvent, error) {
	if err := os.MkdirAll(manifestCacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create manifest cache dir: %w", err)
	}
	cachePath := filepath.Join(manifestCacheDir, work.WorkItemID+".json")
	if err := os.WriteFile(cachePath, []byte(work.ManifestJSON), 0o644); err != nil {
		return nil, fmt.Errorf("write manifest cache: %w", err)
	}
	if err := r.repo.SaveWork(ctx, agentdomain.LocalApply{
		WorkItemID:    work.WorkItemID,
		ApplyID:       work.ApplyID,
		ManifestJSON:  work.ManifestJSON,
		State:         agentdomain.PhasePersisted,
		Summary:       "manifest cached locally",
		LeaseToken:    work.LeaseToken,
		LastUpdatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	return []*scmv1.ApplyEvent{{
		ID:         newEventID(work.WorkItemID, agentdomain.PhasePersisted),
		ApplyID:    work.ApplyID,
		HostID:     work.HostID,
		WorkItemID: work.WorkItemID,
		Level:      "info",
		Phase:      agentdomain.PhasePersisted,
		Message:    "manifest persisted locally",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}}, nil
}

// Execute runs the manifest reconciliation flow for a claimed work item.
func (r *Runner) Execute(ctx context.Context, work *scmv1.WorkItem) (string, []*scmv1.ApplyEvent, string, error) {
	var apiManifest scmv1.Manifest
	if err := json.Unmarshal([]byte(work.ManifestJSON), &apiManifest); err != nil {
		return "", nil, "", fmt.Errorf("decode manifest json: %w", err)
	}
	compiled, err := manifestFromAPI(&apiManifest)
	if err != nil {
		return "", nil, "", err
	}

	events := []*scmv1.ApplyEvent{{
		ID:         newEventID(work.WorkItemID, agentdomain.PhasePlanning),
		ApplyID:    work.ApplyID,
		HostID:     work.HostID,
		WorkItemID: work.WorkItemID,
		Level:      "info",
		Phase:      agentdomain.PhasePlanning,
		Message:    fmt.Sprintf("planning %d resources", len(compiled.OrderedResources)),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}}

	notifySet := make(map[string]manifestdomain.ServiceResource)
	for _, resource := range compiled.OrderedResources {
		result, err := r.applyResource(ctx, resource, false)
		events = append(events, resultEvent(work, result))
		if err != nil {
			if r.metrics != nil {
				r.metrics.WorkFinished.WithLabelValues("failed").Inc()
			}
			return "", events, "failed", err
		}
		if result.Changed {
			for _, notifyID := range resource.GetNotifies() {
				notifyResource, ok := findServiceByID(compiled.Resources, notifyID)
				if ok {
					notifySet[notifyID] = notifyResource
				}
			}
		}
	}

	if len(notifySet) > 0 {
		events = append(events, &scmv1.ApplyEvent{
			ID:         newEventID(work.WorkItemID, agentdomain.PhaseNotifying),
			ApplyID:    work.ApplyID,
			HostID:     work.HostID,
			WorkItemID: work.WorkItemID,
			Level:      "info",
			Phase:      agentdomain.PhaseNotifying,
			Message:    fmt.Sprintf("running %d notify follow-ups", len(notifySet)),
			CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		})
		for _, resource := range notifySet {
			result, err := r.applyResource(ctx, resource, true)
			events = append(events, resultEvent(work, result))
			if err != nil {
				if r.metrics != nil {
					r.metrics.WorkFinished.WithLabelValues("failed").Inc()
				}
				return "", events, "failed", err
			}
		}
	}

	if r.metrics != nil {
		r.metrics.WorkFinished.WithLabelValues("completed").Inc()
	}
	return fmt.Sprintf("applied %d resources", len(compiled.OrderedResources)), events, "completed", nil
}

// Complete persists the terminal local work state.
func (r *Runner) Complete(ctx context.Context, workItemID string, state string, summary string) error {
	return r.repo.UpdateWork(ctx, workItemID, state, summary, time.Now().UTC())
}

func (r *Runner) applyResource(ctx context.Context, resource manifestdomain.Resource, notifyOnly bool) (agentdomain.ResourceResult, error) {
	switch typed := resource.(type) {
	case manifestdomain.PackageResource:
		changed, message, err := r.backend.EnsurePackage(ctx, typed)
		return agentdomain.ResourceResult{ResourceID: typed.ID, ResourceType: typed.GetType(), Changed: changed, Message: message}, err
	case manifestdomain.FileResource:
		changed, message, err := r.backend.EnsureFile(ctx, typed)
		return agentdomain.ResourceResult{ResourceID: typed.ID, ResourceType: typed.GetType(), Changed: changed, Message: message}, err
	case manifestdomain.ServiceResource:
		changed, message, err := r.backend.EnsureService(ctx, typed, notifyOnly)
		return agentdomain.ResourceResult{ResourceID: typed.ID, ResourceType: typed.GetType(), Changed: changed, Message: message}, err
	default:
		return agentdomain.ResourceResult{}, errors.New("unsupported resource type")
	}
}

func findServiceByID(resources []manifestdomain.Resource, id string) (manifestdomain.ServiceResource, bool) {
	for _, resource := range resources {
		if resource.GetID() != id {
			continue
		}
		service, ok := resource.(manifestdomain.ServiceResource)
		return service, ok
	}
	return manifestdomain.ServiceResource{}, false
}

func manifestFromAPI(manifest *scmv1.Manifest) (manifestdomain.CompiledManifest, error) {
	if manifest == nil {
		return manifestdomain.CompiledManifest{}, errors.New("manifest is required")
	}
	target := &scmv1.TargetSpec{}
	if manifest.Target != nil {
		target = manifest.Target
	}
	selector := &scmv1.HostSelector{}
	if target.Selector != nil {
		selector = target.Selector
	}
	resources := make([]manifestdomain.Resource, 0, len(manifest.Resources))
	for _, resource := range manifest.Resources {
		switch resource.Type {
		case manifestdomain.ResourceTypePackage:
			resources = append(resources, manifestdomain.PackageResource{
				ID:       resource.ID,
				Name:     resource.Name,
				State:    resource.State,
				Requires: append([]string(nil), resource.Requires...),
				Notifies: append([]string(nil), resource.Notifies...),
			})
		case manifestdomain.ResourceTypeFile:
			resources = append(resources, manifestdomain.FileResource{
				ID:       resource.ID,
				Path:     resource.Path,
				Content:  resource.Content,
				Mode:     resource.Mode,
				Owner:    resource.Owner,
				Group:    resource.Group,
				State:    resource.State,
				Requires: append([]string(nil), resource.Requires...),
				Notifies: append([]string(nil), resource.Notifies...),
			})
		case manifestdomain.ResourceTypeService:
			enabled := resource.Enabled
			resources = append(resources, manifestdomain.ServiceResource{
				ID:       resource.ID,
				Name:     resource.Name,
				State:    resource.State,
				Enabled:  &enabled,
				Requires: append([]string(nil), resource.Requires...),
				Notifies: append([]string(nil), resource.Notifies...),
			})
		default:
			return manifestdomain.CompiledManifest{}, fmt.Errorf("unsupported resource type %q", resource.Type)
		}
	}
	return (manifestdomain.Manifest{
		APIVersion: manifest.APIVersion,
		Kind:       manifest.Kind,
		Name:       manifest.Name,
		Labels:     cloneMap(manifest.Labels),
		Target: manifestdomain.TargetSpec{
			Hosts:          append([]string(nil), target.Hosts...),
			SelectorLabels: cloneMap(selector.MatchLabels),
		},
		Resources: resources,
	}).Validate()
}

func resultEvent(work *scmv1.WorkItem, result agentdomain.ResourceResult) *scmv1.ApplyEvent {
	level := "info"
	if !result.Changed {
		level = "debug"
	}
	return &scmv1.ApplyEvent{
		ID:         newEventID(work.WorkItemID, result.ResourceID),
		ApplyID:    work.ApplyID,
		HostID:     work.HostID,
		WorkItemID: work.WorkItemID,
		Level:      level,
		Phase:      agentdomain.PhaseApplying,
		Message:    fmt.Sprintf("%s (%s): %s", result.ResourceID, result.ResourceType, result.Message),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
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

func newEventID(workItemID string, suffix string) string {
	return fmt.Sprintf("%s-%s-%d", workItemID, suffix, time.Now().UTC().UnixNano())
}
