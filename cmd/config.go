package cmd

import (
	"fmt"

	"tazx/internal/config"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage your server configuration",
	Long:  `View and manage your server's configuration settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, path, err := config.LoadConfig(cfgFile)
		libs.Colorize(libs.Bold, "\n⚙️  Tazx Configuration\n\n")

		if err != nil {
			libs.Colorize(libs.Yellow, fmt.Sprintf("Using default configuration (File at %s not found or invalid)\n\n", path))
		} else {
			libs.Colorize(libs.Green, fmt.Sprintf("Loaded from: %s\n\n", path))
		}

		fmt.Printf("  %-20s : %s\n", "Log Path", cfg.LogPath)
		fmt.Printf("  %-20s : %ds\n", "Refresh Rate", cfg.RefreshRate)
		fmt.Printf("  %-20s : %.1f%%\n", "CPU Threshold", cfg.CpuThreshold)
		fmt.Printf("  %-20s : %.1f%%\n", "RAM Threshold", cfg.MemoryThreshold)
		fmt.Printf("  %-20s : %.1f%%\n", "Disk Threshold", cfg.DiskThreshold)
		fmt.Println()
		libs.Colorize(libs.Cyan, "To re-initialize config file, run 'tazx init'.\n\n")
	},
}
