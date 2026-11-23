//go:build linux

package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// IsSystemdAvailable reports whether systemd is running as PID 1 / active and systemctl exists.
func IsSystemdAvailable() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// ListServices lists systemd services (or only failed services when failedOnly is true).
func ListServices(ctx context.Context, failedOnly bool) ([]ServiceInfo, error) {
	if !IsSystemdAvailable() {
		return nil, ErrSystemdUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	args := []string{"list-units", "--type=service", "--no-pager", "--no-legend", "--plain"}
	if failedOnly {
		args = append(args, "--state=failed")
	} else {
		args = append(args, "--all")
	}

	out, err := exec.CommandContext(runCtx, "systemctl", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("systemctl list-units failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	units := ParseUnitList(string(out))

	// Enrich with UnitFileState (enabled/disabled/static/masked) when available.
	ufCtx, ufCancel := context.WithTimeout(ctx, 5*time.Second)
	defer ufCancel()
	if ufOut, ufErr := exec.CommandContext(ufCtx, "systemctl", "list-unit-files", "--type=service", "--no-pager", "--no-legend", "--plain").Output(); ufErr == nil {
		states := ParseUnitFiles(string(ufOut))
		for i := range units {
			if st, ok := states[units[i].Name]; ok && st != "" {
				units[i].Enabled = st
			}
		}
	}

	return units, nil
}

// GetService retrieves detailed systemd metadata for a single validated service unit.
func GetService(ctx context.Context, name string) (*ServiceInfo, error) {
	if !IsSystemdAvailable() {
		return nil, ErrSystemdUnavailable
	}
	unitName, err := ValidateServiceName(name)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(
		runCtx,
		"systemctl",
		"show",
		unitName,
		"--no-pager",
		"--property=Id,LoadState,ActiveState,SubState,UnitFileState,Description,MainPID,ActiveEnterTimestamp",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("systemctl show %s failed: %w (%s)", unitName, err, strings.TrimSpace(string(out)))
	}

	info, err := ParseServiceShow(string(out))
	if err != nil {
		return nil, err
	}
	return info, nil
}
