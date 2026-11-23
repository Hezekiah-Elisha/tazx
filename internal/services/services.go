package services

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrSystemdUnavailable is returned when systemd cannot be queried on the host.
var ErrSystemdUnavailable = errors.New("systemd is not available on this system")

// ServiceInfo describes the state of a systemd service unit.
type ServiceInfo struct {
	Name        string `json:"name"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	Enabled     string `json:"enabled"`
	Description string `json:"description"`
	MainPID     int    `json:"main_pid,omitempty"`
	Since       string `json:"since,omitempty"`
}

var validServiceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9@_.:-]+$`)

var validUnitSuffixes = []string{
	".service", ".socket", ".timer", ".target", ".mount",
	".path", ".slice", ".scope", ".swap", ".automount", ".device",
}

func hasUnitSuffix(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range validUnitSuffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}

// ValidateServiceName checks that name is a safe systemd unit identifier and appends .service if missing.
func ValidateServiceName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("service name must not be empty")
	}
	if len(trimmed) > 256 {
		return "", errors.New("service name exceeds maximum length")
	}
	if strings.HasPrefix(trimmed, "-") {
		return "", errors.New("service name must not start with '-'")
	}
	if strings.ContainsAny(trimmed, "/\\ \t\r\n") {
		return "", errors.New("service name contains invalid characters")
	}
	if !validServiceNameRegex.MatchString(trimmed) {
		return "", errors.New("service name contains unsupported characters")
	}
	if !hasUnitSuffix(trimmed) {
		trimmed += ".service"
	}
	return trimmed, nil
}

// ParseUnitList parses the output of:
// systemctl list-units --type=service --all --no-pager --no-legend --plain
func ParseUnitList(output string) []ServiceInfo {
	lines := strings.Split(output, "\n")
	var result []ServiceInfo

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[0]
		loadState := fields[1]
		activeState := fields[2]
		subState := fields[3]
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}
		result = append(result, ServiceInfo{
			Name:        name,
			LoadState:   loadState,
			ActiveState: activeState,
			SubState:    subState,
			Enabled:     "unknown",
			Description: desc,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ParseUnitFiles parses the output of:
// systemctl list-unit-files --type=service --no-pager --no-legend --plain
func ParseUnitFiles(output string) map[string]string {
	states := make(map[string]string)
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			states[fields[0]] = fields[1]
		}
	}
	return states
}

// ParseServiceShow parses key=value output from `systemctl show`.
func ParseServiceShow(output string) (*ServiceInfo, error) {
	info := &ServiceInfo{
		Enabled: "unknown",
	}
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "Id":
			info.Name = v
		case "LoadState":
			info.LoadState = v
		case "ActiveState":
			info.ActiveState = v
		case "SubState":
			info.SubState = v
		case "UnitFileState":
			if v != "" {
				info.Enabled = v
			}
		case "Description":
			info.Description = v
		case "MainPID":
			if pid, err := strconv.Atoi(v); err == nil {
				info.MainPID = pid
			}
		case "ActiveEnterTimestamp":
			info.Since = v
		}
	}
	if info.Name == "" {
		return nil, fmt.Errorf("service metadata not found in systemctl output")
	}
	return info, nil
}
