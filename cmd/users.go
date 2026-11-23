package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "View active logged-in server user sessions",
	Long:  `View active interactive user accounts and terminal sessions connected to this server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		sysStatus, err := system.GetSystemStatus(ctx)
		if err != nil {
			return fmt.Errorf("error fetching active user sessions: %w", err)
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				Users    []system.UserSession `json:"users"`
				Warnings []string             `json:"warnings,omitempty"`
			}{
				Users:    sysStatus.ActiveUsers,
				Warnings: sysStatus.Warnings,
			})
		}

		libs.Colorize(libs.Bold, "\n👥 Active System Users & Sessions\n\n")

		if len(sysStatus.ActiveUsers) == 0 {
			libs.Colorize(libs.Yellow, "No active interactive login sessions detected.\n\n")
			return nil
		}

		fmt.Printf("%-16s %-16s %-26s %s\n", "USER", "TERMINAL", "HOST / IP", "LOGIN TIME")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, u := range sysStatus.ActiveUsers {
			loginTime := "-"
			if u.Started > 0 {
				loginTime = time.Unix(u.Started, 0).Format("2006-01-02 15:04:05")
			}
			hostStr := u.Host
			if hostStr == "" {
				hostStr = "local / console"
			}

			fmt.Printf("%-16s %-16s %-26s %s\n",
				libs.Truncate(libs.SanitizeTerminal(u.User), 16),
				libs.Truncate(libs.SanitizeTerminal(u.Terminal), 16),
				libs.Truncate(libs.SanitizeTerminal(hostStr), 26),
				loginTime,
			)
		}
		fmt.Println()
		return nil
	},
}
