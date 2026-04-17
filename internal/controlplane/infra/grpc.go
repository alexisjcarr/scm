package infra

import (
	"context"
	"errors"
	"time"

	cpapp "github.com/alexisjcarr/scm/internal/controlplane/app"
	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	"github.com/alexisjcarr/scm/internal/controlplane/inventory"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

// GRPCServer exposes the control-plane application service through the API package.
type GRPCServer struct {
	scmv1.UnimplementedApplyServiceServer
	scmv1.UnimplementedAgentServiceServer

	service *cpapp.Service
}

func NewGRPCServer(service *cpapp.Service) *GRPCServer {
	return &GRPCServer{service: service}
}

func (s *GRPCServer) SubmitApply(ctx context.Context, req *scmv1.SubmitApplyRequest) (*scmv1.SubmitApplyResponse, error) {
	compiled, err := apiManifestToDomain(req.Manifest)
	if err != nil {
		return nil, err
	}
	apply, workItems, err := s.service.SubmitApply(ctx, compiled, req.RawManifest, req.SubmittedBy)
	if err != nil {
		return nil, err
	}
	return &scmv1.SubmitApplyResponse{
		ApplyID:     apply.ApplyID,
		Status:      apply.Status,
		TargetCount: int32(len(workItems)),
	}, nil
}

func (s *GRPCServer) GetApply(ctx context.Context, req *scmv1.GetApplyRequest) (*scmv1.ApplySummary, error) {
	apply, workItems, err := s.service.GetApply(ctx, req.ApplyID)
	if err != nil {
		return nil, err
	}
	return toApplySummary(apply, workItems), nil
}

func (s *GRPCServer) StreamApplyEvents(req *scmv1.StreamApplyEventsRequest, stream scmv1.ApplyService_StreamApplyEventsServer) error {
	offset := req.FromOffset
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		events, err := s.service.ListEvents(stream.Context(), req.ApplyID, offset)
		if err != nil {
			return err
		}
		for _, event := range events {
			offset++
			if err := stream.Send(&scmv1.ApplyEvent{
				ID:         event.ID,
				ApplyID:    event.ApplyID,
				HostID:     event.HostID,
				WorkItemID: event.WorkItemID,
				Level:      event.Level,
				Phase:      event.Phase,
				Message:    event.Message,
				CreatedAt:  event.CreatedAt.Format(time.RFC3339Nano),
			}); err != nil {
				return err
			}
		}

		apply, _, err := s.service.GetApply(stream.Context(), req.ApplyID)
		if err != nil {
			return err
		}
		if apply.Status == cpdomain.ApplyStatusCompleted || apply.Status == cpdomain.ApplyStatusFailed || apply.Status == cpdomain.ApplyStatusStalled {
			return nil
		}

		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
		}
	}
}

func (s *GRPCServer) RegisterAgent(ctx context.Context, req *scmv1.RegisterAgentRequest) (*scmv1.RegisterAgentResponse, error) {
	agent, err := s.service.RegisterAgent(ctx, inventory.RegisterInput{
		AgentID:      req.AgentID,
		HostID:       req.HostID,
		Version:      req.Version,
		Labels:       cloneMap(req.Labels),
		Capabilities: append([]string(nil), req.Capabilities...),
	})
	if err != nil {
		return nil, err
	}
	return &scmv1.RegisterAgentResponse{AgentID: agent.AgentID}, nil
}

func (s *GRPCServer) Heartbeat(ctx context.Context, req *scmv1.HeartbeatRequest) (*scmv1.HeartbeatResponse, error) {
	if err := s.service.Heartbeat(ctx, req.AgentID, req.Idle, req.CurrentWorkItemID); err != nil {
		return nil, err
	}
	return &scmv1.HeartbeatResponse{Status: "ok"}, nil
}

