package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

type fakeClient struct {
	mu         sync.Mutex
	fetchResp  *scmv1.FetchWorkResponse
	reports    []*scmv1.ReportWorkStatusRequest
	heartbeats []*scmv1.HeartbeatRequest
}

func (f *fakeClient) RegisterAgent(context.Context, *scmv1.RegisterAgentRequest, ...grpc.CallOption) (*scmv1.RegisterAgentResponse, error) {
	return &scmv1.RegisterAgentResponse{AgentID: "agent-1"}, nil
}
func (f *fakeClient) Heartbeat(_ context.Context, req *scmv1.HeartbeatRequest, _ ...grpc.CallOption) (*scmv1.HeartbeatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heartbeats = append(f.heartbeats, req)
	return &scmv1.HeartbeatResponse{Status: "ok"}, nil
}
func (f *fakeClient) FetchWork(context.Context, *scmv1.FetchWorkRequest, ...grpc.CallOption) (*scmv1.FetchWorkResponse, error) {
	return f.fetchResp, nil
}
func (f *fakeClient) ReportWorkStatus(_ context.Context, req *scmv1.ReportWorkStatusRequest, _ ...grpc.CallOption) (*scmv1.ReportWorkStatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, req)
	return &scmv1.ReportWorkStatusResponse{Status: "ok"}, nil
}

func (f *fakeClient) heartbeatCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.heartbeats)
}

func (f *fakeClient) reportCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reports)
}

type fakeRunner struct{}

func (fakeRunner) Prepare(context.Context, *scmv1.WorkItem, string) ([]*scmv1.ApplyEvent, error) {
	return []*scmv1.ApplyEvent{{ID: "evt-1"}}, nil
}

func (fakeRunner) Execute(context.Context, *scmv1.WorkItem) (string, []*scmv1.ApplyEvent, string, error) {
	return "done", []*scmv1.ApplyEvent{{ID: "evt-2"}}, "completed", nil
}

func (fakeRunner) Complete(context.Context, string, string, string) error { return nil }

func TestRunOnceDelegatesToRuntimeRunner(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		fetchResp: &scmv1.FetchWorkResponse{
			HasWork: true,
			WorkItem: &scmv1.WorkItem{
				WorkItemID:   "work-1",
				ApplyID:      "apply-1",
				HostID:       "node-1",
				LeaseToken:   "lease",
				ManifestJSON: "{}",
			},
		},
	}

	service := NewService(client, fakeRunner{}, nil, nil, "agent-1", "node-1", "dev", nil, nil, "test-token", t.TempDir())
	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if got := client.reportCount(); got != 2 {
		t.Fatalf("expected two status reports, got %d", got)
	}
	if got := client.heartbeatCount(); got < 2 {
		t.Fatalf("expected heartbeats before and after work, got %d", got)
	}

	snapshot := service.StatusSnapshot()
	if !snapshot.ConnectedToControlPlane {
		t.Fatalf("expected service to remain connected after successful run")
	}
	if snapshot.State != "ready" {
		t.Fatalf("expected service to return to ready state, got %q", snapshot.State)
	}
	if snapshot.CurrentWorkItemID != "" {
		t.Fatalf("expected no current work item after completion, got %q", snapshot.CurrentWorkItemID)
	}
	if snapshot.LastSuccessfulHeartbeatAt == "" {
		t.Fatalf("expected a heartbeat timestamp in snapshot")
	}
	if snapshot.LastWorkReportAt == "" {
		t.Fatalf("expected a work report timestamp in snapshot")
	}
}

func TestRunOnceRejectsWorkForDifferentHost(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		fetchResp: &scmv1.FetchWorkResponse{
			HasWork: true,
			WorkItem: &scmv1.WorkItem{
				WorkItemID:   "work-1",
				ApplyID:      "apply-1",
				HostID:       "node-2",
				LeaseToken:   "lease",
				ManifestJSON: "{}",
			},
		},
	}

	service := NewService(client, fakeRunner{}, nil, nil, "agent-1", "node-1", "dev", nil, nil, "test-token", t.TempDir())
	err := service.RunOnce(context.Background())
	if err == nil {
		t.Fatal("RunOnce returned nil, want host mismatch error")
	}
	if !strings.Contains(err.Error(), "refusing work item") {
		t.Fatalf("RunOnce error = %v, want host mismatch refusal", err)
	}
	if got := client.reportCount(); got != 0 {
		t.Fatalf("expected no work reports after host mismatch, got %d", got)
	}
	if got := client.heartbeatCount(); got != 1 {
		t.Fatalf("expected only the initial idle heartbeat before refusal, got %d", got)
	}
}

type blockingRunner struct {
	started chan struct{}
	release chan struct{}
}

func (b blockingRunner) Prepare(context.Context, *scmv1.WorkItem, string) ([]*scmv1.ApplyEvent, error) {
	return []*scmv1.ApplyEvent{{ID: "evt-1"}}, nil
}

func (b blockingRunner) Execute(context.Context, *scmv1.WorkItem) (string, []*scmv1.ApplyEvent, string, error) {
	close(b.started)
	<-b.release
	return "done", []*scmv1.ApplyEvent{{ID: "evt-2"}}, "completed", nil
}

func (b blockingRunner) Complete(context.Context, string, string, string) error { return nil }

func TestRunOnceSendsProgressHeartbeatsWhileExecuting(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		fetchResp: &scmv1.FetchWorkResponse{
			HasWork: true,
			WorkItem: &scmv1.WorkItem{
				WorkItemID:   "work-1",
				ApplyID:      "apply-1",
				HostID:       "node-1",
				LeaseToken:   "lease",
				ManifestJSON: "{}",
			},
		},
	}
	runner := blockingRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := NewService(client, runner, nil, nil, "agent-1", "node-1", "dev", nil, nil, "test-token", t.TempDir())
	service.progressInterval = 5 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		done <- service.RunOnce(context.Background())
	}()

	<-runner.started
	time.Sleep(20 * time.Millisecond)
	close(runner.release)

	if err := <-done; err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if got := client.heartbeatCount(); got < 3 {
		t.Fatalf("expected an extra progress heartbeat during execution, got %d heartbeats", got)
	}
}
