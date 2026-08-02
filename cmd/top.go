package cmd

import (
	"fmt"
	"time"

	"tazx/internal/system"
	"tazx/libs"

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
	Long:  `Get a real-time view of the top processes consuming CPU and Memory resources on your server.`,
	Run: func(cmd *cobra.Command, args []string) {
		for {
			clearScreen()
			renderTopView()
			if onceMode {
				break
			}
			time.Sleep(time.Duration(refreshSec) * time.Second)
		}
	},
}

func renderTopView() {
	sysStatus, err := system.GetSystemStatus()
	if err != nil {
		libs.Colorize(libs.Red, fmt.Sprintf("Error fetching system info: %v\n", err))
		return
	}

	libs.Colorize(libs.Bold, "⚡ TAZX TOP - Real-Time Server Processes\n")
	fmt.Printf("Host: %s | Platform: %s | Uptime: %ds\n", sysStatus.HostName, sysStatus.Platform, sysStatus.UptimeSeconds)
	fmt.Printf("CPU: %.1f%%  |  RAM: %.1fGB / %.1fGB (%.1f%%)  |  Disk: %.1f%%\n\n",
		sysStatus.CPUPercent, sysStatus.MemoryUsedGB, sysStatus.MemoryTotalGB, sysStatus.MemoryPercent, sysStatus.DiskPercent)

	sortLabel := "CPU %"
	if sortByMem {
		sortLabel = "RAM %"
	}
	libs.Colorize(libs.Cyan, fmt.Sprintf("Top %d Processes (Sorted by %s):\n", topLimit, sortLabel))
	fmt.Printf("%-8s %-15s %-25s %-10s %-10s %s\n", "PID", "USER", "COMMAND", "CPU %", "RAM %", "STATUS")
	fmt.Println("---------------------------------------------------------------------------------")

	procs, err := system.GetTopProcesses(topLimit, sortByMem)
	if err != nil {
		libs.Colorize(libs.Red, fmt.Sprintf("Error fetching process list: %v\n", err))
		return
	}

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

		cpuStr := libs.SprintColor(cpuColor, fmt.Sprintf("%.1f%%", p.CPUPercent))
		memStr := libs.SprintColor(memColor, fmt.Sprintf("%.1f%%", p.MemPercent))

		fmt.Printf("%-8d %-15s %-25s %-10s %-10s %s\n", p.PID, trunc(p.Username, 15), trunc(p.Name, 25), cpuStr, memStr, p.Status)
	}

	if !onceMode {
		libs.Colorize(libs.Gray, fmt.Sprintf("\nRefreshing every %ds. Press Ctrl+C to exit...\n", refreshSec))
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func init() {
	topCmd.Flags().BoolVarP(&sortByMem, "sort-mem", "m", false, "Sort processes by Memory usage instead of CPU")
	topCmd.Flags().IntVarP(&topLimit, "limit", "l", 15, "Number of processes to list")
	topCmd.Flags().IntVarP(&refreshSec, "refresh", "r", 2, "Refresh interval in seconds")
	topCmd.Flags().BoolVar(&onceMode, "once", false, "Run once and exit without live refresh")
}
