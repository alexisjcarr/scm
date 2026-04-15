package app

import (
	"context"
	"testing"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeRepo struct {
	agents []cpdomain.Agent
	apply  cpdomain.Apply
	work   []cpdomain.WorkItem
}

func (f *fakeRepo) UpsertAgent(context.Context, cpdomain.Agent) error { return nil }
func (f *fakeRepo) UpdateHeartbeat(context.Context, string, bool, string, time.Time) error {
	return nil
}
func (f *fakeRepo) ListAgents(context.Context) ([]cpdomain.Agent, error)  { return f.agents, nil }
func (f *fakeRepo) ListApplies(context.Context) ([]cpdomain.Apply, error) { return nil, nil }
func (f *fakeRepo) CreateApply(_ context.Context, apply cpdomain.Apply, work []cpdomain.WorkItem, _ []cpdomain.ApplyEvent) error {
	f.apply = apply
	f.work = work
	return nil
}
func (f *fakeRepo) GetApply(context.Context, string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	return cpdomain.Apply{}, nil, nil
}
func (f *fakeRepo) ListEvents(context.Context, string, int64) ([]cpdomain.ApplyEvent, error) {
	return nil, nil
}
func (f *fakeRepo) ClaimNextWork(context.Context, string, time.Duration, time.Time) (*cpdomain.WorkItem, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateWork(context.Context, string, string, string, string, string, []cpdomain.ApplyEvent, time.Time) error {
	return nil
}

func TestResolveTargetHostsDeduplicatesExplicitAndSelectorMatches(t *testing.T) {
	t.Parallel()

	hosts := resolveTargetHosts(manifestdomain.TargetSpec{
		Hosts:          []string{"web-2", "web-1"},
		SelectorLabels: map[string]string{"role": "web"},
	}, []cpdomain.Agent{
		{HostID: "web-1", Labels: map[string]string{"role": "web"}},
		{HostID: "web-3", Labels: map[string]string{"role": "web"}},
		{HostID: "db-1", Labels: map[string]string{"role": "db"}},
	})

	expected := []string{"web-1", "web-2", "web-3"}
	if len(hosts) != len(expected) {
		t.Fatalf("expected %d hosts, got %d", len(expected), len(hosts))
	}
	for i := range expected {
		if hosts[i] != expected[i] {
			t.Fatalf("expected host %q at index %d, got %q", expected[i], i, hosts[i])
		}
	}
}

func TestSubmitApplyCreatesOneWorkItemPerResolvedHost(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		agents: []cpdomain.Agent{
			{HostID: "web-1", Labels: map[string]string{"role": "web"}},
			{HostID: "web-2", Labels: map[string]string{"role": "web"}},
		},
	}

	svc := NewService(repo, fakeClock{now: time.Unix(1700000000, 0).UTC()}, time.Minute)
	compiled := manifestdomain.CompiledManifest{
		Manifest: manifestdomain.Manifest{
			APIVersion: "scm/v1",
			Kind:       "Manifest",
			Name:       "nginx",
			Target: manifestdomain.TargetSpec{
				Hosts:          []string{"web-1"},
				SelectorLabels: map[string]string{"role": "web"},
			},
			Resources: []manifestdomain.Resource{
				manifestdomain.PackageResource{ID: "pkg", Name: "nginx", State: manifestdomain.PackageStateInstalled},
			},
		},
	}

	_, work, err := svc.SubmitApply(context.Background(), compiled, "raw", "alexis")
	if err != nil {
		t.Fatalf("SubmitApply returned error: %v", err)
	}

	if len(work) != 2 {
		t.Fatalf("expected 2 work items, got %d", len(work))
	}
}
