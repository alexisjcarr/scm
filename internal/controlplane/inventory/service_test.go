package inventory

import (
	"testing"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
)

func TestResolveTargetHostsDeduplicatesExplicitAndSelectorMatches(t *testing.T) {
	t.Parallel()

	hosts := ResolveTargetHosts(manifestdomain.TargetSpec{
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
