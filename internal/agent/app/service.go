package app

import (
	"context"
	"fmt"
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
	status           *statusTracker
	agentID          string
	hostID           string
	version          string
	labels           map[string]string
	capabilities     []string
	authToken        string
	manifestCacheDir string
	progressInterval time.Duration
}

func NewService(client ControlPlaneClient, runner RuntimeRunner, logger *slog.Logger, metrics *platformmetrics.AgentMetrics, agentID, hostID, version string, labels map[string]string, capabilities []string, authToken string, manifestCacheDir string) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		client:           client,
		runner:           runner,
		logger:           logger,
		metrics:          metrics,
		status:           newStatusTracker(agentID, hostID, version),
		agentID:          agentID,
		hostID:           hostID,
		version:          version,
		labels:           cloneMap(labels),
		capabilities:     append([]string(nil), capabilities...),
		authToken:        authToken,
		manifestCacheDir: manifestCacheDir,
		progressInterval: 30 * time.Second,
	}
}

func (s *Service) Register(ctx context.Context) error {
	_, err := s.client.RegisterAgent(ctx, &scmv1.RegisterAgentRequest{
		AgentID:      s.agentID,
		HostID:       s.hostID,
		AuthToken:    s.authToken,
		Version:      s.version,
		Labels:       cloneMap(s.labels),
		Capabilities: append([]string(nil), s.capabilities...),
	})
	if err != nil {
		s.status.markConnectionFailure()
		return err
	}
	s.status.markRegisterSuccess()
	return err
}

func (s *Service) Heartbeat(ctx context.Context, idle bool, currentWorkItemID string) error {
	_, err := s.client.Heartbeat(ctx, &scmv1.HeartbeatRequest{
		AgentID:           s.agentID,
		AuthToken:         s.authToken,
		Idle:              idle,
		CurrentWorkItemID: currentWorkItemID,
	})
	if err != nil {
		s.status.markConnectionFailure()
		return err
	}
	s.status.markHeartbeatSuccess(time.Now().UTC(), idle, currentWorkItemID)
	return err
}

// StatusSnapshot returns the current operator-facing agent status.
func (s *Service) StatusSnapshot() StatusSnapshot {
	return s.status.snapshot()
}

// RunOnce fetches work when idle and delegates execution to the runtime runner.
func (s *Service) RunOnce(ctx context.Context) error {
	if s.metrics != nil {
		s.metrics.Polls.Inc()
	}
	if err := s.Heartbeat(ctx, true, ""); err != nil {
		return err
	}

	resp, err := s.client.FetchWork(ctx, &scmv1.FetchWorkRequest{AgentID: s.agentID, AuthToken: s.authToken})
	if err != nil {
		s.status.markConnectionFailure()
		return err
	}
	if !resp.HasWork || resp.WorkItem == nil {
		return nil
	}

	work := resp.WorkItem
	if work.HostID != "" && work.HostID != s.hostID {
		return fmt.Errorf("refusing work item %s for host %q on agent host %q", work.WorkItemID, work.HostID, s.hostID)
	}
	if s.metrics != nil {
		s.metrics.WorkStarted.Inc()
	}
	if err := s.Heartbeat(ctx, false, work.WorkItemID); err != nil {
		return err
	}
	stopProgressHeartbeats := s.startProgressHeartbeats(ctx, work.WorkItemID)
	defer stopProgressHeartbeats()

	events, err := s.runner.Prepare(ctx, work, s.manifestCacheDir)
	if err != nil {
		return err
	}
	if _, err := s.client.ReportWorkStatus(ctx, &scmv1.ReportWorkStatusRequest{
		AgentID:    s.agentID,
		AuthToken:  s.authToken,
		WorkItemID: work.WorkItemID,
		LeaseToken: work.LeaseToken,
		State:      "running",
		Summary:    "planning reconciliation",
		Events:     events,
	}); err != nil {
		s.status.markConnectionFailure()
		return err
	}
	s.status.markWorkReportSuccess(time.Now().UTC(), work.WorkItemID, "running")

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
		AuthToken:  s.authToken,
		WorkItemID: work.WorkItemID,
		LeaseToken: work.LeaseToken,
		State:      finalState,
		Summary:    summary,
		Events:     reportEvents,
	}); reportErr != nil {
		s.status.markConnectionFailure()
		return reportErr
	}
	s.status.markWorkReportSuccess(time.Now().UTC(), work.WorkItemID, finalState)
	if err := s.runner.Complete(ctx, work.WorkItemID, finalState, summary); err != nil {
		return err
	}
	return s.Heartbeat(ctx, true, "")
}

func (s *Service) startProgressHeartbeats(ctx context.Context, workItemID string) func() {
	if s.progressInterval <= 0 {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(s.progressInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := s.Heartbeat(heartbeatCtx, false, workItemID); err != nil {
					s.logger.Warn("progress heartbeat failed", "work_item_id", workItemID, "error", err)
				}
			}
		}
	}()
	return cancel
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
