package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cfgFile string

const banner = `
████████╗ █████╗ ███████╗██╗  ██╗
╚══██╔══╝██╔══██╗╚══███╔╝╚██╗██╔╝
   ██║   ███████║  ███╔╝  ╚███╔╝
   ██║   ██╔══██║ ███╔╝   ██╔██╗
   ██║   ██║  ██║███████╗██╔╝ ██╗
   ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝
`

var rootCmd = &cobra.Command{
	Use:   "tazx",
	Short: "Tazama your server’s pulse instantly",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Print(banner)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(dockerCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cpuCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(topCmd)
	rootCmd.AddCommand(usersCmd)
}

func Execute() {
	// Add the persistent --config flag to the root command.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.myapp.yaml)")
	cobra.CheckErr(rootCmd.Execute())
}
