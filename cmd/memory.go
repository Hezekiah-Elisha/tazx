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
		fmt.Printf("--- Memory Information ---\n")
		fmt.Printf("Total: %v GB, Free: %v GB, Used: %v GB, UsedPercent: %f%%\n", totalStorage, freeStorage, usedStorage, v.UsedPercent)
		fmt.Printf("Total Swap: %.2f GB\n", float64(v.SwapTotal)/1024/1024/1024)
		fmt.Printf("Total Memory: %.2f GB\n", float64(v.Total)/1024/1024/1024)
		fmt.Printf("Memory Usage: %.2f%%\n", v.UsedPercent)
		if usedStorage > 90 {
			fmt.Println("Warning: Memory usage is above 90%")
		} else if usedStorage > 80 {
			fmt.Println("Warning: Memory usage is above 80%")
		} else {
			fmt.Println("Memory usage is within safe limits")
		}
	},
}
