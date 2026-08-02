package cmd

import (
	"fmt"

	"tazx/internal/doctor"
	"tazx/internal/logs"
	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of your server",
	Long:  `Get a quick overview of your server's health and performance.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetAppConfig()

		sysStatus, err := system.GetSystemStatus()
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error fetching system status: %v\n", err))
			return
		}

		fmt.Println("\n=== SYSTEM ===")
		cpuStr := fmt.Sprintf("CPU: %.1f%%", sysStatus.CPUPercent)
		if sysStatus.CPUPercent > cfg.CpuThreshold {
			cpuStr = libs.SprintColor(libs.Red, cpuStr)
		} else {
			cpuStr = libs.SprintColor(libs.Green, cpuStr)
		}

		ramStr := fmt.Sprintf("RAM: %.1fGB / %.1fGB (%.1f%%)", sysStatus.MemoryUsedGB, sysStatus.MemoryTotalGB, sysStatus.MemoryPercent)
		if sysStatus.MemoryPercent > cfg.MemoryThreshold {
			ramStr = libs.SprintColor(libs.Red, ramStr)
		} else {
			ramStr = libs.SprintColor(libs.Green, ramStr)
		}

		diskBadge := sysStatus.DiskStatus
		if sysStatus.DiskStatus == "OK" {
			diskBadge = libs.SprintColor(libs.Green, fmt.Sprintf("Disk: OK (%.1f%%)", sysStatus.DiskPercent))
		} else {
			diskBadge = libs.SprintColor(libs.Red, fmt.Sprintf("Disk: %s (%.1f%%)", sysStatus.DiskStatus, sysStatus.DiskPercent))
		}

		fmt.Printf("%s   %s   %s\n", cpuStr, ramStr, diskBadge)

		logSummary, logErr := logs.AnalyzeLogs(cfg.LogPath)
		fmt.Println("\n=== APP ===")
		if logErr != nil || logSummary == nil {
			libs.Colorize(libs.Yellow, fmt.Sprintf("Log Path: %s (No log entries parsed)\n", cfg.LogPath))
		} else {
			errPercent := 0.0
			if logSummary.TotalRequests > 0 {
				errPercent = (float64(logSummary.ErrorCount) / float64(logSummary.TotalRequests)) * 100
			}

			fmt.Printf("Requests/min: %.0f\n", logSummary.RequestsPerMin)
			fmt.Printf("Total Requests: %d\n", logSummary.TotalRequests)

			errStr := fmt.Sprintf("Errors: %d (%.1f%%)", logSummary.ErrorCount, errPercent)
			if logSummary.ErrorCount > 0 {
				libs.Colorize(libs.Yellow, errStr+"\n")
			} else {
				libs.Colorize(libs.Green, errStr+"\n")
			}

			if len(logSummary.TopRoutes) > 0 {
				fmt.Printf("Top Route: %s (%d requests)\n", logSummary.TopRoutes[0].Route, logSummary.TopRoutes[0].Count)
			}
		}

		fmt.Println("\n=== ALERTS ===")
		docReport, _ := doctor.RunDiagnostics(cfg)
		if docReport != nil && len(docReport.Issues) > 0 {
			for _, issue := range docReport.Issues {
				color := libs.Yellow
				if issue.Severity == doctor.SeverityCritical {
					color = libs.Red
				} else if issue.Severity == doctor.SeverityInfo {
					color = libs.Cyan
				}
				libs.Colorize(color, fmt.Sprintf("%s %s\n", issue.Icon, issue.Title))
			}
		} else {
			libs.Colorize(libs.Green, "✅ No active system or app alerts.\n")
		}
		fmt.Println()
	},
}
