package cmd

import (
	"fmt"

	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "View server RAM and swap memory usage",
	Long:  `View detailed memory metrics including virtual memory allocation, swap utilization, and top memory consumer processes.`,
	Run: func(cmd *cobra.Command, args []string) {
		sysStatus, err := system.GetSystemStatus()
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error fetching memory stats: %v\n", err))
			return
		}

		libs.Colorize(libs.Bold, "\n🧠 Server Memory Metrics\n\n")

		memColor := libs.Green
		if sysStatus.MemoryPercent > 90.0 {
			memColor = libs.Red
		} else if sysStatus.MemoryPercent > 80.0 {
			memColor = libs.Yellow
		}

		libs.Colorize(libs.Cyan, "--- RAM Usage ---\n")
		fmt.Printf("Total Memory : %.2f GB\n", sysStatus.MemoryTotalGB)
		fmt.Printf("Used Memory  : %.2f GB\n", sysStatus.MemoryUsedGB)
		fmt.Printf("Free Memory  : %.2f GB\n", sysStatus.MemoryFreeGB)
		libs.Colorize(memColor, fmt.Sprintf("Memory Usage : %.2f%%\n", sysStatus.MemoryPercent))

		if sysStatus.MemoryPercent > 90.0 {
			libs.Colorize(libs.Red, "⚠️ Warning: Memory usage is critically high (> 90%)\n")
		} else if sysStatus.MemoryPercent > 80.0 {
			libs.Colorize(libs.Yellow, "⚠️ Warning: Memory usage is elevated (> 80%)\n")
		} else {
			libs.Colorize(libs.Green, "✅ Memory usage is within healthy limits\n")
		}

		procs, procErr := system.GetTopProcesses(5, true)
		if procErr == nil && len(procs) > 0 {
			libs.Colorize(libs.Cyan, "\n--- Top Memory Consumer Processes ---\n")
			for _, p := range procs {
				fmt.Printf("  PID %-6d %-20s RAM: %.1f%%\n", p.PID, trunc(p.Name, 20), p.MemPercent)
			}
		}
		fmt.Println()
	},
}
