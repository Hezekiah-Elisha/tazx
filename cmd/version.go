package cmd

import (
	"fmt"
	"runtime"

	"tazx/libs"

	"github.com/spf13/cobra"
)

const AppVersion = "v1.0.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Tazx version and system environment",
	Long:  `Display the current version of Tazx, Go runtime version, and OS target.`,
	Run: func(cmd *cobra.Command, args []string) {
		libs.Colorize(libs.Bold, "\n⚡ Tazx Server Pulse CLI\n")
		libs.Colorize(libs.Green, fmt.Sprintf("Version: %s\n", AppVersion))
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
	},
}
