package infra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	manifestdomain "github.com/alexisjcarr/scm/internal/manifest/domain"
)

// LinuxBackend implements resource reconciliation for Ubuntu-like hosts.
type LinuxBackend struct {
	Runner         CommandRunner
	FileHelperPath string
}

// NewLinuxBackend constructs the default Linux backend with sudo-aware command execution.
func NewLinuxBackend() LinuxBackend {
	return LinuxBackend{
		Runner:         NewExecRunner(os.Geteuid() != 0),
		FileHelperPath: DefaultFileHelperPath,
	}
}

func (b LinuxBackend) EnsurePackage(ctx context.Context, resource manifestdomain.PackageResource) (bool, string, error) {
	installed := exec.CommandContext(ctx, "dpkg", "-s", resource.Name).Run() == nil
	switch resource.State {
	case manifestdomain.PackageStateInstalled:
		if installed {
			return false, "package already installed", nil
		}
		if _, err := b.runner().Run(ctx, Command{Name: "apt-get", Args: []string{"update"}, Privileged: true}); err != nil {
			return false, "", err
		}
		if _, err := b.runner().Run(ctx, Command{Name: "apt-get", Args: []string{"install", "-y", resource.Name}, Privileged: true}); err != nil {
			return false, "", err
		}
		return true, "package installed", nil
	case manifestdomain.PackageStateAbsent:
		if !installed {
			return false, "package already absent", nil
		}
		if _, err := b.runner().Run(ctx, Command{Name: "apt-get", Args: []string{"remove", "-y", resource.Name}, Privileged: true}); err != nil {
			return false, "", err
		}
		return true, "package removed", nil
	default:
		return false, "", fmt.Errorf("unsupported package state %q", resource.State)
	}
}

func (b LinuxBackend) EnsureFile(ctx context.Context, resource manifestdomain.FileResource) (bool, string, error) {
	if b.requiresPrivilegedFileOps(resource) {
		return b.ensureFilePrivileged(ctx, resource)
	}
	return b.ensureFileDirect(resource)
}

func (b LinuxBackend) ensureFileDirect(resource manifestdomain.FileResource) (bool, string, error) {
	switch resource.State {
	case manifestdomain.FileStateAbsent:
		if _, err := os.Stat(resource.Path); errors.Is(err, os.ErrNotExist) {
			return false, "file already absent", nil
		}
		if err := os.Remove(resource.Path); err != nil {
			return false, "", fmt.Errorf("remove file %s: %w", resource.Path, err)
		}
		return true, "file removed", nil
	case manifestdomain.FileStatePresent:
		currentContent, _ := os.ReadFile(resource.Path)
		modeChanged := false
		contentChanged := !bytes.Equal(currentContent, []byte(resource.Content))
		if contentChanged {
			if err := os.MkdirAll(filepath.Dir(resource.Path), 0o755); err != nil {
				return false, "", fmt.Errorf("create parent dir for %s: %w", resource.Path, err)
			}
			if err := os.WriteFile(resource.Path, []byte(resource.Content), 0o644); err != nil {
				return false, "", fmt.Errorf("write file %s: %w", resource.Path, err)
			}
		}
		if resource.Mode != "" {
			mode, err := strconv.ParseUint(resource.Mode, 8, 32)
			if err != nil {
				return false, "", fmt.Errorf("parse file mode %q: %w", resource.Mode, err)
			}
			info, err := os.Stat(resource.Path)
			if err != nil {
				return false, "", fmt.Errorf("stat file %s: %w", resource.Path, err)
			}
			if info.Mode().Perm() != os.FileMode(mode) {
				if err := os.Chmod(resource.Path, os.FileMode(mode)); err != nil {
					return false, "", fmt.Errorf("chmod %s: %w", resource.Path, err)
				}
				modeChanged = true
			}
		}
		changed := contentChanged || modeChanged || resource.Owner != "" || resource.Group != ""
		if changed {
			return true, "file updated", nil
		}
		return false, "file already converged", nil
	default:
		return false, "", fmt.Errorf("unsupported file state %q", resource.State)
	}
}

