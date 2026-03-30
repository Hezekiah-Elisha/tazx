package cmd

import "github.com/spf13/cobra"

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "View the top processes running on your server",
	Long:  `Get a real-time view of the top processes consuming resources on your server, helping you identify performance bottlenecks and manage your server effectively.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for top command logic
	},
}
