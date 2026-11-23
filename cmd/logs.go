package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/internal/logs"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	errorsOnly bool
	routesOnly bool
	tailLines  int
	followLogs bool
)

// resolveLogPath selects the target log file from positional arg, --path flag, or config.
func resolveLogPath(args []string, cfg config.Config) string {
	if logPathFlag != "" {
		return logPathFlag
	}
	if len(args) > 0 {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "nginx":
			return config.FindNginxLogPath()
		case "apache", "apache2", "httpd":
			return config.FindApacheLogPath()
		default:
			return args[0]
		}
	}
	return cfg.LogPath
}

var logsCmd = &cobra.Command{
	Use:   "logs [service|path]",
	Short: "View and analyze server logs (defaults to nginx)",
	Long:  `Access and analyze server access logs (defaults to Nginx) to identify errors, unusual routes, and traffic patterns. Specify 'nginx', 'apache', or a custom log file path.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if followLogs && jsonFlag {
			return fmt.Errorf("--json is not supported with --follow")
		}
		if tailLines < 1 {
			return fmt.Errorf("--lines must be at least 1 (got %d)", tailLines)
		}

		ctx, cancel := signalContext()
		defer cancel()

		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}
		logPath := resolveLogPath(args, meta.Config)

		if followLogs {
			libs.Colorize(libs.Cyan, fmt.Sprintf("Streaming logs from %s (Press Ctrl+C to stop)...\n\n", libs.SanitizeTerminal(logPath)))
			if err := logs.FollowLogs(ctx, logPath, errorsOnly, printLogEntry); err != nil {
				printLogError(logPath, err)
				return err
			}
			fmt.Println()
			return nil
		}

		if routesOnly {
			summary, err := logs.AnalyzeLogs(ctx, logPath)
			if err != nil {
				printLogError(logPath, err)
				return err
			}
			if jsonFlag {
				return libs.PrintJSON(os.Stdout, summary)
			}

			libs.Colorize(libs.Bold, fmt.Sprintf("\n📜 Route & Traffic Analysis for %s\n\n", libs.SanitizeTerminal(logPath)))
			fmt.Printf("Total Requests: %d\n", summary.TotalRequests)
			fmt.Printf("Requests / Min: %.1f\n", summary.RequestsPerMin)
			fmt.Printf("Error Rate:     %.1f%% (%d errors)\n", summary.ErrorRate, summary.HTTPErrorCount)
			if summary.UnparsedLines > 0 {
				fmt.Printf("Unparsed Lines: %d\n", summary.UnparsedLines)
			}

			libs.PrintSection(os.Stdout, "TOP ROUTES")
			if len(summary.TopRoutes) == 0 {
				fmt.Println("  No HTTP routes recorded.")
			} else {
				for _, r := range summary.TopRoutes {
					libs.Colorize(libs.Green, fmt.Sprintf("  %-35s : %d requests\n", libs.Truncate(r.Route, 35), r.Count))
				}
			}

			if len(summary.SuspiciousRoutes) > 0 {
				libs.PrintSection(os.Stdout, "SUSPICIOUS / SCAN ROUTES")
				susRoutes := make([]string, 0, len(summary.SuspiciousRoutes))
				for r := range summary.SuspiciousRoutes {
					susRoutes = append(susRoutes, r)
				}
				sort.Strings(susRoutes)
				for _, r := range susRoutes {
					libs.Colorize(libs.Red, fmt.Sprintf("  %-35s : %d requests\n", libs.Truncate(r, 35), summary.SuspiciousRoutes[r]))
				}
			}

			if len(summary.TopIPs) > 0 {
				libs.PrintSection(os.Stdout, "TOP CLIENT IPS")
				for _, ipStr := range summary.TopIPs {
					libs.Colorize(libs.Cyan, fmt.Sprintf("  %s\n", ipStr))
				}
			}
			fmt.Println()
			return nil
		}

		entries, err := logs.TailLogs(ctx, logPath, tailLines, errorsOnly)
		if err != nil {
			printLogError(logPath, err)
			return err
		}

		if jsonFlag {
			if entries == nil {
				entries = []logs.LogEntry{}
			}
			return libs.PrintJSON(os.Stdout, struct {
				LogPath    string          `json:"log_path"`
				ErrorsOnly bool            `json:"errors_only"`
				Count      int             `json:"count"`
				Entries    []logs.LogEntry `json:"entries"`
			}{
				LogPath:    logPath,
				ErrorsOnly: errorsOnly,
				Count:      len(entries),
				Entries:    entries,
			})
		}

		title := fmt.Sprintf("Server Logs (%s)", libs.SanitizeTerminal(logPath))
		if errorsOnly {
			title = fmt.Sprintf("Error Logs (%s)", libs.SanitizeTerminal(logPath))
		}
		libs.Colorize(libs.Bold, fmt.Sprintf("\n📜 %s [Last %d entries]\n\n", title, len(entries)))

		if len(entries) == 0 {
			libs.Colorize(libs.Yellow, "No matching log entries found.\n\n")
			return nil
		}

		for _, entry := range entries {
			printLogEntry(entry)
		}
		fmt.Println()
		return nil
	},
}

func printLogEntry(entry logs.LogEntry) {
	if !entry.Parsed {
		if entry.IsError {
			libs.Colorize(libs.Red, entry.RawLine+"\n")
		} else {
			fmt.Println(entry.RawLine)
		}
		return
	}

	statusColor := libs.Green
	if entry.Status >= 500 {
		statusColor = libs.Red
	} else if entry.Status >= 400 {
		statusColor = libs.Yellow
	}

	statusStr := libs.SprintColor(statusColor, fmt.Sprintf("[%d]", entry.Status))
	timeStr := "--:--:--"
	if entry.HasTimestamp {
		timeStr = entry.Timestamp.Format("15:04:05")
	}

	fmt.Printf("%s %-15s %s %-6s %s\n", timeStr, entry.IP, statusStr, entry.Method, entry.Route)
}

func printLogError(logPath string, err error) {
	safePath := libs.SanitizeTerminal(logPath)
	if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory") {
		libs.ColorizeTo(os.Stderr, libs.Yellow, fmt.Sprintf("Log file not found at '%s'.\n", safePath))
		libs.ColorizeTo(os.Stderr, libs.Gray, "Specify a custom log path with '--path <file>' (for example: tazx logs -p /var/log/nginx/access.log).\n")
	} else if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		libs.ColorizeTo(os.Stderr, libs.Yellow, fmt.Sprintf("Permission denied accessing '%s'.\n", safePath))
		libs.ColorizeTo(os.Stderr, libs.Gray, "Grant read-only access to your user (for example via the 'adm' group or 'setfacl -m u:$USER:r'), rather than running Tazx as root.\n")
	}
}

func init() {
	logsCmd.Flags().BoolVarP(&errorsOnly, "errors", "e", false, "filter log entries for errors (HTTP >= 400 or error lines)")
	logsCmd.Flags().BoolVarP(&routesOnly, "routes", "r", false, "display route statistics and traffic patterns")
	logsCmd.Flags().IntVarP(&tailLines, "lines", "n", 30, "number of log lines to show")
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "follow log stream in real time")
}
