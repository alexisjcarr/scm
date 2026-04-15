package app

import (
	"os"
	"strings"
	"testing"

	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
)

func TestParseAndValidateSuccess(t *testing.T) {
	t.Parallel()

	data := []byte(`
apiVersion: scm/v1
kind: Manifest
metadata:
  name: nginx
target:
  hosts:
    - web-1
  selector:
    matchLabels:
      role: web
resources:
  - id: pkg
    type: package
    name: nginx
    state: installed
  - id: cfg
    type: file
    path: /etc/nginx/nginx.conf
    content: hello
    mode: "0644"
    state: present
    requires: [pkg]
    notifies: [svc]
  - id: svc
    type: service
    name: nginx
    state: running
    enabled: true
    requires: [cfg]
`)

	compiled, err := Service{}.ParseAndValidate(data)
	if err != nil {
		t.Fatalf("ParseAndValidate returned error: %v", err)
	}

	if compiled.Name != "nginx" {
		t.Fatalf("expected manifest name nginx, got %q", compiled.Name)
	}
	if got := len(compiled.OrderedResources); got != 3 {
		t.Fatalf("expected 3 ordered resources, got %d", got)
	}
	if compiled.OrderedResources[0].GetID() != "pkg" || compiled.OrderedResources[2].GetID() != "svc" {
		t.Fatalf("unexpected resource order: %#v", compiled.OrderedResources)
	}
}

func TestParseAndValidateRejectsCycles(t *testing.T) {
	t.Parallel()

	data := []byte(`
apiVersion: scm/v1
kind: Manifest
metadata:
  name: cycle
target:
  hosts: [node-1]
resources:
  - id: a
    type: package
    name: one
    state: installed
    requires: [b]
  - id: b
    type: package
    name: two
    state: installed
    requires: [a]
`)

	_, err := Service{}.ParseAndValidate(data)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestParseAndValidateRejectsUnknownNotifyTarget(t *testing.T) {
	t.Parallel()

	data := []byte(`
apiVersion: scm/v1
kind: Manifest
metadata:
  name: bad
target:
  hosts: [node-1]
resources:
  - id: pkg
    type: package
    name: nginx
    state: installed
    notifies: [missing]
`)

	_, err := Service{}.ParseAndValidate(data)
	if err == nil || !strings.Contains(err.Error(), "unknown resource") {
		t.Fatalf("expected unknown resource error, got %v", err)
	}
}

func TestParseAndValidatePHPAppManifestExample(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../../examples/manifests/php-app-two-hosts.yaml")
	if err != nil {
		t.Fatalf("read php app manifest example: %v", err)
	}

	compiled, err := Service{}.ParseAndValidate(data)
	if err != nil {
		t.Fatalf("ParseAndValidate returned error: %v", err)
	}
	if compiled.Name != "php-app-two-hosts" {
		t.Fatalf("expected manifest name php-app-two-hosts, got %q", compiled.Name)
	}
	if got := len(compiled.Target.Hosts); got != 2 {
		t.Fatalf("expected two explicit hosts, got %d", got)
	}
	if got := len(compiled.OrderedResources); got != 6 {
		t.Fatalf("expected 6 ordered resources, got %d", got)
	}
	foundHelloWorld := false
	for _, resource := range compiled.Resources {
		fileResource, ok := resource.(manifestdomain.FileResource)
		if !ok || fileResource.ID != "app_index" {
			continue
		}
		if strings.Contains(fileResource.Content, "Hello, world!") {
			foundHelloWorld = true
		}
	}
	if !foundHelloWorld {
		t.Fatal("expected php app manifest example to serve Hello, world!")
	}
}