func (s *GRPCServer) FetchWork(ctx context.Context, req *scmv1.FetchWorkRequest) (*scmv1.FetchWorkResponse, error) {
	workItem, err := s.service.FetchWork(ctx, req.AgentID)
	if err != nil {
		return nil, err
	}
	if workItem == nil {
		return &scmv1.FetchWorkResponse{HasWork: false}, nil
	}
	return &scmv1.FetchWorkResponse{
		HasWork: true,
		WorkItem: &scmv1.WorkItem{
			WorkItemID:     workItem.WorkItemID,
			ApplyID:        workItem.ApplyID,
			HostID:         workItem.HostID,
			LeaseToken:     workItem.LeaseToken,
			State:          workItem.State,
			ManifestJSON:   workItem.ManifestJSON,
			AssignedAt:     formatTimePtr(workItem.AssignedAt),
			LeaseExpiresAt: formatTimePtr(workItem.LeaseExpiresAt),
		},
	}, nil
}

func (s *GRPCServer) ReportWorkStatus(ctx context.Context, req *scmv1.ReportWorkStatusRequest) (*scmv1.ReportWorkStatusResponse, error) {
	events := make([]cpdomain.ApplyEvent, 0, len(req.Events))
	for _, event := range req.Events {
		ts, err := time.Parse(time.RFC3339Nano, event.CreatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, cpdomain.ApplyEvent{
			ID:         event.ID,
			ApplyID:    event.ApplyID,
			HostID:     event.HostID,
			WorkItemID: event.WorkItemID,
			Level:      event.Level,
			Phase:      event.Phase,
			Message:    event.Message,
			CreatedAt:  ts,
		})
	}
	if err := s.service.ReportWorkStatus(ctx, req.AgentID, req.WorkItemID, req.LeaseToken, req.State, req.Summary, events); err != nil {
		return nil, err
	}
	return &scmv1.ReportWorkStatusResponse{Status: "ok"}, nil
}

func (s *GRPCServer) ListAgents(ctx context.Context, _ *scmv1.ListAgentsRequest) (*scmv1.ListAgentsResponse, error) {
	agents, err := s.service.GetAgents(ctx)
	if err != nil {
		return nil, err
	}
	response := &scmv1.ListAgentsResponse{Agents: make([]*scmv1.AgentSummary, 0, len(agents))}
	for _, agent := range agents {
		response.Agents = append(response.Agents, &scmv1.AgentSummary{
			AgentID:           agent.AgentID,
			HostID:            agent.HostID,
			Idle:              agent.Idle,
			CurrentWorkItemID: agent.CurrentWorkItemID,
			Version:           agent.Version,
			Labels:            cloneMap(agent.Labels),
			LastSeenAt:        agent.LastSeenAt.Format(time.RFC3339Nano),
		})
	}
	return response, nil
}

func toApplySummary(apply cpdomain.Apply, workItems []cpdomain.WorkItem) *scmv1.ApplySummary {
	summary := &scmv1.ApplySummary{
		ApplyID:     apply.ApplyID,
		Name:        apply.Name,
		Status:      apply.Status,
		SubmittedBy: apply.SubmittedBy,
		CreatedAt:   apply.CreatedAt.Format(time.RFC3339Nano),
		Targets:     make([]*scmv1.ApplyTargetSummary, 0, len(workItems)),
	}
	for _, work := range workItems {
		summary.Targets = append(summary.Targets, &scmv1.ApplyTargetSummary{
			HostID:     work.HostID,
			WorkItemID: work.WorkItemID,
			Status:     work.State,
			Summary:    work.Summary,
			UpdatedAt:  work.UpdatedAt.Format(time.RFC3339Nano),
		})
	}
	return summary
}

func apiManifestToDomain(manifest *scmv1.Manifest) (manifestdomain.CompiledManifest, error) {
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
		}
	}

	compiled, err := (manifestdomain.Manifest{
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
	if err != nil {
		return manifestdomain.CompiledManifest{}, err
	}
	return compiled, nil
}

func formatTimePtr(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.Format(time.RFC3339Nano)
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
