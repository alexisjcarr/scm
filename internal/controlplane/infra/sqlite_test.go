package infra

import (
	"context"
	"testing"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
)

func TestClaimNextWorkOnlyAllowsSingleWinner(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file:claim-next-work?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	now := time.Unix(1700000000, 0).UTC()
	if err := repo.UpsertAgent(context.Background(), cpdomain.Agent{
		AgentID:    "agent-1",
		HostID:     "host-1",
		Version:    "dev",
		Labels:     map[string]string{"role": "web"},
		Idle:       true,
		LastSeenAt: now,
	}); err != nil {
		t.Fatalf("UpsertAgent returned error: %v", err)
	}
	if err := repo.UpsertAgent(context.Background(), cpdomain.Agent{
		AgentID:    "agent-2",
		HostID:     "host-2",
		Version:    "dev",
		Labels:     map[string]string{"role": "web"},
		Idle:       true,
		LastSeenAt: now,
	}); err != nil {
		t.Fatalf("UpsertAgent returned error: %v", err)
	}

	err = repo.CreateApply(context.Background(), cpdomain.Apply{
		ApplyID:      "apply-1",
		Name:         "nginx",
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  "tester",
		RawManifest:  "raw",
		ManifestJSON: "{}",
		CreatedAt:    now,
	}, []cpdomain.WorkItem{{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "host-1",
		State:        cpdomain.WorkStatePending,
		ManifestJSON: "{}",
		UpdatedAt:    now,
	}}, nil)
	if err != nil {
		t.Fatalf("CreateApply returned error: %v", err)
	}

	first, err := repo.ClaimNextWork(context.Background(), "agent-1", time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimNextWork returned error: %v", err)
	}
	if first == nil {
		t.Fatal("expected first claim to receive work")
	}

	second, err := repo.ClaimNextWork(context.Background(), "agent-2", time.Minute, now)
	if err != nil {
		t.Fatalf("second ClaimNextWork returned error: %v", err)
	}
	if second != nil {
		t.Fatalf("expected second claim to return nil, got %+v", second)
	}
}

func TestClaimNextWorkOnlyReturnsWorkForAgentHost(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file:claim-next-work-by-host?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	now := time.Unix(1700000000, 0).UTC()
	for _, agent := range []cpdomain.Agent{
		{AgentID: "agent-1", HostID: "host-1", Version: "dev", Idle: true, LastSeenAt: now},
		{AgentID: "agent-2", HostID: "host-2", Version: "dev", Idle: true, LastSeenAt: now},
	} {
		if err := repo.UpsertAgent(context.Background(), agent); err != nil {
			t.Fatalf("UpsertAgent(%q) returned error: %v", agent.AgentID, err)
		}
	}

	err = repo.CreateApply(context.Background(), cpdomain.Apply{
		ApplyID:      "apply-1",
		Name:         "nginx",
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  "tester",
		RawManifest:  "raw",
		ManifestJSON: "{}",
		CreatedAt:    now,
	}, []cpdomain.WorkItem{{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "host-1",
		State:        cpdomain.WorkStatePending,
		ManifestJSON: "{}",
		UpdatedAt:    now,
	}}, nil)
	if err != nil {
		t.Fatalf("CreateApply returned error: %v", err)
	}

	got, err := repo.ClaimNextWork(context.Background(), "agent-2", time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimNextWork(agent-2) returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("agent-2 claimed %+v, want nil because work targets host-1", got)
	}

	got, err = repo.ClaimNextWork(context.Background(), "agent-1", time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimNextWork(agent-1) returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected agent-1 to claim host-1 work")
	}
	if got.HostID != "host-1" {
		t.Fatalf("claimed work host = %q, want host-1", got.HostID)
	}
}

func TestReconcileStalledMarksPendingWorkWithoutHealthyAgent(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file:reconcile-stalled?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	now := time.Unix(1700000000, 0).UTC()
	staleSeen := now.Add(-10 * time.Minute)
	if err := repo.UpsertAgent(context.Background(), cpdomain.Agent{
		AgentID:    "agent-1",
		HostID:     "host-1",
		Version:    "dev",
		Labels:     map[string]string{"role": "web"},
		Idle:       true,
		LastSeenAt: staleSeen,
	}); err != nil {
		t.Fatalf("UpsertAgent returned error: %v", err)
	}

	err = repo.CreateApply(context.Background(), cpdomain.Apply{
		ApplyID:      "apply-1",
		Name:         "nginx",
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  "tester",
		RawManifest:  "raw",
		ManifestJSON: "{}",
		CreatedAt:    now,
	}, []cpdomain.WorkItem{{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "host-1",
		State:        cpdomain.WorkStatePending,
		ManifestJSON: "{}",
		UpdatedAt:    staleSeen,
	}}, nil)
	if err != nil {
		t.Fatalf("CreateApply returned error: %v", err)
	}

	if err := repo.ReconcileStalled(context.Background(), now, 2*time.Minute); err != nil {
		t.Fatalf("ReconcileStalled returned error: %v", err)
	}

	apply, workItems, err := repo.GetApply(context.Background(), "apply-1")
	if err != nil {
		t.Fatalf("GetApply returned error: %v", err)
	}
	if apply.Status != cpdomain.ApplyStatusStalled {
		t.Fatalf("apply.Status = %q, want %q", apply.Status, cpdomain.ApplyStatusStalled)
	}
	if len(workItems) != 1 || workItems[0].State != cpdomain.WorkStateStalled {
		t.Fatalf("workItems = %+v, want one stalled work item", workItems)
	}
	events, err := repo.ListEvents(context.Background(), "apply-1", 0)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].Phase != "stalled" {
		t.Fatalf("events = %+v, want trailing stalled event", events)
	}
}

