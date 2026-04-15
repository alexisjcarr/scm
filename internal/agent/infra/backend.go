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
type LinuxBackend struct{}

func (LinuxBackend) EnsurePackage(ctx context.Context, resource manifestdomain.PackageResource) (bool, string, error) {
	installed := exec.CommandContext(ctx, "dpkg", "-s", resource.Name).Run() == nil
	switch resource.State {
	case manifestdomain.PackageStateInstalled:
		if installed {
			return false, "package already installed", nil
		}
		if err := run(ctx, "apt-get", "update"); err != nil {
			return false, "", err
		}
		if err := run(ctx, "apt-get", "install", "-y", resource.Name); err != nil {
			return false, "", err
		}
		return true, "package installed", nil
	case manifestdomain.PackageStateAbsent:
		if !installed {
			return false, "package already absent", nil
		}
		if err := run(ctx, "apt-get", "remove", "-y", resource.Name); err != nil {
			return false, "", err
		}
		return true, "package removed", nil
	default:
		return false, "", fmt.Errorf("unsupported package state %q", resource.State)
	}
}

func (LinuxBackend) EnsureFile(_ context.Context, resource manifestdomain.FileResource) (bool, string, error) {
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
		if resource.Owner != "" || resource.Group != "" {
			if err := run(context.Background(), "chown", ownerGroup(resource.Owner, resource.Group), resource.Path); err != nil {
				return false, "", err
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

func (LinuxBackend) EnsureService(ctx context.Context, resource manifestdomain.ServiceResource, notifyOnly bool) (bool, string, error) {
	if notifyOnly {
		if err := run(ctx, "systemctl", "restart", resource.Name); err != nil {
			return false, "", err
		}
		return true, "service restarted due to notify", nil
	}

	changed := false
	if resource.Enabled != nil {
		enabled := exec.CommandContext(ctx, "systemctl", "is-enabled", resource.Name).Run() == nil
		if *resource.Enabled && !enabled {
			if err := run(ctx, "systemctl", "enable", resource.Name); err != nil {
				return false, "", err
			}
			changed = true
		}
		if !*resource.Enabled && enabled {
			if err := run(ctx, "systemctl", "disable", resource.Name); err != nil {
				return false, "", err
			}
			changed = true
		}
	}

	active := exec.CommandContext(ctx, "systemctl", "is-active", resource.Name).Run() == nil
	switch resource.State {
	case manifestdomain.ServiceStateRunning:
		if !active {
			if err := run(ctx, "systemctl", "start", resource.Name); err != nil {
				return false, "", err
			}
			changed = true
		}
	case manifestdomain.ServiceStateStopped:
		if active {
			if err := run(ctx, "systemctl", "stop", resource.Name); err != nil {
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

func ownerGroup(owner, group string) string {
	if owner == "" {
		owner = "root"
	}
	if group == "" {
		group = owner
	}
	return owner + ":" + group
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func currentUIDGID(info os.FileInfo) (uint32, uint32, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return stat.Uid, stat.Gid, true
}
