package cmd

import (
	"fmt"

	"github.com/shirou/gopsutil/host"
	"github.com/spf13/cobra"
)

type UserStat struct {
	User     string `json:"user"`
	Terminal string `json:"terminal"`
	Host     string `json:"host"`
	Started  int    `json:"started"`
}

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage your server users",
	Long:  `View and manage your server's user accounts, including active sessions and user information.`,
	Run: func(cmd *cobra.Command, args []string) {
		u, err := host.Users()
		if err != nil {
			Colorize(Red, fmt.Sprintf("Error fetching user info: %v\n", err))
			return
		}
		Colorize(Blue, "--- Active Users ---\n")
		for _, user := range u {
			Colorize(Green, fmt.Sprintf("User: %s, Terminal: %s, Host: %s, Started: %d\n", user.User, user.Terminal, user.Host, user.Started))
		}
	},
}
