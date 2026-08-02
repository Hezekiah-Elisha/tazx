package cmd

import (
	"fmt"

	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "View server disk storage and partition usage",
	Long:  `View detailed storage allocation and usage across root partition.`,
	Run: func(cmd *cobra.Command, args []string) {
		sysStatus, err := system.GetSystemStatus()
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error fetching storage metrics: %v\n", err))
			return
		}

		libs.Colorize(libs.Bold, "\n💾 Storage & Partition Metrics (/)\n\n")

		diskColor := libs.Green
		if sysStatus.DiskPercent > 90.0 {
			diskColor = libs.Red
		} else if sysStatus.DiskPercent > 80.0 {
			diskColor = libs.Yellow
		}

		fmt.Printf("Total Capacity: %.2f GB\n", sysStatus.DiskTotalGB)
		fmt.Printf("Used Space    : %.2f GB\n", sysStatus.DiskUsedGB)
		fmt.Printf("Free Space    : %.2f GB\n", sysStatus.DiskFreeGB)
		libs.Colorize(diskColor, fmt.Sprintf("Disk Usage    : %.2f%% [%s]\n", sysStatus.DiskPercent, sysStatus.DiskStatus))

		if sysStatus.DiskPercent > 90.0 {
			libs.Colorize(libs.Red, "\n⚠️ Critical: Disk usage is above 90%%. Consider running log rotation or cleaning temporary space.\n")
		} else if sysStatus.DiskPercent > 80.0 {
			libs.Colorize(libs.Yellow, "\n⚠️ Warning: Storage space is getting low (> 80%%).\n")
		} else {
			libs.Colorize(libs.Green, "\n✅ Disk storage is healthy.\n")
		}
		fmt.Println()
	},
}
