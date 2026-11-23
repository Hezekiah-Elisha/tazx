package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Hezekiah-Elisha/tazx/internal/docker"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	showAllContainers  bool
	dockerStatsWatch   bool
	dockerStatsRefresh int
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Inspect Docker containers and live resource statistics",
	Long:  `Read-only overview of Docker containers' status, port mappings, and live CPU/memory/network/disk I/O statistics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerPs(showAllContainers)
	},
}

var dockerPsCmd = &cobra.Command{
	Use:   "ps",
	Short: "List Docker containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerPs(showAllContainers)
	},
}

var dockerStatsCmd = &cobra.Command{
	Use:   "stats [container]",
	Short: "Display container CPU, memory, network, and block I/O statistics",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dockerStatsWatch && jsonFlag {
			return fmt.Errorf("--json is not supported with --watch")
		}
		if dockerStatsRefresh < 1 {
			return fmt.Errorf("--refresh must be at least 1 second (got %d)", dockerStatsRefresh)
		}
		filter := ""
		if len(args) > 0 {
			filter = args[0]
		}
		return runDockerStats(filter, dockerStatsWatch, dockerStatsRefresh)
	},
}

func printDockerErrorTip(err error) {
	_, hint := docker.ClassifyDockerError(err)
	if hint != "" {
		libs.ColorizeTo(os.Stderr, libs.Cyan, fmt.Sprintf("💡 Tip: %s\n", hint))
	}
}

func runDockerPs(all bool) error {
	ctx, cancel := signalContext()
	defer cancel()

	containers, err := docker.GetContainers(ctx, all)
	if err != nil {
		printDockerErrorTip(err)
		return err
	}

	if jsonFlag {
		if containers == nil {
			containers = []docker.ContainerInfo{}
		}
		return libs.PrintJSON(os.Stdout, containers)
	}

	libs.Colorize(libs.Bold, "\n🐳 Docker Containers Overview\n\n")

	if len(containers) == 0 {
		libs.Colorize(libs.Yellow, "No Docker containers found.\n\n")
		return nil
	}

	fmt.Printf("%-14s %-24s %-28s %-22s %s\n", "CONTAINER ID", "NAME", "IMAGE", "STATUS", "PORTS")
	fmt.Println("-----------------------------------------------------------------------------------------------------------")

	for _, c := range containers {
		statusColor := libs.Green
		if c.State != "running" {
			statusColor = libs.Yellow
		}

		fmt.Printf("%s %s %s %s %s\n",
			libs.PadRight(c.ID, 14),
			libs.PadRight(libs.Truncate(libs.SanitizeTerminal(c.Names), 24), 24),
			libs.PadRight(libs.Truncate(libs.SanitizeTerminal(c.Image), 28), 28),
			libs.ColorPadRight(statusColor, libs.SanitizeTerminal(c.Status), 22),
			libs.SanitizeTerminal(c.Ports),
		)
	}
	fmt.Println()
	return nil
}

func runDockerStats(filter string, watch bool, refresh int) error {
	ctx, cancel := signalContext()
	defer cancel()

	if !watch {
		stats, err := docker.GetContainerStats(ctx, filter)
		if err != nil {
			printDockerErrorTip(err)
			return err
		}

		if jsonFlag {
			if stats == nil {
				stats = []docker.ContainerStats{}
			}
			return libs.PrintJSON(os.Stdout, stats)
		}

		fmt.Fprintln(os.Stdout)
		writeDockerStatsTable(os.Stdout, stats, filter, false, 0)
		return nil
	}

	screen := libs.NewLiveScreen(os.Stdout)
	err := libs.RunLiveLoop(ctx, screen, time.Duration(refresh)*time.Second, "", func(runCtx context.Context, w io.Writer) error {
		stats, err := docker.GetContainerStats(runCtx, filter)
		if err != nil {
			if runCtx.Err() != nil {
				return runCtx.Err()
			}
			return err
		}
		writeDockerStatsTable(w, stats, filter, true, refresh)
		return nil
	})
	if err != nil {
		printDockerErrorTip(err)
	}
	return err
}

func writeDockerStatsTable(w io.Writer, stats []docker.ContainerStats, filter string, watch bool, refresh int) {
	libs.ColorizeTo(w, libs.Bold, "🐳 Docker Container Resource Statistics\n\n")

	if len(stats) == 0 {
		if filter != "" {
			libs.ColorizeTo(w, libs.Yellow, fmt.Sprintf("No running containers matching %q.\n", libs.SanitizeTerminal(filter)))
		} else {
			libs.ColorizeTo(w, libs.Yellow, "No running containers found.\n")
		}
	} else {
		fmt.Fprintf(w, "%-14s %-22s %-8s %-23s %-8s %-21s %-21s %s\n",
			"CONTAINER ID", "NAME", "CPU %", "MEM USAGE / LIMIT", "MEM %", "NET I/O", "BLOCK I/O", "PIDS")
		fmt.Fprintln(w, "----------------------------------------------------------------------------------------------------------------------------------")

		for _, s := range stats {
			cpuColor := libs.Green
			if s.CPUPercent >= 80.0 {
				cpuColor = libs.Red
			} else if s.CPUPercent >= 50.0 {
				cpuColor = libs.Yellow
			}

			memColor := libs.Green
			if s.MemPercent >= 85.0 {
				memColor = libs.Red
			} else if s.MemPercent >= 70.0 {
				memColor = libs.Yellow
			}

			memPair := fmt.Sprintf("%s / %s", libs.FormatBytes(s.MemUsedBytes), libs.FormatBytes(s.MemLimitBytes))
			if s.MemLimitBytes == 0 {
				memPair = fmt.Sprintf("%s / -", libs.FormatBytes(s.MemUsedBytes))
			}
			netPair := fmt.Sprintf("%s / %s", libs.FormatBytes(s.NetRxBytes), libs.FormatBytes(s.NetTxBytes))
			blkPair := fmt.Sprintf("%s / %s", libs.FormatBytes(s.BlockReadBytes), libs.FormatBytes(s.BlockWriteBytes))

			fmt.Fprintf(w, "%s %s %s %s %s %s %s %d\n",
				libs.PadRight(s.ID, 14),
				libs.PadRight(libs.Truncate(libs.SanitizeTerminal(s.Name), 22), 22),
				libs.ColorPadRight(cpuColor, fmt.Sprintf("%6.2f%%", s.CPUPercent), 8),
				libs.PadRight(memPair, 23),
				libs.ColorPadRight(memColor, fmt.Sprintf("%6.2f%%", s.MemPercent), 8),
				libs.PadRight(netPair, 21),
				libs.PadRight(blkPair, 21),
				s.PIDs,
			)
		}
	}

	if watch {
		libs.ColorizeTo(w, libs.Gray, fmt.Sprintf("\nRefreshing every %ds. Press Ctrl+C to exit...\n", refresh))
	} else {
		fmt.Fprintln(w)
	}
}

func init() {
	dockerCmd.Flags().BoolVarP(&showAllContainers, "all", "a", true, "show all containers (including stopped)")
	dockerPsCmd.Flags().BoolVarP(&showAllContainers, "all", "a", true, "show all containers (including stopped)")
	dockerStatsCmd.Flags().BoolVarP(&dockerStatsWatch, "watch", "w", false, "continuously refresh container statistics in real time")
	dockerStatsCmd.Flags().IntVarP(&dockerStatsRefresh, "refresh", "r", 2, "refresh interval in seconds when --watch is set")
	dockerCmd.AddCommand(dockerPsCmd)
	dockerCmd.AddCommand(dockerStatsCmd)
}
