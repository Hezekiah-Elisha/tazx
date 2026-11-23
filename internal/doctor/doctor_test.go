package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/internal/docker"
	"github.com/Hezekiah-Elisha/tazx/internal/logs"
	"github.com/Hezekiah-Elisha/tazx/internal/network"
	"github.com/Hezekiah-Elisha/tazx/internal/services"
	"github.com/Hezekiah-Elisha/tazx/internal/system"
)

func TestRunDiagnosticsDetectsLogIssues(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "doctor_test.log")

	logContent := `192.168.1.10 - - [02/Aug/2026:14:00:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:01:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:02:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
45.33.32.156 - - [02/Aug/2026:14:03:00 +0300] "GET /wp-admin HTTP/1.1" 404 162 "-" "BotAgent"
45.33.32.156 - - [02/Aug/2026:14:04:00 +0300] "GET /.env HTTP/1.1" 404 162 "-" "BotAgent"`

	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	cfg := config.Config{
		LogPath:         logPath,
		RefreshRate:     2,
		CpuThreshold:    80.0,
		MemoryThreshold: 85.0,
		DiskThreshold:   85.0,
	}

	report, err := RunDiagnostics(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunDiagnostics failed: %v", err)
	}

	foundBotScan := false
	foundFailedLogins := false
	for _, issue := range report.Issues {
		if issue.Impact == "" || issue.Recommendation == "" {
			t.Errorf("issue missing impact or recommendation: %+v", issue)
		}
		if strings.Contains(issue.Title, "bot/scan") {
			foundBotScan = true
		}
		if strings.Contains(issue.Title, "failed authentication") {
			foundFailedLogins = true
		}
	}

	if !foundBotScan {
		t.Error("expected bot scan issue to be detected")
	}
	if !foundFailedLogins {
		t.Error("expected failed logins issue to be detected")
	}
}

func TestRunDiagnosticsWithMockProviders(t *testing.T) {
	tempDir := t.TempDir()
	insecureCfg := filepath.Join(tempDir, ".tazx.yaml")
	if err := os.WriteFile(insecureCfg, []byte("refresh_rate: 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := Providers{
		SystemStatus: func(context.Context) (*system.SystemStatus, error) {
			return &system.SystemStatus{
				CPUPercent:     95.0,
				MemoryPercent:  91.0,
				SwapAvailable:  true,
				SwapTotalBytes: 1024 * 1024 * 1024,
				SwapUsedGB:     0.9,
				SwapTotalGB:    1.0,
				SwapPercent:    90.0,
			}, nil
		},
		TopProcesses: func(_ context.Context, _ int, sortByMem bool) ([]system.ProcessInfo, error) {
			return []system.ProcessInfo{{PID: 42, Name: "worker", CPUPercent: 88.0, MemPercent: 60.0}}, nil
		},
		DiskPartitions: func(context.Context, bool, float64) ([]system.DiskPartition, []string, error) {
			return []system.DiskPartition{
				{Filesystem: "/dev/sda1", Mountpoint: "/", UsedPercent: 93.0, TotalBytes: 100, AvailableBytes: 7},
			}, nil, nil
		},
		AnalyzeLogs: func(context.Context, string) (*logs.LogSummary, error) {
			return nil, os.ErrPermission
		},
		ListServices: func(context.Context, bool) ([]services.ServiceInfo, error) {
			return []services.ServiceInfo{{Name: "nginx.service", ActiveState: "failed"}}, nil
		},
		DockerConfigured: func() bool { return true },
		ListContainers: func(context.Context, bool) ([]docker.ContainerInfo, error) {
			return []docker.ContainerInfo{
				{ID: "abc123", Names: "api", State: "exited", Status: "Exited (137) 2 minutes ago"},
				{ID: "def456", Names: "job", State: "exited", Status: "Exited (0) 2 minutes ago"},
			}, nil
		},
		ReadDNS: func() (*network.DNSConfig, error) {
			return &network.DNSConfig{Nameservers: nil}, nil
		},
		StatFile: os.Stat,
	}

	cfg := config.DefaultConfig()
	report, err := RunDiagnosticsWithProviders(context.Background(), cfg, insecureCfg, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	categories := make(map[string]bool)
	for _, iss := range report.Issues {
		categories[iss.Category] = true
	}

	for _, want := range []string{"cpu", "memory", "storage", "services", "docker", "logs", "config", "network"} {
		if !categories[want] {
			t.Errorf("expected diagnostic issue in category %q", want)
		}
	}

	// Verify Critical issues sort before Warning issues.
	seenWarning := false
	for _, iss := range report.Issues {
		if iss.Severity == SeverityWarning {
			seenWarning = true
		}
		if iss.Severity == SeverityCritical && seenWarning {
			t.Errorf("critical issue %q appeared after a warning issue", iss.Title)
		}
	}
}

func TestRunDiagnosticsIgnoresSystemdUnavailable(t *testing.T) {
	p := Providers{
		SystemStatus: func(context.Context) (*system.SystemStatus, error) {
			return &system.SystemStatus{CPUPercent: 10, MemoryPercent: 20}, nil
		},
		DiskPartitions: func(context.Context, bool, float64) ([]system.DiskPartition, []string, error) {
			return []system.DiskPartition{{Mountpoint: "/", UsedPercent: 30}}, nil, nil
		},
		AnalyzeLogs: func(context.Context, string) (*logs.LogSummary, error) {
			return &logs.LogSummary{TotalRequests: 1}, nil
		},
		ListServices: func(context.Context, bool) ([]services.ServiceInfo, error) {
			return nil, services.ErrSystemdUnavailable
		},
		DockerConfigured: func() bool { return false },
		ReadDNS: func() (*network.DNSConfig, error) {
			return &network.DNSConfig{Nameservers: []string{"1.1.1.1"}}, nil
		},
	}

	report, err := RunDiagnosticsWithProviders(context.Background(), config.DefaultConfig(), "", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Errorf("expected 0 issues when systemd unavailable and metrics healthy, got %+v", report.Issues)
	}

	// Check non-ErrSystemdUnavailable error still surfaces info issue.
	p.ListServices = func(context.Context, bool) ([]services.ServiceInfo, error) {
		return nil, errors.New("dbus timeout")
	}
	report, err = RunDiagnosticsWithProviders(context.Background(), config.DefaultConfig(), "", p)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Category != "services" {
		t.Errorf("expected 1 services info issue, got %+v", report.Issues)
	}
}
