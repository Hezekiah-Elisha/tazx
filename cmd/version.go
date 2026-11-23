package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

// AppVersion can be overridden at build time via -ldflags "-X github.com/Hezekiah-Elisha/tazx/cmd.AppVersion=...".
var AppVersion = "v1.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Tazx version and system environment",
	Long:  `Display the current version of Tazx, Go runtime version, and OS target.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				Version   string `json:"version"`
				GoVersion string `json:"go_version"`
				OS        string `json:"os"`
				Arch      string `json:"arch"`
			}{
				Version:   AppVersion,
				GoVersion: runtime.Version(),
				OS:        runtime.GOOS,
				Arch:      runtime.GOARCH,
			})
		}

		libs.Colorize(libs.Bold, "\n⚡ Tazx Server Pulse CLI\n")
		libs.Colorize(libs.Green, fmt.Sprintf("Version:    %s\n", AppVersion))
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch:    %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
		return nil
	},
}
