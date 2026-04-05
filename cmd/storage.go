package cmd

import (
	"fmt"

	"github.com/shirou/gopsutil/disk"
	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage your server storage",
	Long:  `View and manage your server's storage usage, including disk space and partitions.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for storage command logic
		d, _ := disk.Usage("/")
		totalStorage := d.Total / (1024 * 1024 * 1024) // Convert bytes to GB
		freeStorage := d.Free / (1024 * 1024 * 1024)
		usedStorage := totalStorage - freeStorage

		Colorize(Blue, "--- Storage Information ---\n")
		Colorize(Red, fmt.Sprintf("Total: %v GB, Free: %v GB, Used: %v GB, UsedPercent: %f%%\n", totalStorage, freeStorage, usedStorage, d.UsedPercent))
		Colorize(Green, fmt.Sprintf("Total Storage: %.2f GB\n", float64(d.Total)/1024/1024/1024))
		Colorize(Green, fmt.Sprintf("Used Storage: %.2f GB\n", float64(d.Used)/1024/1024/1024))
		Colorize(Green, fmt.Sprintf("Free Storage: %.2f GB\n", float64(d.Free)/1024/1024/1024))
		Colorize(Green, fmt.Sprintf("Storage Usage: %.2f%%\n", d.UsedPercent))

		if usedStorage > 90 {
			Colorize(Yellow, "Warning: Storage usage is above 90%\n")
		} else if usedStorage > 80 {
			Colorize(Yellow, "Warning: Storage usage is above 80%\n")
		} else {
			Colorize(Green, "Storage usage is within safe limits\n")
		}
	},
}
