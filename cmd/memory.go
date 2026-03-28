package cmd

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage your server memory",
	Long:  `View and manage your server's memory usage, including RAM and swap space.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for storage command logic
		v, _ := mem.VirtualMemory()
		totalStorage := v.Total / (1024 * 1024 * 1024) // Convert bytes to GB
		freeStorage := v.Free / (1024 * 1024 * 1024)
		usedStorage := totalStorage - freeStorage
		Colorize(Blue, "--- Memory Information ---\n")
		Colorize(Red, fmt.Sprintf("Total: %v GB, Free: %v GB, Used: %v GB, UsedPercent: %f%%\n", totalStorage, freeStorage, usedStorage, v.UsedPercent))
		Colorize(Green, fmt.Sprintf("Total Swap: %.2f GB\n", float64(v.SwapTotal)/1024/1024/1024))
		Colorize(Green, fmt.Sprintf("Total Memory: %.2f GB\n", float64(v.Total)/1024/1024/1024))
		Colorize(Green, fmt.Sprintf("Memory Usage: %.2f%%\n", v.UsedPercent))
		if usedStorage > 90 {
			Colorize(Yellow, "Warning: Memory usage is above 90%\n")
		} else if usedStorage > 80 {
			Colorize(Yellow, "Warning: Memory usage is above 80%\n")
		} else {
			Colorize(Green, "Memory usage is within safe limits\n")
		}
	},
}
