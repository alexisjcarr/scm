package app

import (
	"strings"
	"testing"
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
