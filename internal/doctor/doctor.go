package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/internal/docker"
	"github.com/Hezekiah-Elisha/tazx/internal/logs"
	"github.com/Hezekiah-Elisha/tazx/internal/network"
	"github.com/Hezekiah-Elisha/tazx/internal/services"
	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

type Issue struct {
	Severity       Severity `json:"severity"`
	Category       string   `json:"category"`
	Icon           string   `json:"icon"`
	Title          string   `json:"title"`
	Impact         string   `json:"impact"`
	Explanation    string   `json:"explanation"`
	Recommendation string   `json:"recommendation"`
}

type DiagnosticReport struct {
	Issues        []Issue              `json:"issues"`
	SystemSummary *system.SystemStatus `json:"system_summary,omitempty"`
	LogSummary    *logs.LogSummary     `json:"log_summary,omitempty"`
}

// Providers abstracts external system probes so diagnostic rules can be tested deterministically.
type Providers struct {
	SystemStatus     func(context.Context) (*system.SystemStatus, error)
	TopProcesses     func(context.Context, int, bool) ([]system.ProcessInfo, error)
	DiskPartitions   func(context.Context, bool, float64) ([]system.DiskPartition, []string, error)
	AnalyzeLogs      func(context.Context, string) (*logs.LogSummary, error)
	ListServices     func(context.Context, bool) ([]services.ServiceInfo, error)
	ListContainers   func(context.Context, bool) ([]docker.ContainerInfo, error)
	DockerConfigured func() bool
	ReadDNS          func() (*network.DNSConfig, error)
	StatFile         func(string) (os.FileInfo, error)
}

// DefaultProviders returns production system diagnostic providers.
func DefaultProviders() Providers {
	return Providers{
		SystemStatus:   system.GetSystemStatus,
		TopProcesses:   system.GetTopProcesses,
		DiskPartitions: system.GetDiskPartitions,
		AnalyzeLogs:    logs.AnalyzeLogs,
		ListServices:   services.ListServices,
		ListContainers: docker.GetContainers,
		DockerConfigured: func() bool {
			if os.Getenv("DOCKER_HOST") != "" {
				return true
			}
			if _, err := os.Stat("/var/run/docker.sock"); err == nil {
				return true
			}
			_, err := exec.LookPath("docker")
			return err == nil
		},
		ReadDNS:  network.ReadSystemDNS,
		StatFile: os.Stat,
	}
}

var nonZeroExitRegex = regexp.MustCompile(`(?i)exited\s+\(([1-9][0-9]*)\)`)

func isUnexpectedContainerFailure(c docker.ContainerInfo) bool {
	state := strings.ToLower(strings.TrimSpace(c.State))
	if state == "dead" || state == "restarting" {
		return true
	}
	if state == "exited" && nonZeroExitRegex.MatchString(c.Status) {
		return true
	}
	return false
}

// RunDiagnostics runs read-only server diagnostics using default system providers.
func RunDiagnostics(ctx context.Context, cfg config.Config) (*DiagnosticReport, error) {
	return RunDiagnosticsWithProviders(ctx, cfg, "", DefaultProviders())
}

