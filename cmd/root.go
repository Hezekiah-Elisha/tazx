package cmd

import (
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "tazx",
	Short: "Tazama your server’s pulse instantly",
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(dockerCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	// Add the persistent --config flag to the root command.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.myapp.yaml)")
	cobra.CheckErr(rootCmd.Execute())
}
