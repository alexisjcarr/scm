package infra

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultFileHelperPath is the installed helper used for privileged file operations.
const DefaultFileHelperPath = "/usr/local/libexec/scm-agent-fileop"

// Command describes a host-level operation the backend needs to execute.
type Command struct {
	Name       string
	Args       []string
	Privileged bool
	Stdin      []byte
}

// CommandRunner executes backend commands and centralizes privilege elevation.
type CommandRunner interface {
	Run(context.Context, Command) ([]byte, error)
}

// ExecRunner is the default CommandRunner backed by exec.CommandContext.
type ExecRunner struct {
	UseSudo     bool
	execCommand func(context.Context, string, ...string) *exec.Cmd
}

// NewExecRunner constructs the default command runner.
func NewExecRunner(useSudo bool) *ExecRunner {
	return &ExecRunner{
		UseSudo:     useSudo,
		execCommand: exec.CommandContext,
	}
}

// Run executes the provided command, elevating via sudo when requested.
func (r *ExecRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	name, args := r.expand(command)
	cmd := r.execCommand(ctx, name, args...)
	if len(command.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(command.Stdin)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed: %w: %s", r.errorPrefix(command), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (r *ExecRunner) expand(command Command) (string, []string) {
	args := append([]string(nil), command.Args...)
	if command.Privileged && r.UseSudo {
		return "sudo", append([]string{"-n", command.Name}, args...)
	}
	return command.Name, args
}

func (r *ExecRunner) errorPrefix(command Command) string {
	name, args := r.expand(command)
	rendered := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	if command.Privileged && r.UseSudo {
		return rendered + " (passwordless sudo required for scmctld-agent)"
	}
	return rendered
}
