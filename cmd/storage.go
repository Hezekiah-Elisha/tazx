package cmd

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage your server storage",
	Long:  `View and manage your server's storage usage, including disk space and storage devices.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for storage command logic
		v, _ := mem.VirtualMemory()
		totalStorage := v.Total / (1024 * 1024 * 1024) // Convert bytes to GB
		freeStorage := v.Free / (1024 * 1024 * 1024)
		usedStorage := totalStorage - freeStorage
		fmt.Printf("Total: %v GB, Free: %v GB, Used: %v GB, UsedPercent: %f%%\n", totalStorage, freeStorage, usedStorage, v.UsedPercent)

		if usedStorage > 90 {
			fmt.Println("Warning: Storage usage is above 90%")
		} else if usedStorage > 80 {
			fmt.Println("Warning: Storage usage is above 80%")
		} else {
			fmt.Println("Storage usage is within safe limits")
		}
	},
}
