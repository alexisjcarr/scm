package packaging

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdUnitsUseDedicatedServiceUsers(t *testing.T) {
	t.Parallel()

	controlPlaneUnit := readFile(t, filepath.Join("systemd", "scmctld.service"))
	if !strings.Contains(controlPlaneUnit, "User=scmctld") || !strings.Contains(controlPlaneUnit, "Group=scmctld") {
		t.Fatalf("expected dedicated scmctld service user in unit:\n%s", controlPlaneUnit)
	}

	agentUnit := readFile(t, filepath.Join("systemd", "scmctld-agent.service"))
	if !strings.Contains(agentUnit, "User=scmctld-agent") || !strings.Contains(agentUnit, "Group=scmctld-agent") {
		t.Fatalf("expected dedicated scmctld-agent service user in unit:\n%s", agentUnit)
	}
	if strings.Contains(agentUnit, "scmctld.service") {
		t.Fatalf("agent unit should not depend on a local scmctld service:\n%s", agentUnit)
	}
	if !strings.Contains(agentUnit, "network-online.target") {
		t.Fatalf("agent unit should depend on network-online.target:\n%s", agentUnit)
	}
}

func TestInstallScriptsParseAndShipPrivilegeAssets(t *testing.T) {
	t.Parallel()

	scriptPaths := []string{
		filepath.Join("install", "install.sh"),
		filepath.Join("install", "uninstall.sh"),
		filepath.Join("install", "scm-agent-fileop"),
	}
	args := append([]string{"-n"}, scriptPaths...)
	cmd := exec.Command("bash", args...)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n failed: %v\n%s", err, output)
	}

	sudoers := readFile(t, filepath.Join("sudoers", "scmctld-agent"))
	for _, expected := range []string{
		"/usr/bin/apt-get",
		"/usr/bin/systemctl",
		"/usr/local/libexec/scm-agent-fileop",
	} {
		if !strings.Contains(sudoers, expected) {
			t.Fatalf("expected sudoers drop-in to allow %s:\n%s", expected, sudoers)
		}
	}
}

func readFile(t *testing.T, relative string) string {
	t.Helper()

	data, err := os.ReadFile(relative)
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(data)
}
