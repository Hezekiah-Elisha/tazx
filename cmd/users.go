package cmd

import (
	"fmt"
	"time"

	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage and view logged-in server users",
	Long:  `View active user accounts and terminal sessions connected to this server.`,
	Run: func(cmd *cobra.Command, args []string) {
		sysStatus, err := system.GetSystemStatus()
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error fetching active user sessions: %v\n", err))
			return
		}

		libs.Colorize(libs.Bold, "\n👥 Active System Users & Sessions\n\n")

		if len(sysStatus.ActiveUsers) == 0 {
			libs.Colorize(libs.Yellow, "No active interactive login sessions detected.\n\n")
			return
		}

		fmt.Printf("%-15s %-15s %-25s %s\n", "USER", "TERMINAL", "HOST / IP", "LOGIN TIME")
		fmt.Println("-------------------------------------------------------------------------")

		for _, u := range sysStatus.ActiveUsers {
			loginTime := time.Unix(int64(u.Started), 0).Format("2006-01-02 15:04:05")
			hostStr := u.Host
			if hostStr == "" {
				hostStr = "local / console"
			}

			fmt.Printf("%-15s %-15s %-25s %s\n", u.User, u.Terminal, hostStr, loginTime)
		}
		fmt.Println()
	},
}
