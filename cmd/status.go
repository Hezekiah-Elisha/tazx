package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of your server",
	Long:  `Get a quick overview of your server's health and performance.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for status command logic
		fmt.Printf("heyy")
	},
}
