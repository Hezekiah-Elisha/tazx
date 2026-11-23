package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Hezekiah-Elisha/tazx/internal/docker"
	"github.com/Hezekiah-Elisha/tazx/internal/logs"
	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var watchErrorsOnly bool

var watchCmd = &cobra.Command{
	Use:   "watch [resource]",
	Short: "Watch a resource in real time (cpu, logs, docker)",
	Long:  `Continuously monitor a server resource like CPU usage, logs, or Docker containers in real time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			return fmt.Errorf("--json is not supported for continuous 'tazx watch' streams")
		}
		if len(args) == 0 {
			return fmt.Errorf("specify a resource to watch: cpu, logs, or docker (example: tazx watch cpu)")
		}

		ctx, cancel := signalContext()
		defer cancel()

		switch strings.ToLower(args[0]) {
		case "cpu":
			return watchCPU(ctx)
		case "logs":
			return runWatchLogs(ctx, args[1:])
		case "docker":
			return watchDocker(ctx)
		default:
			return fmt.Errorf("unknown resource %q (available: cpu, logs, docker)", args[0])
		}
	},
}

var watchCpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "Watch CPU utilization in real time",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			return fmt.Errorf("--json is not supported for continuous 'tazx watch' streams")
		}
		ctx, cancel := signalContext()
		defer cancel()
		return watchCPU(ctx)
	},
}

var watchLogsCmd = &cobra.Command{
	Use:   "logs [service|path]",
	Short: "Watch server log stream in real time (defaults to nginx)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			return fmt.Errorf("--json is not supported for continuous 'tazx watch' streams")
		}
		ctx, cancel := signalContext()
		defer cancel()
		return runWatchLogs(ctx, args)
	},
}

var watchDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Watch Docker containers in real time",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			return fmt.Errorf("--json is not supported for continuous 'tazx watch' streams")
		}
		ctx, cancel := signalContext()
		defer cancel()
		return watchDocker(ctx)
	},
}

func runWatchLogs(ctx context.Context, args []string) error {
	meta, err := GetAppConfigWithMeta()
	if err != nil {
		return err
	}
	logPath := resolveLogPath(args, meta.Config)
	libs.Colorize(libs.Cyan, fmt.Sprintf("Streaming logs from %s (Press Ctrl+C to stop)...\n\n", libs.SanitizeTerminal(logPath)))
	if err := logs.FollowLogs(ctx, logPath, watchErrorsOnly, printLogEntry); err != nil {
		printLogError(logPath, err)
		return err
	}
	fmt.Println()
	return nil
}

func watchCPU(ctx context.Context) error {
	sampler := system.NewProcessSampler()
	screen := libs.NewLiveScreen(os.Stdout)

	return libs.RunLiveLoop(ctx, screen, 2*time.Second, "", func(runCtx context.Context, w io.Writer) error {
		sysStatus, err := system.GetSystemStatus(runCtx)
		if runCtx.Err() != nil {
			return runCtx.Err()
		}

		libs.ColorizeTo(w, libs.Bold, "⚡ TAZX WATCH - Real-Time CPU Monitor\n\n")
		if err != nil {
			libs.ColorizeTo(w, libs.Red, fmt.Sprintf("Error: %v\n", err))
		} else {
			cpuColor := libs.Green
			if sysStatus.CPUPercent > 80 {
				cpuColor = libs.Red
			} else if sysStatus.CPUPercent > 50 {
				cpuColor = libs.Yellow
			}
			libs.ColorizeTo(w, cpuColor, fmt.Sprintf("CPU Load: %5.1f%%", sysStatus.CPUPercent))
			if sysStatus.LoadAvgAvailable {
				fmt.Fprintf(w, "  |  Load Avg: %.2f, %.2f, %.2f", sysStatus.LoadAvg1, sysStatus.LoadAvg5, sysStatus.LoadAvg15)
			}
			fmt.Fprint(w, "\n\n")

			procs, procErr := sampler.Sample(runCtx, 8, false)
			if procErr == nil {
				fmt.Fprintf(w, "%-8s %-24s %s\n", "PID", "NAME", "CPU %")
				fmt.Fprintln(w, "------------------------------------------")
				for _, p := range procs {
					fmt.Fprintf(w, "%-8d %-24s %.1f%%\n",
						p.PID,
						libs.Truncate(libs.SanitizeTerminal(p.Name), 24),
						p.CPUPercent)
				}
			}
		}

		libs.ColorizeTo(w, libs.Gray, "\nRefreshing every 2s. Press Ctrl+C to stop watching...\n")
		return nil
	})
}

func watchDocker(ctx context.Context) error {
	screen := libs.NewLiveScreen(os.Stdout)

	return libs.RunLiveLoop(ctx, screen, 3*time.Second, "", func(runCtx context.Context, w io.Writer) error {
		containers, err := docker.GetContainers(runCtx, true)
		if runCtx.Err() != nil {
			return runCtx.Err()
		}

		libs.ColorizeTo(w, libs.Bold, "🐳 TAZX WATCH - Docker Containers\n\n")
		if err != nil {
			libs.ColorizeTo(w, libs.Yellow, fmt.Sprintf("Docker error: %v\n", err))
			_, hint := docker.ClassifyDockerError(err)
			if hint != "" {
				libs.ColorizeTo(w, libs.Cyan, fmt.Sprintf("💡 Tip: %s\n", hint))
			}
		} else if len(containers) == 0 {
			libs.ColorizeTo(w, libs.Yellow, "No Docker containers found.\n")
		} else {
			fmt.Fprintf(w, "%-14s %-24s %-26s %s\n", "ID", "NAMES", "IMAGE", "STATUS")
			fmt.Fprintln(w, "-----------------------------------------------------------------------------------")
			for _, c := range containers {
				statusColor := libs.Green
				if c.State != "running" {
					statusColor = libs.Yellow
				}
				fmt.Fprintf(w, "%s %s %s %s\n",
					libs.PadRight(c.ID, 14),
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(c.Names), 24), 24),
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(c.Image), 26), 26),
					libs.SprintColor(statusColor, libs.SanitizeTerminal(c.Status)),
				)
			}
		}

		libs.ColorizeTo(w, libs.Gray, "\nRefreshing every 3s. Press Ctrl+C to stop watching...\n")
		return nil
	})
}

func init() {
	watchLogsCmd.Flags().BoolVarP(&watchErrorsOnly, "errors", "e", false, "filter log stream for error entries")
	watchCmd.Flags().BoolVarP(&watchErrorsOnly, "errors", "e", false, "filter log stream for error entries (with 'logs')")
	watchCmd.AddCommand(watchCpuCmd, watchLogsCmd, watchDockerCmd)
}
