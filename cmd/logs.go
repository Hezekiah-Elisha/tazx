package cmd

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"tazx/internal/logs"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var (
	errorsOnly  bool
	routesOnly  bool
	tailLines   int
	followLogs  bool
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View and analyze your server logs",
	Long:  `Access and analyze your server logs to identify errors, unusual routes, and traffic patterns.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetAppConfig()
		logPath := cfg.LogPath

		if followLogs {
			libs.Colorize(libs.Cyan, fmt.Sprintf("Streaming logs from %s (Press Ctrl+C to stop)...\n\n", logPath))
			followLogFile(logPath, errorsOnly)
			return
		}

		if routesOnly {
			libs.Colorize(libs.Bold, fmt.Sprintf("\n📜 Route & Traffic Analysis for %s\n\n", logPath))
			summary, err := logs.AnalyzeLogs(logPath)
			if err != nil {
				libs.Colorize(libs.Red, fmt.Sprintf("Error analyzing logs: %v\n", err))
				return
			}

			fmt.Printf("Total Requests: %d\n", summary.TotalRequests)
			fmt.Printf("Requests / Min: %.1f\n\n", summary.RequestsPerMin)

			libs.Colorize(libs.Bold, "=== TOP ROUTES ===\n")
			for _, r := range summary.TopRoutes {
				libs.Colorize(libs.Green, fmt.Sprintf("  %-30s : %d requests\n", r.Route, r.Count))
			}

			if len(summary.SuspiciousRoutes) > 0 {
				libs.Colorize(libs.Bold, "\n=== SUSPICIOUS / SCAN ROUTES ===\n")
				for r, c := range summary.SuspiciousRoutes {
					libs.Colorize(libs.Red, fmt.Sprintf("  %-30s : %d requests\n", r, c))
				}
			}

			if len(summary.TopIPs) > 0 {
				libs.Colorize(libs.Bold, "\n=== TOP CLIENT IPS ===\n")
				for _, ipStr := range summary.TopIPs {
					libs.Colorize(libs.Cyan, fmt.Sprintf("  %s\n", ipStr))
				}
			}
			fmt.Println()
			return
		}

		entries, _, err := logs.TailLogs(logPath, tailLines, errorsOnly, false)
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error reading logs: %v\n", err))
			return
		}

		title := fmt.Sprintf("Server Logs (%s)", logPath)
		if errorsOnly {
			title = fmt.Sprintf("Error Logs (%s)", logPath)
		}
		libs.Colorize(libs.Bold, fmt.Sprintf("\n📜 %s [Last %d entries]\n\n", title, len(entries)))

		if len(entries) == 0 {
			libs.Colorize(libs.Yellow, "No matching log entries found.\n\n")
			return
		}

		for _, entry := range entries {
			statusColor := libs.Green
			if entry.Status >= 500 {
				statusColor = libs.Red
			} else if entry.Status >= 400 {
				statusColor = libs.Yellow
			}

			statusStr := libs.SprintColor(statusColor, fmt.Sprintf("[%d]", entry.Status))
			timeStr := entry.Timestamp.Format("15:04:05")

			if entry.Method != "" {
				fmt.Printf("%s %s %s %-6s %s\n", timeStr, entry.IP, statusStr, entry.Method, entry.Route)
			} else {
				fmt.Printf("%s\n", entry.RawLine)
			}
		}
		fmt.Println()
	},
}

func followLogFile(path string, errorsOnly bool) {
	_ = logs.EnsureSampleLog(path)
	file, err := os.Open(path)
	if err != nil {
		libs.Colorize(libs.Red, fmt.Sprintf("Error opening file: %v\n", err))
		return
	}
	defer file.Close()

	// Seek to end
	_, _ = file.Seek(0, ioSeekEnd)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		entry, parseErr := logs.ParseLine(line)
		if parseErr == nil {
			if errorsOnly && entry.Status < 400 {
				continue
			}
			statusColor := libs.Green
			if entry.Status >= 500 {
				statusColor = libs.Red
			} else if entry.Status >= 400 {
				statusColor = libs.Yellow
			}

			statusStr := libs.SprintColor(statusColor, fmt.Sprintf("[%d]", entry.Status))
			timeStr := entry.Timestamp.Format("15:04:05")
			fmt.Printf("%s %s %s %-6s %s\n", timeStr, entry.IP, statusStr, entry.Method, entry.Route)
		} else {
			fmt.Print(line)
		}
	}
}

const ioSeekEnd = 2

func init() {
	logsCmd.Flags().BoolVarP(&errorsOnly, "errors", "e", false, "filter log entries for error status codes (>= 400)")
	logsCmd.Flags().BoolVarP(&routesOnly, "routes", "r", false, "display route statistics and traffic patterns")
	logsCmd.Flags().IntVarP(&tailLines, "lines", "n", 30, "number of log lines to show")
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "follow log stream in real time")
}
