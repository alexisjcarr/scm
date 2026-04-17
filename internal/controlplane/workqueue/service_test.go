package workqueue

import (
	"context"
	"strings"
	"testing"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeStore struct {
	claimed bool
}

func (f *fakeStore) ClaimNextWork(context.Context, string, time.Duration, time.Time) (*cpdomain.WorkItem, error) {
	f.claimed = true
	return &cpdomain.WorkItem{WorkItemID: "work-1"}, nil
}

func (f *fakeStore) UpdateWork(context.Context, string, string, string, string, string, []cpdomain.ApplyEvent, time.Time) error {
	return nil
}

func (f *fakeStore) ReconcileStalled(context.Context, time.Time, time.Duration) error {
	return nil
}

func TestFetchRequiresAgentID(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{}, fakeClock{now: time.Unix(1, 0)}, time.Minute, nil)
	_, err := svc.Fetch(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "agent_id") {
		t.Fatalf("expected missing agent_id error, got %v", err)
	}
}

func TestReportRejectsUnknownState(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{}, fakeClock{now: time.Unix(1, 0)}, time.Minute, nil)
	err := svc.Report(context.Background(), "agent-1", "work-1", "lease", "pending", "", nil)
	if err == nil || !strings.Contains(err.Error(), "invalid work state") {
		t.Fatalf("expected invalid state error, got %v", err)
	}
}
