package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View active server configuration",
	Long:  `Display active thresholds, log path, refresh interval, and configuration source.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}
		cfg := meta.Config

		var permWarning string
		if meta.LoadedFromFile {
			if info, statErr := os.Stat(meta.Path); statErr == nil && info.Mode().Perm()&0077 != 0 {
				permWarning = fmt.Sprintf("File %s has mode %04o; consider running 'chmod 600 %s'.", meta.Path, info.Mode().Perm(), meta.Path)
			}
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				Config         config.Config `json:"config"`
				Path           string        `json:"path"`
				LoadedFromFile bool          `json:"loaded_from_file"`
				Warning        string        `json:"warning,omitempty"`
			}{
				Config:         cfg,
				Path:           meta.Path,
				LoadedFromFile: meta.LoadedFromFile,
				Warning:        permWarning,
			})
		}

		libs.Colorize(libs.Bold, "\n⚙️  Tazx Configuration\n\n")
		if meta.LoadedFromFile {
			libs.Colorize(libs.Green, fmt.Sprintf("Loaded from: %s\n\n", meta.Path))
		} else {
			libs.Colorize(libs.Yellow, fmt.Sprintf("Using default configuration (no config file at %s)\n\n", meta.Path))
		}

		fmt.Printf("  %-20s : %s\n", "Log Path", libs.SanitizeTerminal(cfg.LogPath))
		fmt.Printf("  %-20s : %ds\n", "Refresh Rate", cfg.RefreshRate)
		fmt.Printf("  %-20s : %.1f%%\n", "CPU Threshold", cfg.CpuThreshold)
		fmt.Printf("  %-20s : %.1f%%\n", "RAM Threshold", cfg.MemoryThreshold)
		fmt.Printf("  %-20s : %.1f%%\n", "Disk Threshold", cfg.DiskThreshold)

		if permWarning != "" {
			fmt.Println()
			libs.Colorize(libs.Yellow, fmt.Sprintf("⚠️  %s\n", permWarning))
		}

		fmt.Println()
		libs.Colorize(libs.Cyan, "Note: Configuration is optional. To create a default file, run 'tazx init'.\n\n")
		return nil
	},
}
