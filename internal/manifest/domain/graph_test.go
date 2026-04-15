package domain

import "testing"

func TestTopoSortOrdersDependenciesDeterministically(t *testing.T) {
	t.Parallel()

	ordered, err := topoSort([]Resource{
		PackageResource{ID: "svc", Name: "nginx", State: PackageStateInstalled, Requires: []string{"cfg"}},
		FileResource{ID: "cfg", Path: "/etc/nginx/nginx.conf", Content: "x", State: FileStatePresent, Requires: []string{"pkg"}},
		PackageResource{ID: "pkg", Name: "nginx", State: PackageStateInstalled},
		PackageResource{ID: "alpha", Name: "curl", State: PackageStateInstalled},
	})
	if err != nil {
		t.Fatalf("topoSort returned error: %v", err)
	}

	got := []string{
		ordered[0].GetID(),
		ordered[1].GetID(),
		ordered[2].GetID(),
		ordered[3].GetID(),
	}
	want := []string{"alpha", "pkg", "cfg", "svc"}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("unexpected order got=%v want=%v", got, want)
		}
	}
}

func TestTopoSortRejectsCycles(t *testing.T) {
	t.Parallel()

	_, err := topoSort([]Resource{
		PackageResource{ID: "a", Name: "a", State: PackageStateInstalled, Requires: []string{"b"}},
		PackageResource{ID: "b", Name: "b", State: PackageStateInstalled, Requires: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
