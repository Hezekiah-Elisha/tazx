package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/updater"
	"github.com/Hezekiah-Elisha/tazx/libs"
	"github.com/spf13/cobra"
)

var (
	updateCheckOnly bool
	updateVersion   string
	updateForce     bool
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   "Check for updates or update Tazx to the latest release",
	Long:    `Check GitHub Releases for a newer version of Tazx, verify its SHA-256 checksum over HTTPS, and replace the installed binary atomically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		client := updater.NewClient()

		if updateCheckOnly {
			res, err := client.CheckForUpdate(ctx, AppVersion, updateVersion, "")
			if err != nil {
				return err
			}
			if jsonFlag {
				return libs.PrintJSON(os.Stdout, res)
			}

			libs.Colorize(libs.Bold, "\n🔄 Tazx Update Check\n\n")
			fmt.Printf("  %-18s : %s\n", "Current Version", res.CurrentVersion)
			fmt.Printf("  %-18s : %s\n", "Target Version", res.LatestVersion)
			fmt.Printf("  %-18s : %s\n", "Executable", res.ExecutablePath)
			if res.UpdateAvailable {
				libs.Colorize(libs.Green, fmt.Sprintf("\n⬆️  Update available: %s -> %s\n", res.CurrentVersion, res.LatestVersion))
				if res.ReleaseURL != "" {
					fmt.Printf("   Release: %s\n", res.ReleaseURL)
				}
				fmt.Println("   Run 'tazx update' to install it.")
			} else {
				libs.Colorize(libs.Green, "\n✅ Tazx is up to date.\n")
			}
			fmt.Println()
			return nil
		}

		res, err := client.Update(ctx, AppVersion, "", updater.Options{
			TargetVersion: updateVersion,
			Force:         updateForce,
		})
		if err != nil {
			return err
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, res)
		}

		libs.Colorize(libs.Bold, "\n🔄 Tazx Update\n\n")
		if res.Updated {
			libs.Colorize(libs.Green, fmt.Sprintf("✅ %s\n", res.Message))
			fmt.Printf("   Binary   : %s\n", res.ExecutablePath)
			fmt.Printf("   SHA-256  : %s\n", res.Checksum)
		} else {
			libs.Colorize(libs.Green, fmt.Sprintf("✅ %s\n", res.Message))
			fmt.Printf("   Binary   : %s\n", res.ExecutablePath)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "check if a newer release is available without modifying the binary")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "install a specific release tag (for example: v1.1.0)")
	updateCmd.Flags().BoolVarP(&updateForce, "force", "f", false, "reinstall even if already on the target version")
}
