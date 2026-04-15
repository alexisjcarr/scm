package infra

import (
	"context"
	"testing"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
)

func TestClaimNextWorkOnlyAllowsSingleWinner(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository("file::memory:?cache=shared")
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
