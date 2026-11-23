package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/internal/doctor"
	"github.com/Hezekiah-Elisha/tazx/internal/logs"
	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	monitorOnce    bool
	monitorRefresh int
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor your server in real time",
	Long:  `Continuously monitor server performance, access log traffic, and active diagnostics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag && !monitorOnce {
			return fmt.Errorf("--json requires --once for 'tazx monitor'")
		}

		ctx, cancel := signalContext()
		defer cancel()

		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}
		cfg := meta.Config

		refresh := cfg.RefreshRate
		if monitorRefresh > 0 {
			refresh = monitorRefresh
		}
		if refresh < 1 {
			refresh = 2
		}

		docCfgPath := ""
		if meta.LoadedFromFile {
			docCfgPath = meta.Path
		}

		sampler := system.NewProcessSampler()

		if monitorOnce {
			return renderMonitorView(ctx, os.Stdout, cfg, docCfgPath, refresh, sampler, true)
		}

		screen := libs.NewLiveScreen(os.Stdout)
		return libs.RunLiveLoop(ctx, screen, time.Duration(refresh)*time.Second, "", func(runCtx context.Context, w io.Writer) error {
			return renderMonitorView(runCtx, w, cfg, docCfgPath, refresh, sampler, false)
		})
	},
}

func renderMonitorView(
	ctx context.Context,
	w io.Writer,
	cfg config.Config,
	docCfgPath string,
	refresh int,
	sampler *system.ProcessSampler,
	oneShot bool,
) error {
	sysStatus, sysErr := system.GetSystemStatus(ctx)
	if sysErr != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	logSummary, logErr := logs.AnalyzeLogs(ctx, cfg.LogPath)
	providers := doctor.DefaultProviders()
	providers.SystemStatus = func(context.Context) (*system.SystemStatus, error) {
		return sysStatus, sysErr
	}
	if sampler != nil {
		providers.TopProcesses = sampler.Sample
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
		return libs.PrintJSON(w, struct {
			Timestamp string               `json:"timestamp"`
			System    *system.SystemStatus `json:"system,omitempty"`
			Logs      *logs.LogSummary     `json:"logs,omitempty"`
			Alerts    []doctor.Issue       `json:"alerts"`
		}{
			Timestamp: time.Now().Format(time.RFC3339),
			System:    sysStatus,
			Logs:      logSummary,
			Alerts:    issues,
		})
	}

	libs.ColorizeTo(w, libs.Bold, "⚡ TAZX REAL-TIME SERVER MONITOR\n")
	fmt.Fprintf(w, "Time: %s | Refresh: %ds | Log Path: %s\n",
		time.Now().Format("15:04:05"), refresh, libs.SanitizeTerminal(cfg.LogPath))

	libs.PrintSection(w, "SYSTEM METRICS")
	if sysErr != nil {
		libs.ColorizeTo(w, libs.Red, fmt.Sprintf("Error fetching system metrics: %v\n", sysErr))
	} else {
		diskStatus := system.DiskStatusFor(sysStatus.DiskPercent, cfg.DiskThreshold)
		fmt.Fprintf(w, "CPU: %5.1f%%  |  RAM: %5.1fGB/%5.1fGB (%5.1f%%)  |  Disk (/): %5.1f%% [%-8s]\n",
			sysStatus.CPUPercent, sysStatus.MemoryUsedGB, sysStatus.MemoryTotalGB, sysStatus.MemoryPercent, sysStatus.DiskPercent, diskStatus)
		if sysStatus.HostName != "" {
			fmt.Fprintf(w, "Host: %s | Uptime: %s", libs.SanitizeTerminal(sysStatus.HostName), libs.FormatDuration(sysStatus.UptimeSeconds))
			if sysStatus.LoadAvgAvailable {
				fmt.Fprintf(w, " | Load: %.2f, %.2f, %.2f", sysStatus.LoadAvg1, sysStatus.LoadAvg5, sysStatus.LoadAvg15)
			}
			fmt.Fprintln(w)
		}
	}

	libs.PrintSection(w, "LOG & TRAFFIC STATS")
	if logErr != nil || logSummary == nil {
		libs.ColorizeTo(w, libs.Gray, fmt.Sprintf("Logs unavailable: %v\n", logErr))
	} else {
		fmt.Fprintf(w, "Req/min: %6.1f  |  Total: %-7d  |  Errors: %-5d (%5.1f%%)  |  Failed Logins: %d\n",
			logSummary.RequestsPerMin, logSummary.TotalRequests, logSummary.ErrorCount, logSummary.ErrorRate, logSummary.FailedLogins)
	}

	libs.PrintSection(w, "DIAGNOSTICS & ALERTS")
	if docReport != nil && len(docReport.Issues) > 0 {
		for _, issue := range docReport.Issues {
			color := libs.Yellow
			if issue.Severity == doctor.SeverityCritical {
				color = libs.Red
			} else if issue.Severity == doctor.SeverityInfo {
				color = libs.Cyan
			}
			libs.ColorizeTo(w, color, fmt.Sprintf("%s [%s] %s\n", issue.Icon, issue.Severity, issue.Title))
		}
	} else {
		libs.ColorizeTo(w, libs.Green, "✅ All server parameters normal.\n")
	}

	if !oneShot {
		libs.ColorizeTo(w, libs.Gray, fmt.Sprintf("\nRefreshing every %ds. Press Ctrl+C to stop monitoring...\n", refresh))
	} else {
		fmt.Fprintln(w)
	}
	return nil
}

func init() {
	monitorCmd.Flags().BoolVar(&monitorOnce, "once", false, "render a single monitor snapshot and exit")
	monitorCmd.Flags().IntVarP(&monitorRefresh, "refresh", "r", 0, "override refresh interval in seconds")
}
