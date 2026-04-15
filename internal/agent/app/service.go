package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

// ControlPlaneClient is the subset of agent RPCs used by the runtime.
type ControlPlaneClient interface {
	RegisterAgent(context.Context, *scmv1.RegisterAgentRequest, ...grpc.CallOption) (*scmv1.RegisterAgentResponse, error)
	Heartbeat(context.Context, *scmv1.HeartbeatRequest, ...grpc.CallOption) (*scmv1.HeartbeatResponse, error)
	FetchWork(context.Context, *scmv1.FetchWorkRequest, ...grpc.CallOption) (*scmv1.FetchWorkResponse, error)
	ReportWorkStatus(context.Context, *scmv1.ReportWorkStatusRequest, ...grpc.CallOption) (*scmv1.ReportWorkStatusResponse, error)
}

// Repository stores host-local work metadata.
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

// Service drives the host agent pull loop.
type Service struct {
	client           ControlPlaneClient
	repo             Repository
	backend          Backend
	logger           *slog.Logger
	agentID          string
	hostID           string
	version          string
	labels           map[string]string
	capabilities     []string
	manifestCacheDir string
}

func NewService(client ControlPlaneClient, repo Repository, backend Backend, logger *slog.Logger, agentID, hostID, version string, labels map[string]string, capabilities []string, manifestCacheDir string) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		client:           client,
		repo:             repo,
		backend:          backend,
		logger:           logger,
		agentID:          agentID,
		hostID:           hostID,
		version:          version,
		labels:           cloneMap(labels),
		capabilities:     append([]string(nil), capabilities...),
		manifestCacheDir: manifestCacheDir,
	}
}

func (s *Service) Register(ctx context.Context) error {
	_, err := s.client.RegisterAgent(ctx, &scmv1.RegisterAgentRequest{
		AgentID:      s.agentID,
		HostID:       s.hostID,
		Version:      s.version,
		Labels:       cloneMap(s.labels),
		Capabilities: append([]string(nil), s.capabilities...),
	})
	return err
}

func (s *Service) Heartbeat(ctx context.Context, idle bool, currentWorkItemID string) error {
	_, err := s.client.Heartbeat(ctx, &scmv1.HeartbeatRequest{
		AgentID:           s.agentID,
		Idle:              idle,
		CurrentWorkItemID: currentWorkItemID,
	})
	return err
}

// RunOnce fetches work when idle and reconciles it locally.
func (s *Service) RunOnce(ctx context.Context) error {
	if err := s.Heartbeat(ctx, true, ""); err != nil {
		return err
	}

	resp, err := s.client.FetchWork(ctx, &scmv1.FetchWorkRequest{AgentID: s.agentID})
	if err != nil {
		return err
	}
	if !resp.HasWork || resp.WorkItem == nil {
		return nil
	}

	work := resp.WorkItem
	if err := s.Heartbeat(ctx, false, work.WorkItemID); err != nil {
		return err
	}

	if err := s.persistLocalWork(ctx, work); err != nil {
		return err
	}

	events := []*scmv1.ApplyEvent{{
		ID:         newEventID(work.WorkItemID, agentdomain.PhasePersisted),
		ApplyID:    work.ApplyID,
		HostID:     work.HostID,
		WorkItemID: work.WorkItemID,
		Level:      "info",
		Phase:      agentdomain.PhasePersisted,
		Message:    "manifest persisted locally",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}}
	if _, err := s.client.ReportWorkStatus(ctx, &scmv1.ReportWorkStatusRequest{
		AgentID:    s.agentID,
		WorkItemID: work.WorkItemID,
		LeaseToken: work.LeaseToken,
		State:      "running",
		Summary:    "planning reconciliation",
		Events:     events,
	}); err != nil {
		return err
	}

	summary, reportEvents, finalState, err := s.executeWork(ctx, work)
	if err != nil {
		reportEvents = append(reportEvents, &scmv1.ApplyEvent{
			ID:         newEventID(work.WorkItemID, "error"),
			ApplyID:    work.ApplyID,
			HostID:     work.HostID,
			WorkItemID: work.WorkItemID,
			Level:      "error",
			Phase:      agentdomain.PhaseTerminal,
			Message:    err.Error(),
			CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		})
		finalState = "failed"
		summary = err.Error()
	}

	if _, reportErr := s.client.ReportWorkStatus(ctx, &scmv1.ReportWorkStatusRequest{
		AgentID:    s.agentID,
		WorkItemID: work.WorkItemID,
		LeaseToken: work.LeaseToken,
		State:      finalState,
		Summary:    summary,
		Events:     reportEvents,
	}); reportErr != nil {
		return reportErr
	}

	if err := s.repo.UpdateWork(ctx, work.WorkItemID, finalState, summary, time.Now().UTC()); err != nil {
		return err
	}
	return s.Heartbeat(ctx, true, "")
}

func (s *Service) persistLocalWork(ctx context.Context, work *scmv1.WorkItem) error {
	if err := os.MkdirAll(s.manifestCacheDir, 0o755); err != nil {
		return fmt.Errorf("create manifest cache dir: %w", err)
	}
	cachePath := filepath.Join(s.manifestCacheDir, work.WorkItemID+".json")
	if err := os.WriteFile(cachePath, []byte(work.ManifestJSON), 0o644); err != nil {
		return fmt.Errorf("write manifest cache: %w", err)
	}
	return s.repo.SaveWork(ctx, agentdomain.LocalApply{
		WorkItemID:    work.WorkItemID,
		ApplyID:       work.ApplyID,
		ManifestJSON:  work.ManifestJSON,
		State:         agentdomain.PhasePersisted,
		Summary:       "manifest cached locally",
		LeaseToken:    work.LeaseToken,
		LastUpdatedAt: time.Now().UTC(),
	})
}

func (s *Service) executeWork(ctx context.Context, work *scmv1.WorkItem) (string, []*scmv1.ApplyEvent, string, error) {
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
		result, err := s.applyResource(ctx, resource, false)
		events = append(events, resultEvent(work, result))
		if err != nil {
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
			result, err := s.applyResource(ctx, resource, true)
			events = append(events, resultEvent(work, result))
			if err != nil {
				return "", events, "failed", err
			}
		}
	}

	return fmt.Sprintf("applied %d resources", len(compiled.OrderedResources)), events, "completed", nil
}

func (s *Service) applyResource(ctx context.Context, resource manifestdomain.Resource, notifyOnly bool) (agentdomain.ResourceResult, error) {
	switch typed := resource.(type) {
	case manifestdomain.PackageResource:
		changed, message, err := s.backend.EnsurePackage(ctx, typed)
		return agentdomain.ResourceResult{ResourceID: typed.ID, ResourceType: typed.GetType(), Changed: changed, Message: message}, err
	case manifestdomain.FileResource:
		changed, message, err := s.backend.EnsureFile(ctx, typed)
		return agentdomain.ResourceResult{ResourceID: typed.ID, ResourceType: typed.GetType(), Changed: changed, Message: message}, err
	case manifestdomain.ServiceResource:
		changed, message, err := s.backend.EnsureService(ctx, typed, notifyOnly)
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
