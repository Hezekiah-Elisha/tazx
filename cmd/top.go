package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	sortByMem  bool
	topLimit   int
	refreshSec int
	onceMode   bool
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "View the top processes running on your server",
	Long:  `Get a real-time view of the top processes consuming CPU and memory resources on your server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag && !onceMode {
			return fmt.Errorf("--json requires --once for 'tazx top'")
		}
		if topLimit < 1 {
			return fmt.Errorf("--limit must be at least 1 (got %d)", topLimit)
		}
		if refreshSec < 1 {
			return fmt.Errorf("--refresh must be at least 1 second (got %d)", refreshSec)
		}

		ctx, cancel := signalContext()
		defer cancel()

		sampler := system.NewProcessSampler()

		if onceMode {
			return renderTopView(ctx, os.Stdout, sampler, true)
		}

		screen := libs.NewLiveScreen(os.Stdout)
		return libs.RunLiveLoop(ctx, screen, time.Duration(refreshSec)*time.Second, "", func(runCtx context.Context, w io.Writer) error {
			return renderTopView(runCtx, w, sampler, false)
		})
	},
}

func renderTopView(ctx context.Context, w io.Writer, sampler *system.ProcessSampler, oneShot bool) error {
	sysStatus, err := system.GetSystemStatus(ctx)
	if err != nil {
		return fmt.Errorf("error fetching system info: %w", err)
	}

	procs, err := sampler.Sample(ctx, topLimit, sortByMem)
	if err != nil {
		return fmt.Errorf("error fetching process list: %w", err)
	}

	if jsonFlag {
		return libs.PrintJSON(w, struct {
			System    *system.SystemStatus `json:"system"`
			SortBy    string               `json:"sort_by"`
			Processes []system.ProcessInfo `json:"processes"`
		}{
			System: sysStatus,
			SortBy: func() string {
				if sortByMem {
					return "memory"
				}
				return "cpu"
			}(),
			Processes: procs,
		})
	}

	libs.ColorizeTo(w, libs.Bold, "⚡ TAZX TOP - Server Processes\n")
	fmt.Fprintf(w, "Host: %s | Platform: %s | Uptime: %s\n",
		libs.SanitizeTerminal(sysStatus.HostName),
		libs.SanitizeTerminal(sysStatus.Platform),
		libs.FormatDuration(sysStatus.UptimeSeconds),
	)
	fmt.Fprintf(w, "CPU: %5.1f%%  |  RAM: %5.1fGB / %5.1fGB (%5.1f%%)  |  Disk (/): %5.1f%%\n\n",
		sysStatus.CPUPercent, sysStatus.MemoryUsedGB, sysStatus.MemoryTotalGB, sysStatus.MemoryPercent, sysStatus.DiskPercent)

	sortLabel := "CPU %"
	if sortByMem {
		sortLabel = "RAM %"
	}
	libs.ColorizeTo(w, libs.Cyan, fmt.Sprintf("Top %d Processes (Sorted by %s):\n", topLimit, sortLabel))
	fmt.Fprintf(w, "%-8s %-15s %-25s %-10s %-10s %s\n", "PID", "USER", "COMMAND", "CPU %", "RAM %", "STATUS")
	fmt.Fprintln(w, "---------------------------------------------------------------------------------")

	for _, p := range procs {
		cpuColor := libs.Reset
		if p.CPUPercent > 50.0 {
			cpuColor = libs.Red
		} else if p.CPUPercent > 20.0 {
			cpuColor = libs.Yellow
		}

		memColor := libs.Reset
		if p.MemPercent > 20.0 {
			memColor = libs.Yellow
		}

		fmt.Fprintf(w, "%-8d %s %s %s %s %s\n",
			p.PID,
			libs.PadRight(libs.Truncate(libs.SanitizeTerminal(p.Username), 15), 15),
			libs.PadRight(libs.Truncate(libs.SanitizeTerminal(p.Name), 25), 25),
			libs.ColorPadRight(cpuColor, fmt.Sprintf("%.1f%%", p.CPUPercent), 10),
			libs.ColorPadRight(memColor, fmt.Sprintf("%.1f%%", p.MemPercent), 10),
			libs.SanitizeTerminal(p.Status),
		)
	}

	if !oneShot {
		libs.ColorizeTo(w, libs.Gray, fmt.Sprintf("\nRefreshing every %ds. Press Ctrl+C to exit...\n", refreshSec))
	} else {
		fmt.Fprintln(w)
	}
	return nil
}

func init() {
	topCmd.Flags().BoolVarP(&sortByMem, "sort-mem", "m", false, "sort processes by Memory usage instead of CPU")
	topCmd.Flags().IntVarP(&topLimit, "limit", "l", 15, "number of processes to list")
	topCmd.Flags().IntVarP(&refreshSec, "refresh", "r", 2, "refresh interval in seconds")
	topCmd.Flags().BoolVar(&onceMode, "once", false, "run once and exit without live refresh")
}
