package cmd

import (
	"fmt"

	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var cpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "View server CPU utilization and core details",
	Long:  `View detailed server CPU metrics including usage percentage, core topology, and model information.`,
	Run: func(cmd *cobra.Command, args []string) {
		sysStatus, err := system.GetSystemStatus()
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error fetching CPU metrics: %v\n", err))
			return
		}

		libs.Colorize(libs.Bold, "\n💻 CPU Utilization & Hardware Specs\n\n")

		color := libs.Green
		if sysStatus.CPUPercent > 85.0 {
			color = libs.Red
		} else if sysStatus.CPUPercent > 70.0 {
			color = libs.Yellow
		}

		libs.Colorize(color, fmt.Sprintf("Overall CPU Usage: %.1f%%\n\n", sysStatus.CPUPercent))

		libs.Colorize(libs.Cyan, "--- Logical Core Information ---\n")
		if len(sysStatus.CPUInfo) == 0 {
			fmt.Println("No detailed CPU hardware topology returned.")
		} else {
			for i, info := range sysStatus.CPUInfo {
				fmt.Printf("Core #%-2d | Model: %-35s | Cores: %d | Speed: %.2f MHz\n",
					i, info.ModelName, info.Cores, info.Mhz)
			}
		}

		// Top CPU processes preview
		procs, procErr := system.GetTopProcesses(5, false)
		if procErr == nil && len(procs) > 0 {
			libs.Colorize(libs.Cyan, "\n--- Top CPU Processes ---\n")
			for _, p := range procs {
				fmt.Printf("  PID %-6d %-20s CPU: %.1f%%\n", p.PID, trunc(p.Name, 20), p.CPUPercent)
			}
		}
		fmt.Println()
	},
}
