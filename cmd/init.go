package cmd

import (
	"fmt"
	"os"

	"tazx/internal/config"
	"tazx/internal/logs"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tazx configuration file and sample environment",
	Long:  `Create a default .tazx.yaml configuration file and prepare sample logs if necessary.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfgPath := GetAppConfig().LogPath
		_ = cfgPath

		targetConfigPath := config.GetConfigPath()
		cfg := config.DefaultConfig()

		err := config.SaveConfig(cfg, targetConfigPath)
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Failed to save config: %v\n", err))
			return
		}

		libs.Colorize(libs.Green, fmt.Sprintf("✅ Configuration initialized successfully at %s\n", targetConfigPath))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   log_path: %s\n", cfg.LogPath))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   refresh_rate: %ds\n\n", cfg.RefreshRate))

		// Ensure sample log file exists if possible
		err = logs.EnsureSampleLog(cfg.LogPath)
		if err == nil {
			libs.Colorize(libs.Green, fmt.Sprintf("✅ Server log ready at %s\n", cfg.LogPath))
		} else if _, statErr := os.Stat(cfg.LogPath); statErr != nil {
			libs.Colorize(libs.Yellow, fmt.Sprintf("ℹ️  Note: Log file not found at %s. Once Nginx is running, logs will be parsed automatically.\n", cfg.LogPath))
		}

		libs.Colorize(libs.Bold, "\n🚀 You can now run:\n")
		fmt.Println("  tazx status")
		fmt.Println("  tazx doctor")
		fmt.Println("  tazx logs")
		fmt.Println("  tazx top")
		fmt.Println()
	},
}
