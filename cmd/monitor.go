package cmd

import (
	"fmt"
	"time"

	"tazx/internal/doctor"
	"tazx/internal/logs"
	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor your server in real-time",
	Long:  `Keep an eye on server performance, live logs, and active diagnostics with real-time continuous refresh.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetAppConfig()
		interval := time.Duration(cfg.RefreshRate) * time.Second

		for {
			clearScreen()
			libs.Colorize(libs.Bold, "⚡ TAZX REAL-TIME SERVER MONITOR\n")
			fmt.Printf("Time: %s | Refresh: %ds | Config: %s\n\n", time.Now().Format("15:04:05"), cfg.RefreshRate, cfg.LogPath)

			sysStatus, sysErr := system.GetSystemStatus()
			if sysErr == nil {
				fmt.Println("=== SYSTEM METRICS ===")
				fmt.Printf("CPU: %.1f%%  |  RAM: %.1fGB/%.1fGB (%.1f%%)  |  Disk: %.1f%% [%s]\n",
					sysStatus.CPUPercent, sysStatus.MemoryUsedGB, sysStatus.MemoryTotalGB, sysStatus.MemoryPercent, sysStatus.DiskPercent, sysStatus.DiskStatus)
			}

			logSummary, logErr := logs.AnalyzeLogs(cfg.LogPath)
			if logErr == nil && logSummary != nil {
				fmt.Println("\n=== LOG & TRAFFIC STATS ===")
				fmt.Printf("Req/min: %.0f  |  Total: %d  |  Errors: %d  |  Failed Logins: %d\n",
					logSummary.RequestsPerMin, logSummary.TotalRequests, logSummary.ErrorCount, logSummary.FailedLogins)
			}

			docReport, _ := doctor.RunDiagnostics(cfg)
			fmt.Println("\n=== DIAGNOSTICS & ALERTS ===")
			if docReport != nil && len(docReport.Issues) > 0 {
				for _, issue := range docReport.Issues {
					color := libs.Yellow
					if issue.Severity == doctor.SeverityCritical {
						color = libs.Red
					}
					libs.Colorize(color, fmt.Sprintf("%s %s\n", issue.Icon, issue.Title))
				}
			} else {
				libs.Colorize(libs.Green, "✅ All server parameters normal.\n")
			}

			libs.Colorize(libs.Gray, "\nPress Ctrl+C to stop monitoring...\n")
			time.Sleep(interval)
		}
	},
}
