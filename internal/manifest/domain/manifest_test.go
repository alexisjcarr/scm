package domain

import "testing"

func TestManifestValidateAndToAPI(t *testing.T) {
	t.Parallel()

	enabled := true
	compiled, err := (Manifest{
		APIVersion: "scm/v1",
		Kind:       "Manifest",
		Name:       "demo",
		Labels:     map[string]string{"team": "platform"},
		Target: TargetSpec{
			Hosts:          []string{"web-1"},
			SelectorLabels: map[string]string{"role": "web"},
		},
		Resources: []Resource{
			PackageResource{ID: "pkg", Name: "nginx", State: PackageStateInstalled},
			FileResource{ID: "cfg", Path: "/etc/nginx/nginx.conf", Content: "hello", State: FileStatePresent, Requires: []string{"pkg"}, Notifies: []string{"svc"}},
			ServiceResource{ID: "svc", Name: "nginx", State: ServiceStateRunning, Enabled: &enabled, Requires: []string{"cfg"}},
		},
	}).Validate()
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if got := len(compiled.OrderedResources); got != 3 {
		t.Fatalf("expected 3 ordered resources, got %d", got)
	}
	if compiled.OrderedResources[0].GetID() != "pkg" || compiled.OrderedResources[2].GetID() != "svc" {
		t.Fatalf("unexpected ordered resources: %#v", compiled.OrderedResources)
	}

	apiManifest := compiled.ToAPI()
	if apiManifest.Name != "demo" {
		t.Fatalf("expected api manifest name demo, got %q", apiManifest.Name)
	}
	if apiManifest.Target == nil || apiManifest.Target.Selector == nil {
		t.Fatal("expected API manifest target selector to be populated")
	}
	if got := apiManifest.Target.Selector.MatchLabels["role"]; got != "web" {
		t.Fatalf("expected selector label role=web, got %q", got)
	}
}

func TestManifestValidateRejectsInvalidNotifyTarget(t *testing.T) {
	t.Parallel()

	_, err := (Manifest{
		APIVersion: "scm/v1",
		Kind:       "Manifest",
		Name:       "bad",
		Target:     TargetSpec{Hosts: []string{"web-1"}},
		Resources: []Resource{
			PackageResource{ID: "pkg", Name: "nginx", State: PackageStateInstalled},
			FileResource{ID: "cfg", Path: "/tmp/demo", Content: "hello", State: FileStatePresent, Notifies: []string{"pkg"}},
		},
	}).Validate()
	if err == nil {
		t.Fatal("expected invalid notify target error")
	}
}

func TestValidateResourceRejectsInvalidState(t *testing.T) {
	t.Parallel()

	err := validateResource(ServiceResource{ID: "svc", Name: "nginx", State: "restarted"})
	if err == nil {
		t.Fatal("expected invalid service state error")
	}
}
