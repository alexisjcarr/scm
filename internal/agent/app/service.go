package app

import (
	"context"
	"io"
	"log/slog"
	"time"

	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
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

// RuntimeRunner handles local persistence and reconciliation of a claimed work item.
type RuntimeRunner interface {
	Prepare(context.Context, *scmv1.WorkItem, string) ([]*scmv1.ApplyEvent, error)
	Execute(context.Context, *scmv1.WorkItem) (string, []*scmv1.ApplyEvent, string, error)
	Complete(context.Context, string, string, string) error
}

// Service drives the host agent pull loop while delegating execution to the runtime runner.
type Service struct {
	client           ControlPlaneClient
	runner           RuntimeRunner
	logger           *slog.Logger
	metrics          *platformmetrics.AgentMetrics
	agentID          string
	hostID           string
	version          string
	labels           map[string]string
	capabilities     []string
	manifestCacheDir string
}

func NewService(client ControlPlaneClient, runner RuntimeRunner, logger *slog.Logger, metrics *platformmetrics.AgentMetrics, agentID, hostID, version string, labels map[string]string, capabilities []string, manifestCacheDir string) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		client:           client,
		runner:           runner,
		logger:           logger,
		metrics:          metrics,
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

// RunOnce fetches work when idle and delegates execution to the runtime runner.
func (s *Service) RunOnce(ctx context.Context) error {
	if s.metrics != nil {
		s.metrics.Polls.Inc()
	}
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
	if s.metrics != nil {
		s.metrics.WorkStarted.Inc()
	}
	if err := s.Heartbeat(ctx, false, work.WorkItemID); err != nil {
		return err
	}

	events, err := s.runner.Prepare(ctx, work, s.manifestCacheDir)
	if err != nil {
		return err
	}
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

	summary, reportEvents, finalState, err := s.runner.Execute(ctx, work)
	if err != nil {
		reportEvents = append(reportEvents, &scmv1.ApplyEvent{
			ID:         work.WorkItemID + "-error",
			ApplyID:    work.ApplyID,
			HostID:     work.HostID,
			WorkItemID: work.WorkItemID,
			Level:      "error",
			Phase:      "terminal",
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
	if err := s.runner.Complete(ctx, work.WorkItemID, finalState, summary); err != nil {
		return err
	}
	return s.Heartbeat(ctx, true, "")
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
