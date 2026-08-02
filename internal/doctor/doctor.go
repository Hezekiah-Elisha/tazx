package doctor

import (
	"fmt"
	"sort"
	"strings"

	"tazx/internal/config"
	"tazx/internal/logs"
	"tazx/internal/system"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

type Issue struct {
	Severity       Severity
	Icon           string
	Title          string
	Explanation    string
	Recommendation string
}

type DiagnosticReport struct {
	Issues        []Issue
	SystemSummary *system.SystemStatus
	LogSummary    *logs.LogSummary
}

func RunDiagnostics(cfg config.Config) (*DiagnosticReport, error) {
	report := &DiagnosticReport{}

	// 1. Check System Metrics
	sysStatus, err := system.GetSystemStatus()
	if err == nil {
		report.SystemSummary = sysStatus

		// CPU Check
		if sysStatus.CPUPercent >= cfg.CpuThreshold {
			sev := SeverityWarning
			icon := "⚠️"
			if sysStatus.CPUPercent >= 90.0 {
				sev = SeverityCritical
				icon = "❌"
			}

			topProcExplanation := ""
			topProcs, procErr := system.GetTopProcesses(3, false)
			if procErr == nil && len(topProcs) > 0 {
				topProcExplanation = fmt.Sprintf("Top CPU consumer: %s (PID %d, %.1f%% CPU)", topProcs[0].Name, topProcs[0].PID, topProcs[0].CPUPercent)
			} else {
				topProcExplanation = "Spike in system workload or background processing"
			}

			report.Issues = append(report.Issues, Issue{
				Severity:       sev,
				Icon:           icon,
				Title:          fmt.Sprintf("High CPU usage (%.1f%%)", sysStatus.CPUPercent),
				Explanation:    fmt.Sprintf("Likely cause: %s", topProcExplanation),
				Recommendation: "Inspect background processes with 'tazx top' or limit CPU intensive tasks.",
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

			topMemExplanation := ""
			topProcs, procErr := system.GetTopProcesses(3, true)
			if procErr == nil && len(topProcs) > 0 {
				topMemExplanation = fmt.Sprintf("Top RAM consumer: %s (PID %d, %.1f%% RAM)", topProcs[0].Name, topProcs[0].PID, topProcs[0].MemPercent)
			} else {
				topMemExplanation = "High memory load across server processes"
			}

			report.Issues = append(report.Issues, Issue{
				Severity:       sev,
				Icon:           icon,
				Title:          fmt.Sprintf("High Memory usage (%.1f%%)", sysStatus.MemoryPercent),
				Explanation:    fmt.Sprintf("Likely cause: %s", topMemExplanation),
				Recommendation: "Consider freeing cache or restarting high-memory services.",
			})
		}

		// Disk Check
		if sysStatus.DiskPercent >= cfg.DiskThreshold {
			sev := SeverityWarning
			icon := "⚠️"
			if sysStatus.DiskPercent >= 90.0 {
				sev = SeverityCritical
				icon = "❌"
			}

			report.Issues = append(report.Issues, Issue{
				Severity:       sev,
				Icon:           icon,
				Title:          fmt.Sprintf("High Disk usage (%.1f%%)", sysStatus.DiskPercent),
				Explanation:    fmt.Sprintf("Likely cause: Log growth, temporary files, or large data stores on /"),
				Recommendation: "Clean up rotated logs in /var/log or purge temporary files.",
			})
		}
	}

	// 2. Check Logs
	logSummary, logErr := logs.AnalyzeLogs(cfg.LogPath)
	if logErr == nil && logSummary != nil {
		report.LogSummary = logSummary

		// Bot scanning check
		if len(logSummary.SuspiciousRoutes) > 0 {
			var scanDetails []string
			totalBotReqs := 0
			for route, count := range logSummary.SuspiciousRoutes {
				scanDetails = append(scanDetails, fmt.Sprintf("%d requests to %s", count, route))
				totalBotReqs += count
			}

			report.Issues = append(report.Issues, Issue{
				Severity:       SeverityWarning,
				Icon:           "⚠️",
				Title:          fmt.Sprintf("%d bot/scan requests detected (%s)", totalBotReqs, strings.Join(scanDetails, ", ")),
				Explanation:    "Possible automated vulnerability scanner searching for exposed admin endpoints or secrets",
				Recommendation: "Block scanner IPs or set up rate limiting / Fail2Ban.",
			})
		}

		// Brute force check
		if logSummary.FailedLogins >= 3 {
			report.Issues = append(report.Issues, Issue{
				Severity:       SeverityWarning,
				Icon:           "⚠️",
				Title:          fmt.Sprintf("Multiple failed logins detected (%d attempts)", logSummary.FailedLogins),
				Explanation:    "Possible brute-force attack targeting authentication routes or unauthorized access",
				Recommendation: "Verify authentication logs and enforce CAPTCHA or IP rate limiting.",
			})
		}

		// High Error rate check
		if logSummary.TotalRequests > 0 {
			errorRate := (float64(logSummary.ErrorCount) / float64(logSummary.TotalRequests)) * 100
			if errorRate >= 15.0 {
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityWarning,
					Icon:           "⚠️",
					Title:          fmt.Sprintf("Elevated HTTP Error Rate (%.1f%% errors)", errorRate),
					Explanation:    fmt.Sprintf("%d of %d requests resulted in 4xx/5xx responses", logSummary.ErrorCount, logSummary.TotalRequests),
					Recommendation: "Check application error logs with 'tazx logs --errors'.",
				})
			}
		}

		// Route spike check
		if len(logSummary.TopRoutes) > 0 && logSummary.TotalRequests > 5 {
			topRoute := logSummary.TopRoutes[0]
			routePercent := (float64(topRoute.Count) / float64(logSummary.TotalRequests)) * 100
			if routePercent >= 50.0 && topRoute.Count >= 5 {
				report.Issues = append(report.Issues, Issue{
					Severity:       SeverityInfo,
					Icon:           "⚠️",
					Title:          fmt.Sprintf("High traffic concentration on %s (%d requests, %.0f%% of total)", topRoute.Route, topRoute.Count, routePercent),
					Explanation:    "Likely cause: traffic spike or client polling loop on a single endpoint",
					Recommendation: "Monitor route response latency and scale endpoint caching if needed.",
				})
			}
		}
	}

	// Sort issues: Critical -> Warning -> Info
	sort.Slice(report.Issues, func(i, j int) bool {
		sevOrder := map[Severity]int{
			SeverityCritical: 0,
			SeverityWarning:  1,
			SeverityInfo:     2,
		}
		return sevOrder[report.Issues[i].Severity] < sevOrder[report.Issues[j].Severity]
	})

	return report, nil
}
