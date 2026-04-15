package infra

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestExecRunnerUsesSudoForPrivilegedCommands(t *testing.T) {
	runner := NewExecRunner(true)
	runner.execCommand = helperExecCommand

	output, err := runner.Run(context.Background(), Command{
		Name:       "apt-get",
		Args:       []string{"install", "-y", "nginx"},
		Privileged: true,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "sudo -n apt-get install -y nginx" {
		t.Fatalf("unexpected privileged invocation %q", got)
	}
}

func TestExecRunnerReturnsHelpfulErrorWhenSudoPrivilegesAreMissing(t *testing.T) {
	t.Setenv("HELPER_EXIT_CODE", "1")
	t.Setenv("HELPER_STDERR", "sudo: a password is required")

	runner := NewExecRunner(true)
	runner.execCommand = helperExecCommand

	_, err := runner.Run(context.Background(), Command{
		Name:       "systemctl",
		Args:       []string{"restart", "nginx"},
		Privileged: true,
	})
	if err == nil {
		t.Fatal("expected privileged runner error")
	}
	if !strings.Contains(err.Error(), "passwordless sudo required for scmctld-agent") {
		t.Fatalf("expected passwordless sudo guidance in %q", err)
	}
	if !strings.Contains(err.Error(), "sudo -n systemctl restart nginx") {
		t.Fatalf("expected rendered sudo command in %q", err)
	}
}

func helperExecCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"-test.run=TestExecRunnerHelperProcess", "--", name}, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestExecRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	separator := 0
	for idx, arg := range os.Args {
		if arg == "--" {
			separator = idx
			break
		}
	}
	args := os.Args[separator+1:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, "missing helper arguments")
		os.Exit(2)
	}

	if stderr := os.Getenv("HELPER_STDERR"); stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	if stdout := os.Getenv("HELPER_STDOUT"); stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
	} else {
		fmt.Fprint(os.Stdout, strings.Join(args, " "))
	}

	exitCode := 0
	if raw := os.Getenv("HELPER_EXIT_CODE"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid HELPER_EXIT_CODE %q", raw)
			os.Exit(2)
		}
		exitCode = parsed
	}
	os.Exit(exitCode)
}