func TestReconcileStalledKeepsFreshPendingWorkPending(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file:reconcile-fresh-pending?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	now := time.Unix(1700000000, 0).UTC()
	if err := repo.CreateApply(context.Background(), cpdomain.Apply{
		ApplyID:      "apply-1",
		Name:         "nginx",
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  "tester",
		RawManifest:  "raw",
		ManifestJSON: "{}",
		CreatedAt:    now,
	}, []cpdomain.WorkItem{{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "host-1",
		State:        cpdomain.WorkStatePending,
		ManifestJSON: "{}",
		UpdatedAt:    now,
	}}, nil); err != nil {
		t.Fatalf("CreateApply returned error: %v", err)
	}

	if err := repo.ReconcileStalled(context.Background(), now, 2*time.Minute); err != nil {
		t.Fatalf("ReconcileStalled returned error: %v", err)
	}

	apply, workItems, err := repo.GetApply(context.Background(), "apply-1")
	if err != nil {
		t.Fatalf("GetApply returned error: %v", err)
	}
	if apply.Status != cpdomain.ApplyStatusPending {
		t.Fatalf("apply.Status = %q, want %q", apply.Status, cpdomain.ApplyStatusPending)
	}
	if len(workItems) != 1 || workItems[0].State != cpdomain.WorkStatePending {
		t.Fatalf("workItems = %+v, want one pending work item", workItems)
	}
}

func TestClaimNextWorkReturnsNilForBusyAgent(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file:claim-busy-agent?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	now := time.Unix(1700000000, 0).UTC()
	if err := repo.UpsertAgent(context.Background(), cpdomain.Agent{
		AgentID:           "agent-1",
		HostID:            "host-1",
		Version:           "dev",
		Idle:              false,
		CurrentWorkItemID: "existing-work",
		LastSeenAt:        now,
	}); err != nil {
		t.Fatalf("UpsertAgent returned error: %v", err)
	}
	if err := repo.CreateApply(context.Background(), cpdomain.Apply{
		ApplyID:      "apply-1",
		Name:         "nginx",
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  "tester",
		RawManifest:  "raw",
		ManifestJSON: "{}",
		CreatedAt:    now,
	}, []cpdomain.WorkItem{{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "host-1",
		State:        cpdomain.WorkStatePending,
		ManifestJSON: "{}",
		UpdatedAt:    now,
	}}, nil); err != nil {
		t.Fatalf("CreateApply returned error: %v", err)
	}

	got, err := repo.ClaimNextWork(context.Background(), "agent-1", time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimNextWork returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("ClaimNextWork returned %+v, want nil for busy agent", got)
	}
}

func TestUpdateWorkNormalizesEventMetadataFromStoredWorkItem(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file:update-work-normalizes-events?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteRepository returned error: %v", err)
	}
	defer repo.Close()

	now := time.Unix(1700000000, 0).UTC()
	if err := repo.UpsertAgent(context.Background(), cpdomain.Agent{
		AgentID:    "agent-1",
		HostID:     "host-1",
		Version:    "dev",
		Idle:       true,
		LastSeenAt: now,
	}); err != nil {
		t.Fatalf("UpsertAgent returned error: %v", err)
	}
	if err := repo.CreateApply(context.Background(), cpdomain.Apply{
		ApplyID:      "apply-1",
		Name:         "nginx",
		Status:       cpdomain.ApplyStatusPending,
		SubmittedBy:  "tester",
		RawManifest:  "raw",
		ManifestJSON: "{}",
		CreatedAt:    now,
	}, []cpdomain.WorkItem{{
		WorkItemID:   "work-1",
		ApplyID:      "apply-1",
		HostID:       "host-1",
		State:        cpdomain.WorkStatePending,
		ManifestJSON: "{}",
		UpdatedAt:    now,
	}}, nil); err != nil {
		t.Fatalf("CreateApply returned error: %v", err)
	}

	work, err := repo.ClaimNextWork(context.Background(), "agent-1", time.Minute, now)
	if err != nil {
		t.Fatalf("ClaimNextWork returned error: %v", err)
	}

	err = repo.UpdateWork(context.Background(), "agent-1", work.WorkItemID, work.LeaseToken, cpdomain.WorkStateRunning, "running", []cpdomain.ApplyEvent{{
		ID:         "evt-1",
		ApplyID:    "wrong-apply",
		HostID:     "wrong-host",
		WorkItemID: "wrong-work",
		Level:      "info",
		Phase:      "running",
		Message:    "hello",
		CreatedAt:  now,
	}}, now)
	if err != nil {
		t.Fatalf("UpdateWork returned error: %v", err)
	}

	events, err := repo.ListEvents(context.Background(), "apply-1", 0)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	got := events[len(events)-1]
	if got.ApplyID != "apply-1" || got.HostID != "host-1" || got.WorkItemID != "work-1" {
		t.Fatalf("event metadata = %+v, want canonical stored work-item metadata", got)
	}
}
