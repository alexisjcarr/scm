package app

import (
	"context"
	"strings"
	"testing"

	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

type fakeClient struct {
	fetchResp  *scmv1.FetchWorkResponse
	reports    []*scmv1.ReportWorkStatusRequest
	heartbeats []*scmv1.HeartbeatRequest
}

func (f *fakeClient) RegisterAgent(context.Context, *scmv1.RegisterAgentRequest, ...grpc.CallOption) (*scmv1.RegisterAgentResponse, error) {
	return &scmv1.RegisterAgentResponse{AgentID: "agent-1"}, nil
}
func (f *fakeClient) Heartbeat(_ context.Context, req *scmv1.HeartbeatRequest, _ ...grpc.CallOption) (*scmv1.HeartbeatResponse, error) {
	f.heartbeats = append(f.heartbeats, req)
	return &scmv1.HeartbeatResponse{Status: "ok"}, nil
}
func (f *fakeClient) FetchWork(context.Context, *scmv1.FetchWorkRequest, ...grpc.CallOption) (*scmv1.FetchWorkResponse, error) {
	return f.fetchResp, nil
}
func (f *fakeClient) ReportWorkStatus(_ context.Context, req *scmv1.ReportWorkStatusRequest, _ ...grpc.CallOption) (*scmv1.ReportWorkStatusResponse, error) {
	f.reports = append(f.reports, req)
	return &scmv1.ReportWorkStatusResponse{Status: "ok"}, nil
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

	service := NewService(client, fakeRunner{}, nil, nil, "agent-1", "node-1", "dev", nil, nil, t.TempDir())
	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if got := len(client.reports); got != 2 {
		t.Fatalf("expected two status reports, got %d", got)
	}
	if got := len(client.heartbeats); got < 2 {
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

	service := NewService(client, fakeRunner{}, nil, nil, "agent-1", "node-1", "dev", nil, nil, t.TempDir())
	err := service.RunOnce(context.Background())
	if err == nil {
		t.Fatal("RunOnce returned nil, want host mismatch error")
	}
	if !strings.Contains(err.Error(), "refusing work item") {
		t.Fatalf("RunOnce error = %v, want host mismatch refusal", err)
	}
	if got := len(client.reports); got != 0 {
		t.Fatalf("expected no work reports after host mismatch, got %d", got)
	}
	if got := len(client.heartbeats); got != 1 {
		t.Fatalf("expected only the initial idle heartbeat before refusal, got %d", got)
	}
}
