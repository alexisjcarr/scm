package runtime

import (
	"context"
	"testing"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

type fakeRepo struct{}

func (fakeRepo) SaveWork(context.Context, agentdomain.LocalApply) error              { return nil }
func (fakeRepo) UpdateWork(context.Context, string, string, string, time.Time) error { return nil }

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

func TestExecuteTriggersNotifyFollowUp(t *testing.T) {
	t.Parallel()

	manifestJSON := `{"api_version":"scm/v1","kind":"Manifest","name":"nginx","target":{"hosts":["node-1"],"selector":{"match_labels":{}}},"resources":[{"id":"cfg","type":"file","path":"/tmp/nginx.conf","content":"hello","mode":"0644","state":"present","notifies":["svc"]},{"id":"svc","type":"service","name":"nginx","state":"running","enabled":true}]}`

	runner := NewRunner(fakeRepo{}, fakeBackend{changed: map[string]bool{"cfg": true}}, nil)
	summary, events, state, err := runner.Execute(context.Background(), &scmv1.WorkItem{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "node-1",
		LeaseToken:   "lease",
		ManifestJSON: manifestJSON,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if state != "completed" {
		t.Fatalf("expected completed state, got %q", state)
	}
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if len(events) < 3 {
		t.Fatalf("expected planning/apply/notify events, got %d", len(events))
	}
}
