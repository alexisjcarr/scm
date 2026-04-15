package infra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
)

type fakeCommandRunner struct {
	calls   []string
	outputs map[string][]byte
	errs    map[string]error
}

func (r *fakeCommandRunner) Run(_ context.Context, command Command) ([]byte, error) {
	key := strings.TrimSpace(strings.Join(append([]string{command.Name}, command.Args...), " "))
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if output, ok := r.outputs[key]; ok {
		return output, nil
	}
	return nil, nil
}

func TestEnsurePackageInstallsWhenMissing(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{}
	backend := LinuxBackend{
		Runner:           runner,
		PackageInstalled: func(context.Context, string) bool { return false },
	}

	changed, message, err := backend.EnsurePackage(context.Background(), manifestdomain.PackageResource{
		ID:    "pkg",
		Name:  "nginx",
		State: manifestdomain.PackageStateInstalled,
	})
	if err != nil {
		t.Fatalf("EnsurePackage returned error: %v", err)
	}
	if !changed || message != "package installed" {
		t.Fatalf("unexpected install result changed=%v message=%q", changed, message)
	}
	if len(runner.calls) != 2 || runner.calls[0] != "apt-get update" || runner.calls[1] != "apt-get install -y nginx" {
		t.Fatalf("unexpected package commands: %#v", runner.calls)
	}
}

func TestEnsureFileDirectIsIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "managed.txt")
	backend := LinuxBackend{}
	resource := manifestdomain.FileResource{
		ID:      "cfg",
		Path:    path,
		Content: "hello",
		Mode:    "0640",
		State:   manifestdomain.FileStatePresent,
	}

	changed, message, err := backend.EnsureFile(context.Background(), resource)
	if err != nil {
		t.Fatalf("EnsureFile returned error: %v", err)
	}
	if !changed || message != "file updated" {
		t.Fatalf("unexpected first apply result changed=%v message=%q", changed, message)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read managed file: %v", err)
	}
	if got := string(data); got != "hello" {
		t.Fatalf("expected file content hello, got %q", got)
	}

	changed, message, err = backend.EnsureFile(context.Background(), resource)
	if err != nil {
		t.Fatalf("EnsureFile returned error on second run: %v", err)
	}
	if changed || message != "file already converged" {
		t.Fatalf("expected idempotent direct file apply, got changed=%v message=%q", changed, message)
	}
}

func TestEnsureFilePrivilegedUsesHelper(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{
		outputs: map[string][]byte{
			"/helper write --path /etc/demo.conf --mode 0644 --owner root --group root": []byte("changed\n"),
		},
	}
	backend := LinuxBackend{
		Runner:         runner,
		FileHelperPath: "/helper",
	}

	changed, message, err := backend.EnsureFile(context.Background(), manifestdomain.FileResource{
		ID:      "cfg",
		Path:    "/etc/demo.conf",
		Content: "managed",
		Mode:    "0644",
		Owner:   "root",
		Group:   "root",
		State:   manifestdomain.FileStatePresent,
	})
	if err != nil {
		t.Fatalf("EnsureFile returned error: %v", err)
	}
	if !changed || message != "file updated" {
		t.Fatalf("unexpected privileged file result changed=%v message=%q", changed, message)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "/helper write --path /etc/demo.conf --mode 0644 --owner root --group root" {
		t.Fatalf("unexpected privileged file helper calls: %#v", runner.calls)
	}
}

func TestEnsureServiceStartsAndEnablesWhenNeeded(t *testing.T) {
	t.Parallel()

	enable := true
	runner := &fakeCommandRunner{
		errs: map[string]error{
			"systemctl is-enabled nginx": errors.New("disabled"),
			"systemctl is-active nginx":  errors.New("inactive"),
		},
	}
	backend := LinuxBackend{Runner: runner}

	changed, message, err := backend.EnsureService(context.Background(), manifestdomain.ServiceResource{
		ID:      "svc",
		Name:    "nginx",
		State:   manifestdomain.ServiceStateRunning,
		Enabled: &enable,
	}, false)
	if err != nil {
		t.Fatalf("EnsureService returned error: %v", err)
	}
	if !changed || message != "service updated" {
		t.Fatalf("unexpected service result changed=%v message=%q", changed, message)
	}
	if want := []string{
		"systemctl is-enabled nginx",
		"systemctl enable nginx",
		"systemctl is-active nginx",
		"systemctl start nginx",
	}; strings.Join(runner.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected service commands: %#v", runner.calls)
	}
}

func TestRequiresPrivilegedFileOps(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "local.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("seed local file: %v", err)
	}

	backend := LinuxBackend{}
	if backend.requiresPrivilegedFileOps(manifestdomain.FileResource{Path: path}) {
		t.Fatal("expected writable local path to avoid privileged helper")
	}
	if !backend.requiresPrivilegedFileOps(manifestdomain.FileResource{Path: path, Owner: "root"}) {
		t.Fatal("expected owner/group changes to require privileged helper")
	}
}
