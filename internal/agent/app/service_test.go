package app

import (
	"context"
	"testing"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

type fakeClient struct {
	fetchResp *scmv1.FetchWorkResponse
	reports   []*scmv1.ReportWorkStatusRequest
}

func (f *fakeClient) RegisterAgent(context.Context, *scmv1.RegisterAgentRequest, ...grpc.CallOption) (*scmv1.RegisterAgentResponse, error) {
	return &scmv1.RegisterAgentResponse{AgentID: "agent-1"}, nil
}
func (f *fakeClient) Heartbeat(context.Context, *scmv1.HeartbeatRequest, ...grpc.CallOption) (*scmv1.HeartbeatResponse, error) {
	return &scmv1.HeartbeatResponse{Status: "ok"}, nil
}
func (f *fakeClient) FetchWork(context.Context, *scmv1.FetchWorkRequest, ...grpc.CallOption) (*scmv1.FetchWorkResponse, error) {
	return f.fetchResp, nil
}
func (f *fakeClient) ReportWorkStatus(_ context.Context, req *scmv1.ReportWorkStatusRequest, _ ...grpc.CallOption) (*scmv1.ReportWorkStatusResponse, error) {
	f.reports = append(f.reports, req)
	return &scmv1.ReportWorkStatusResponse{Status: "ok"}, nil
}

type fakeBackend struct {
	changed map[string]bool
}

func (b fakeBackend) EnsurePackage(context.Context, manifestdomain.PackageResource) (bool, string, error) {
	return false, "package already converged", nil
}
func (b fakeBackend) EnsureFile(_ context.Context, resource manifestdomain.FileResource) (bool, string, error) {
	return b.changed[resource.ID], "file reconciled", nil
}
func (b fakeBackend) EnsureService(_ context.Context, _ manifestdomain.ServiceResource, notifyOnly bool) (bool, string, error) {
	if notifyOnly {
		return true, "service restarted from notify", nil
	}
	return false, "service converged", nil
}

func TestRunOnceTriggersNotifyFollowUp(t *testing.T) {
	t.Parallel()

	manifestJSON := `{"api_version":"scm/v1","kind":"Manifest","name":"nginx","target":{"hosts":["node-1"],"selector":{"match_labels":{}}},"resources":[{"id":"cfg","type":"file","path":"/tmp/nginx.conf","content":"hello","mode":"0644","state":"present","notifies":["svc"]},{"id":"svc","type":"service","name":"nginx","state":"running","enabled":true}]}`

	client := &fakeClient{
		fetchResp: &scmv1.FetchWorkResponse{
			HasWork: true,
			WorkItem: &scmv1.WorkItem{
				WorkItemID:   "work-1",
				ApplyID:      "apply-1",
				HostID:       "node-1",
				LeaseToken:   "lease",
				ManifestJSON: manifestJSON,
			},
		},
	}

	service := NewService(client, noopRepo{}, fakeBackend{changed: map[string]bool{"cfg": true}}, nil, "agent-1", "node-1", "dev", nil, nil, t.TempDir())
	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if got := len(client.reports); got != 2 {
		t.Fatalf("expected two status reports, got %d", got)
	}
	if client.reports[1].State != "completed" {
		t.Fatalf("expected final report to be completed, got %q", client.reports[1].State)
	}
}

type noopRepo struct{}

func (noopRepo) SaveWork(context.Context, agentdomain.LocalApply) error              { return nil }
func (noopRepo) UpdateWork(context.Context, string, string, string, time.Time) error { return nil }
