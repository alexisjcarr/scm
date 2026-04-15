package app

import (
	"context"
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
}