func (b LinuxBackend) ensureFilePrivileged(ctx context.Context, resource manifestdomain.FileResource) (bool, string, error) {
	switch resource.State {
	case manifestdomain.FileStateAbsent:
		output, err := b.runner().Run(ctx, Command{
			Name:       b.fileHelperPath(),
			Args:       []string{"delete", "--path", resource.Path},
			Privileged: true,
		})
		if err != nil {
			return false, "", err
		}
		if strings.TrimSpace(string(output)) == "changed" {
			return true, "file removed", nil
		}
		return false, "file already absent", nil
	case manifestdomain.FileStatePresent:
		args := []string{"write", "--path", resource.Path}
		mode := resource.Mode
		if mode == "" {
			mode = "0644"
		}
		args = append(args, "--mode", mode)
		if resource.Owner != "" {
			args = append(args, "--owner", resource.Owner)
		}
		if resource.Group != "" {
			args = append(args, "--group", resource.Group)
		}
		output, err := b.runner().Run(ctx, Command{
			Name:       b.fileHelperPath(),
			Args:       args,
			Privileged: true,
			Stdin:      []byte(resource.Content),
		})
		if err != nil {
			return false, "", err
		}
		if strings.TrimSpace(string(output)) == "changed" {
			return true, "file updated", nil
		}
		return false, "file already converged", nil
	default:
		return false, "", fmt.Errorf("unsupported file state %q", resource.State)
	}
}

func (b LinuxBackend) EnsureService(ctx context.Context, resource manifestdomain.ServiceResource, notifyOnly bool) (bool, string, error) {
	if notifyOnly {
		if _, err := b.runner().Run(ctx, Command{Name: "systemctl", Args: []string{"restart", resource.Name}, Privileged: true}); err != nil {
			return false, "", err
		}
		return true, "service restarted due to notify", nil
	}

	changed := false
	if resource.Enabled != nil {
		enabled := b.commandSucceeds(ctx, Command{Name: "systemctl", Args: []string{"is-enabled", resource.Name}})
		if *resource.Enabled && !enabled {
			if _, err := b.runner().Run(ctx, Command{Name: "systemctl", Args: []string{"enable", resource.Name}, Privileged: true}); err != nil {
				return false, "", err
			}
			changed = true
		}
		if !*resource.Enabled && enabled {
			if _, err := b.runner().Run(ctx, Command{Name: "systemctl", Args: []string{"disable", resource.Name}, Privileged: true}); err != nil {
				return false, "", err
			}
			changed = true
		}
	}

	active := b.commandSucceeds(ctx, Command{Name: "systemctl", Args: []string{"is-active", resource.Name}})
	switch resource.State {
	case manifestdomain.ServiceStateRunning:
		if !active {
			if _, err := b.runner().Run(ctx, Command{Name: "systemctl", Args: []string{"start", resource.Name}, Privileged: true}); err != nil {
				return false, "", err
			}
			changed = true
		}
	case manifestdomain.ServiceStateStopped:
		if active {
			if _, err := b.runner().Run(ctx, Command{Name: "systemctl", Args: []string{"stop", resource.Name}, Privileged: true}); err != nil {
				return false, "", err
			}
			changed = true
		}
	default:
		return false, "", fmt.Errorf("unsupported service state %q", resource.State)
	}

	if changed {
		return true, "service updated", nil
	}
	return false, "service already converged", nil
}

func (b LinuxBackend) runner() CommandRunner {
	if b.Runner != nil {
		return b.Runner
	}
	return NewExecRunner(os.Geteuid() != 0)
}

func (b LinuxBackend) fileHelperPath() string {
	if b.FileHelperPath != "" {
		return b.FileHelperPath
	}
	return DefaultFileHelperPath
}

func (b LinuxBackend) commandSucceeds(ctx context.Context, command Command) bool {
	_, err := b.runner().Run(ctx, command)
	return err == nil
}

func (b LinuxBackend) requiresPrivilegedFileOps(resource manifestdomain.FileResource) bool {
	if resource.Owner != "" || resource.Group != "" {
		return true
	}

	target := resource.Path
	if _, err := os.Stat(resource.Path); errors.Is(err, os.ErrNotExist) {
		target = nearestExistingPath(filepath.Dir(resource.Path))
	}
	if target == "" {
		return true
	}
	info, err := os.Stat(target)
	if err != nil {
		return true
	}
	return !isWritableByCurrentUser(info)
}

func nearestExistingPath(path string) string {
	current := path
	for current != "" && current != "." && current != string(filepath.Separator) {
		if _, err := os.Stat(current); err == nil {
			return current
		}
		current = filepath.Dir(current)
	}
	if _, err := os.Stat(string(filepath.Separator)); err == nil {
		return string(filepath.Separator)
	}
	return ""
}

func isWritableByCurrentUser(info os.FileInfo) bool {
	if os.Geteuid() == 0 {
		return true
	}

	mode := info.Mode().Perm()
	uid, gid, ok := currentUIDGID(info)
	if ok {
		if int(uid) == os.Geteuid() {
			return mode&0o200 != 0
		}
		if int(gid) == os.Getegid() {
			return mode&0o020 != 0
		}
	}
	return mode&0o002 != 0
}

func currentUIDGID(info os.FileInfo) (uint32, uint32, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return stat.Uid, stat.Gid, true
}