// RunDiagnosticsWithProviders runs read-only server diagnostics using injected providers.
func RunDiagnosticsWithProviders(ctx context.Context, cfg config.Config, cfgFilePath string, p Providers) (*DiagnosticReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	report := &DiagnosticReport{}

	// 1. System Metrics (CPU, RAM, Swap)
	if p.SystemStatus != nil {
		sysStatus, err := p.SystemStatus(ctx)
		if err != nil {
			report.Issues = append(report.Issues, Issue{
				Severity:       SeverityWarning,
				Category:       "system",
				Icon:           "⚠️",
				Title:          "Unable to read full system metrics",
				Impact:         "CPU, memory, or disk saturation could go undetected.",
				Explanation:    err.Error(),
				Recommendation: "Verify host /proc and filesystem permissions.",
			})
		} else if sysStatus != nil {
			report.SystemSummary = sysStatus

			// CPU Check
			if sysStatus.CPUPercent >= cfg.CpuThreshold {
				sev := SeverityWarning
				icon := "⚠️"
				if sysStatus.CPUPercent >= 90.0 {
					sev = SeverityCritical
					icon = "❌"
				}

				topProcExplanation := "Spike in system workload or background processing"
				if p.TopProcesses != nil {
					if topProcs, procErr := p.TopProcesses(ctx, 3, false); procErr == nil && len(topProcs) > 0 {
						topProcExplanation = fmt.Sprintf("Top CPU consumer: %s (PID %d, %.1f%% CPU)",
							libs.SanitizeTerminal(topProcs[0].Name), topProcs[0].PID, topProcs[0].CPUPercent)
					}
				}

				report.Issues = append(report.Issues, Issue{
					Severity:       sev,
					Category:       "cpu",
					Icon:           icon,
					Title:          fmt.Sprintf("High CPU usage (%.1f%%)", sysStatus.CPUPercent),
					Impact:         "Request latency rises and background jobs can stall under CPU saturation.",
					Explanation:    fmt.Sprintf("Likely cause: %s", topProcExplanation),
					Recommendation: "Inspect active processes with 'tazx top' and investigate unexpected CPU consumers.",
				})
			}

			// RAM Check
			if sysStatus.MemoryPercent >= cfg.MemoryThreshold {
				sev := SeverityWarning
				icon := "⚠️"
				if sysStatus.MemoryPercent >= 90.0 {
					sev = SeverityCritical
					icon = "❌"
				}

				topMemExplanation := "High memory load across server processes"
				if p.TopProcesses != nil {
					if topProcs, procErr := p.TopProcesses(ctx, 3, true); procErr == nil && len(topProcs) > 0 {
						topMemExplanation = fmt.Sprintf("Top RAM consumer: %s (PID %d, %.1f%% RAM)",
							libs.SanitizeTerminal(topProcs[0].Name), topProcs[0].PID, topProcs[0].MemPercent)
					}
				}

				report.Issues = append(report.Issues, Issue{
					Severity:       sev,
					Category:       "memory",
					Icon:           icon,
					Title:          fmt.Sprintf("High Memory usage (%.1f%%)", sysStatus.MemoryPercent),
					Impact:         "Low available RAM can trigger heavy swapping or Linux OOM killer terminations.",
					Explanation:    fmt.Sprintf("Likely cause: %s", topMemExplanation),
					Recommendation: "Inspect top memory consumers with 'tazx top --sort-mem' or 'tazx memory'.",
				})
			}

			// Swap Check
			if sysStatus.SwapAvailable && sysStatus.SwapTotalBytes > 0 && sysStatus.SwapPercent >= 80.0 {
				sev := SeverityWarning
				icon := "⚠️"
				if sysStatus.SwapPercent >= 95.0 {
					sev = SeverityCritical
					icon = "❌"
				}
				report.Issues = append(report.Issues, Issue{
					Severity:       sev,
					Category:       "memory",
					Icon:           icon,
					Title:          fmt.Sprintf("High Swap utilization (%.1f%%)", sysStatus.SwapPercent),
					Impact:         "Heavy swap activity severely degrades disk I/O and application response times.",
					Explanation:    fmt.Sprintf("%.2f GB of %.2f GB swap space is currently in use", sysStatus.SwapUsedGB, sysStatus.SwapTotalGB),
					Recommendation: "Check RAM allocation and memory-heavy processes via 'tazx memory' and 'tazx top -m'.",
				})
			}
		}
	}

	// 2. Disk Space Across Partitions
	checkedPartitions := false
	if p.DiskPartitions != nil {
		parts, _, err := p.DiskPartitions(ctx, false, cfg.DiskThreshold)
		if err == nil && len(parts) > 0 {
			checkedPartitions = true
			for _, part := range parts {
				if part.UsedPercent >= cfg.DiskThreshold {
					sev := SeverityWarning
					icon := "⚠️"
					if part.UsedPercent >= 90.0 {
						sev = SeverityCritical
						icon = "❌"
					}
					report.Issues = append(report.Issues, Issue{
						Severity:       sev,
						Category:       "storage",
						Icon:           icon,
						Title:          fmt.Sprintf("High Disk usage on %s (%.1f%%)", libs.SanitizeTerminal(part.Mountpoint), part.UsedPercent),
						Impact:         "Exhausting disk space can break database writes, logging, and container startups.",
						Explanation:    fmt.Sprintf("Filesystem %s (%s free of %s total) exceeded threshold %.0f%%", libs.SanitizeTerminal(part.Filesystem), libs.FormatBytes(part.AvailableBytes), libs.FormatBytes(part.TotalBytes), cfg.DiskThreshold),
						Recommendation: fmt.Sprintf("Inspect large directories with 'tazx storage --large --dir %s' and review log rotation.", libs.SanitizeTerminal(part.Mountpoint)),
					})
				}
			}
		}
	}
	if !checkedPartitions && report.SystemSummary != nil && report.SystemSummary.DiskPercent >= cfg.DiskThreshold {
		sev := SeverityWarning
		icon := "⚠️"
		if report.SystemSummary.DiskPercent >= 90.0 {
			sev = SeverityCritical
			icon = "❌"
		}
		report.Issues = append(report.Issues, Issue{
			Severity:       sev,
			Category:       "storage",
			Icon:           icon,
			Title:          fmt.Sprintf("High Disk usage on / (%.1f%%)", report.SystemSummary.DiskPercent),
			Impact:         "Exhausting disk space can break database writes, logging, and service restarts.",
			Explanation:    "Root filesystem exceeded configured disk threshold.",
			Recommendation: "Inspect large directories with 'tazx storage --large --dir /var' and clean rotated logs.",
		})
	}

	// 3. Systemd Failed Services (Linux only; skipped cleanly when systemd is unavailable)
	if p.ListServices != nil {
		failedUnits, svcErr := p.ListServices(ctx, true)
		if svcErr != nil {
			if !errors.Is(svcErr, services.ErrSystemdUnavailable) {
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityInfo,
					Category:       "services",
					Icon:           "ℹ️",
					Title:          "Could not inspect systemd services",
					Impact:         "Failed background services may go unnoticed.",
					Explanation:    svcErr.Error(),
					Recommendation: "Verify 'systemctl --failed' status manually.",
				})
			}
		} else if len(failedUnits) > 0 {
			names := make([]string, 0, len(failedUnits))
			for i, u := range failedUnits {
				if i >= 5 {
					names = append(names, fmt.Sprintf("+%d more", len(failedUnits)-5))
					break
				}
				names = append(names, libs.SanitizeTerminal(u.Name))
			}
			sev := SeverityWarning
			icon := "⚠️"
			if len(failedUnits) >= 3 {
				sev = SeverityCritical
				icon = "❌"
			}
			report.Issues = append(report.Issues, Issue{
				Severity:       sev,
				Category:       "services",
				Icon:           icon,
				Title:          fmt.Sprintf("%d failed systemd service(s): %s", len(failedUnits), strings.Join(names, ", ")),
				Impact:         "Critical background daemons or web servers may be down.",
				Explanation:    "One or more systemd units entered the failed state.",
				Recommendation: "Inspect unit states with 'tazx services --failed' and check 'journalctl -u <service> -n 50'.",
			})
		}
	}

	// 4. Docker Daemon & Container Health
	if p.DockerConfigured != nil && p.DockerConfigured() && p.ListContainers != nil {
		containers, dErr := p.ListContainers(ctx, true)
		if dErr != nil {
			_, hint := docker.ClassifyDockerError(dErr)
			report.Issues = append(report.Issues, Issue{
				Severity:       SeverityWarning,
				Category:       "docker",
				Icon:           "⚠️",
				Title:          "Docker daemon unavailable or inaccessible",
				Impact:         "Containerized services cannot be inspected and may be stopped.",
				Explanation:    dErr.Error(),
				Recommendation: hint,
			})
		} else {
			var failedNames []string
			for _, c := range containers {
				if isUnexpectedContainerFailure(c) {
					name := c.Names
					if name == "" {
						name = c.ID
					}
					failedNames = append(failedNames, fmt.Sprintf("%s (%s)", libs.SanitizeTerminal(name), libs.SanitizeTerminal(c.Status)))
				}
			}
			if len(failedNames) > 0 {
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Category:       "docker",
					Icon:           "⚠️",
					Title:          fmt.Sprintf("%d Docker container(s) exited unexpectedly or restarting", len(failedNames)),
					Impact:         "Containerized workloads may be crashing or stuck in restart loops.",
					Explanation:    fmt.Sprintf("Affected containers: %s", strings.Join(failedNames, ", ")),
					Recommendation: "Inspect container states with 'tazx docker ps -a' and review 'docker logs <container>'.",
				})
			}
		}
	}

	// 5. Web Access Log Analysis
	if p.AnalyzeLogs != nil {
		logSummary, logErr := p.AnalyzeLogs(ctx, cfg.LogPath)
		if logErr != nil {
			switch {
			case errors.Is(logErr, os.ErrNotExist):
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityInfo,
					Category:       "logs",
					Icon:           "ℹ️",
					Title:          fmt.Sprintf("Configured log file not found (%s)", libs.SanitizeTerminal(cfg.LogPath)),
					Impact:         "HTTP error rate, bot scan, and brute-force checks were skipped.",
					Explanation:    "No web access log exists at the configured path.",
					Recommendation: "Pass a valid path with '--path <file>' or update 'log_path' in ~/.tazx.yaml.",
				})
			case errors.Is(logErr, os.ErrPermission):
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Category:       "logs",
					Icon:           "⚠️",
					Title:          fmt.Sprintf("Permission denied reading log file (%s)", libs.SanitizeTerminal(cfg.LogPath)),
					Impact:         "HTTP traffic and security anomalies in access logs could not be evaluated.",
					Explanation:    "Current user lacks read permission for the configured log file.",
					Recommendation: "Grant read-only log access via the 'adm' group or POSIX ACLs (avoid running monitoring tools as root).",
				})
			default:
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Category:       "logs",
					Icon:           "⚠️",
					Title:          fmt.Sprintf("Unable to analyze log file (%s)", libs.SanitizeTerminal(cfg.LogPath)),
					Impact:         "Log-based error and anomaly checks were skipped.",
					Explanation:    logErr.Error(),
					Recommendation: "Verify that log_path points to a readable regular file.",
				})
			}
		} else if logSummary != nil {
			report.LogSummary = logSummary

			// Bot scanning check
			if len(logSummary.SuspiciousRoutes) > 0 {
				routes := make([]string, 0, len(logSummary.SuspiciousRoutes))
				for r := range logSummary.SuspiciousRoutes {
					routes = append(routes, r)
				}
				sort.Strings(routes)

				var scanDetails []string
				totalBotReqs := 0
				for i, route := range routes {
					count := logSummary.SuspiciousRoutes[route]
					totalBotReqs += count
					if i < 5 {
						scanDetails = append(scanDetails, fmt.Sprintf("%d requests to %s", count, libs.SanitizeTerminal(route)))
					}
				}
				if len(routes) > 5 {
					scanDetails = append(scanDetails, fmt.Sprintf("+%d more routes", len(routes)-5))
				}

				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Category:       "security",
					Icon:           "⚠️",
					Title:          fmt.Sprintf("%d bot/scan requests detected (%s)", totalBotReqs, strings.Join(scanDetails, ", ")),
					Impact:         "Automated scanners probe for exposed admin panels, credentials, or vulnerable endpoints.",
					Explanation:    "Requests matched common vulnerability scan paths in web access logs.",
					Recommendation: "Verify these endpoints return 404/403 and consider rate limiting or Fail2Ban for offending IPs.",
				})
			}

			// Brute-force / unauthorized probe check
			if logSummary.FailedLogins >= 3 {
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Category:       "security",
					Icon:           "⚠️",
					Title:          fmt.Sprintf("Multiple failed authentication responses detected (%d attempts)", logSummary.FailedLogins),
					Impact:         "Repeated 401/403 or failed /login requests may indicate credential stuffing or brute-force attempts.",
					Explanation:    "Elevated 401/403 responses or failed /login requests observed in logs.",
					Recommendation: "Inspect client IPs with 'tazx logs --routes' and enforce rate limiting or MFA.",
				})
			}

			// 5xx Server Error Spike & General HTTP Error Rate
			if logSummary.TotalRequests > 0 {
				rate5xx := (float64(logSummary.Status5xx) / float64(logSummary.TotalRequests)) * 100
				if rate5xx >= 5.0 && logSummary.Status5xx >= 3 {
					report.Issues = append(report.Issues, Issue{
						Severity:       SeverityCritical,
						Category:       "logs",
						Icon:           "❌",
						Title:          fmt.Sprintf("High HTTP 5xx server error rate (%.1f%%)", rate5xx),
						Impact:         "Users are receiving internal server errors or upstream gateway failures.",
						Explanation:    fmt.Sprintf("%d of %d requests returned HTTP 5xx responses", logSummary.Status5xx, logSummary.TotalRequests),
						Recommendation: "Inspect failing requests with 'tazx logs --errors' and check backend service health.",
					})
				} else if logSummary.ErrorRate >= 15.0 && logSummary.HTTPErrorCount >= 3 {
					report.Issues = append(report.Issues, Issue{
						Severity:       SeverityWarning,
						Category:       "logs",
						Icon:           "⚠️",
						Title:          fmt.Sprintf("Elevated HTTP Error Rate (%.1f%% errors)", logSummary.ErrorRate),
						Impact:         "A large share of requests are failing with 4xx/5xx responses.",
						Explanation:    fmt.Sprintf("%d of %d requests resulted in 4xx/5xx responses", logSummary.HTTPErrorCount, logSummary.TotalRequests),
						Recommendation: "Inspect error entries with 'tazx logs --errors'.",
					})
				}
			}

			// Route concentration check
			if len(logSummary.TopRoutes) > 0 && logSummary.TotalRequests > 5 {
				topRoute := logSummary.TopRoutes[0]
				routePercent := (float64(topRoute.Count) / float64(logSummary.TotalRequests)) * 100
				if routePercent >= 50.0 && topRoute.Count >= 5 {
					report.Issues = append(report.Issues, Issue{
						Severity:       SeverityInfo,
						Category:       "traffic",
						Icon:           "ℹ️",
						Title:          fmt.Sprintf("High traffic concentration on %s (%d requests, %.0f%% of total)", libs.SanitizeTerminal(topRoute.Route), topRoute.Count, routePercent),
						Impact:         "Heavy traffic on a single endpoint can bottleneck downstream databases or workers.",
						Explanation:    "Likely cause: traffic spike, client polling loop, or scraper targeting one endpoint.",
						Recommendation: "Check top routes and client IPs with 'tazx logs --routes'.",
					})
				}
			}
		}
	}

	// 6. Common Server / Configuration Checks
	if cfgFilePath != "" && p.StatFile != nil {
		if info, err := p.StatFile(cfgFilePath); err == nil {
			if info.Mode().Perm()&0077 != 0 {
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Category:       "config",
					Icon:           "⚠️",
					Title:          fmt.Sprintf("Tazx config file has overly permissive mode (%04o)", info.Mode().Perm()),
					Impact:         "Local users could read or tamper with monitoring configuration.",
					Explanation:    fmt.Sprintf("File %s is accessible to group or other users.", libs.SanitizeTerminal(cfgFilePath)),
					Recommendation: fmt.Sprintf("Restrict file permissions with 'chmod 600 %s'.", libs.SanitizeTerminal(cfgFilePath)),
				})
			}
		}
	}

	if p.ReadDNS != nil {
		if dnsCfg, err := p.ReadDNS(); err == nil && dnsCfg != nil && len(dnsCfg.Nameservers) == 0 {
			report.Issues = append(report.Issues, Issue{
				Severity:       SeverityWarning,
				Category:       "network",
				Icon:           "⚠️",
				Title:          "No DNS nameservers found in /etc/resolv.conf",
				Impact:         "Outbound hostname resolution and package/API connections may fail.",
				Explanation:    "/etc/resolv.conf was parsed but contains no nameserver entries.",
				Recommendation: "Check systemd-resolved, NetworkManager, or /etc/resolv.conf configuration.",
			})
		}
	}

	// Sort deterministically: Critical -> Warning -> Info
	sevOrder := map[Severity]int{
		SeverityCritical: 0,
		SeverityWarning:  1,
		SeverityInfo:     2,
	}
	sort.SliceStable(report.Issues, func(i, j int) bool {
		return sevOrder[report.Issues[i].Severity] < sevOrder[report.Issues[j].Severity]
	})

	return report, nil
}
