package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create an optional default Tazx configuration file",
	Long:  `Create a default .tazx.yaml configuration file with owner-only (0600) permissions. Tazx works out of the box without running init.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetConfigPath := cfgFile
		if targetConfigPath == "" {
			targetConfigPath = config.GetConfigPath()
		}

		if config.ConfigExists(targetConfigPath) && !forceInit {
			return fmt.Errorf("configuration file already exists at %s (pass --force to overwrite)", targetConfigPath)
		}

		cfg := config.DefaultConfig()
		if logPathFlag != "" {
			cfg.LogPath = logPathFlag
		}

		if err := config.SaveConfig(cfg, targetConfigPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				Path   string        `json:"path"`
				Config config.Config `json:"config"`
			}{
				Path:   targetConfigPath,
				Config: cfg,
			})
		}

		libs.Colorize(libs.Green, fmt.Sprintf("\n✅ Configuration written to %s (mode 0600)\n", targetConfigPath))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   log_path:         %s\n", cfg.LogPath))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   refresh_rate:     %ds\n", cfg.RefreshRate))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   cpu_threshold:    %.0f%%\n", cfg.CpuThreshold))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   memory_threshold: %.0f%%\n", cfg.MemoryThreshold))
		libs.Colorize(libs.Cyan, fmt.Sprintf("   disk_threshold:   %.0f%%\n\n", cfg.DiskThreshold))

		if _, statErr := os.Stat(cfg.LogPath); statErr != nil {
			libs.Colorize(libs.Yellow, fmt.Sprintf("ℹ️  Note: No log file found at %s. Point log_path in %s or pass --path when analyzing logs.\n", cfg.LogPath, targetConfigPath))
		}

		libs.Colorize(libs.Bold, "\n🚀 Next steps:\n")
		fmt.Println("  tazx status")
		fmt.Println("  tazx doctor")
		fmt.Println("  tazx ports")
		fmt.Println("  tazx top")
		fmt.Println()
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "overwrite existing configuration file")
}
