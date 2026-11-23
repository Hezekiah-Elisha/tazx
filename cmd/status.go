package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/doctor"
	"github.com/Hezekiah-Elisha/tazx/internal/logs"
	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the overall status of your server",
	Long:  `Get a quick overview of CPU, RAM, disk, web traffic logs, and active diagnostic alerts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}
		cfg := meta.Config

		sysStatus, err := system.GetSystemStatus(ctx)
		if err != nil {
			return fmt.Errorf("error fetching system status: %w", err)
		}
		sysStatus.DiskStatus = system.DiskStatusFor(sysStatus.DiskPercent, cfg.DiskThreshold)

		logSummary, logErr := logs.AnalyzeLogs(ctx, cfg.LogPath)
		docCfgPath := ""
		if meta.LoadedFromFile {
			docCfgPath = meta.Path
		}
		providers := doctor.DefaultProviders()
		providers.SystemStatus = func(context.Context) (*system.SystemStatus, error) {
			return sysStatus, nil
		}
		providers.AnalyzeLogs = func(context.Context, string) (*logs.LogSummary, error) {
			return logSummary, logErr
		}
		docReport, _ := doctor.RunDiagnosticsWithProviders(ctx, cfg, docCfgPath, providers)

		if jsonFlag {
			var issues []doctor.Issue
			if docReport != nil {
				issues = docReport.Issues
			}
			var logErrMsg string
			if logErr != nil {
				logErrMsg = logErr.Error()
			}
			return libs.PrintJSON(os.Stdout, struct {
				System   *system.SystemStatus `json:"system"`
				App      *logs.LogSummary     `json:"app,omitempty"`
				LogError string               `json:"log_error,omitempty"`
				Alerts   []doctor.Issue       `json:"alerts"`
			}{
				System:   sysStatus,
				App:      logSummary,
				LogError: logErrMsg,
				Alerts:   issues,
			})
		}

		libs.PrintSection(os.Stdout, "SYSTEM")
		cpuColor := libs.Green
		if sysStatus.CPUPercent >= cfg.CpuThreshold {
			cpuColor = libs.Red
		}
		cpuStr := libs.SprintColor(cpuColor, fmt.Sprintf("CPU: %.1f%%", sysStatus.CPUPercent))

		ramColor := libs.Green
		if sysStatus.MemoryPercent >= cfg.MemoryThreshold {
			ramColor = libs.Red
		}
		ramStr := libs.SprintColor(ramColor, fmt.Sprintf("RAM: %.1fGB / %.1fGB (%.1f%%)", sysStatus.MemoryUsedGB, sysStatus.MemoryTotalGB, sysStatus.MemoryPercent))

		diskColor := libs.Green
		if sysStatus.DiskStatus == "WARNING" {
			diskColor = libs.Yellow
		} else if sysStatus.DiskStatus != "OK" {
			diskColor = libs.Red
		}
		diskBadge := libs.SprintColor(diskColor, fmt.Sprintf("Disk: %s (%.1f%%)", sysStatus.DiskStatus, sysStatus.DiskPercent))

		fmt.Printf("%s   %s   %s\n", cpuStr, ramStr, diskBadge)
		if sysStatus.HostName != "" {
			fmt.Printf("Host: %s | Uptime: %s", libs.SanitizeTerminal(sysStatus.HostName), libs.FormatDuration(sysStatus.UptimeSeconds))
			if sysStatus.LoadAvgAvailable {
				fmt.Printf(" | Load: %.2f, %.2f, %.2f", sysStatus.LoadAvg1, sysStatus.LoadAvg5, sysStatus.LoadAvg15)
			}
			fmt.Println()
		}

		libs.PrintSection(os.Stdout, "APP")
		if logErr != nil || logSummary == nil {
			libs.Colorize(libs.Yellow, fmt.Sprintf("Log Path: %s (%v)\n", libs.SanitizeTerminal(cfg.LogPath), logErr))
		} else {
			fmt.Printf("Log Path:       %s\n", libs.SanitizeTerminal(cfg.LogPath))
			fmt.Printf("Requests/min:   %.1f\n", logSummary.RequestsPerMin)
			fmt.Printf("Total Requests: %d\n", logSummary.TotalRequests)

			errStr := fmt.Sprintf("Errors:         %d (%.1f%%)", logSummary.ErrorCount, logSummary.ErrorRate)
			if logSummary.ErrorCount > 0 {
				libs.Colorize(libs.Yellow, errStr+"\n")
			} else {
				libs.Colorize(libs.Green, errStr+"\n")
			}

			if len(logSummary.TopRoutes) > 0 {
				fmt.Printf("Top Route:      %s (%d requests)\n", libs.SanitizeTerminal(logSummary.TopRoutes[0].Route), logSummary.TopRoutes[0].Count)
			}
		}

		libs.PrintSection(os.Stdout, "ALERTS")
		if docReport != nil && len(docReport.Issues) > 0 {
			for _, issue := range docReport.Issues {
				color := libs.Yellow
				if issue.Severity == doctor.SeverityCritical {
					color = libs.Red
				} else if issue.Severity == doctor.SeverityInfo {
					color = libs.Cyan
				}
				libs.Colorize(color, fmt.Sprintf("%s [%s] %s\n", issue.Icon, issue.Severity, issue.Title))
			}
		} else {
			libs.Colorize(libs.Green, "✅ No active system or app alerts.\n")
		}
		fmt.Println()
		return nil
	},
}
