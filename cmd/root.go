package cmd

import (
	"tazx/internal/config"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	logPathFlag string
)

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
	Long:  `Tazx is a lightweight CLI tool that helps developers monitor server health, analyze logs, and detect issues — all from the terminal.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		libs.Colorize(libs.Cyan, banner)
	},
}

func GetAppConfig() config.Config {
	cfg, _, err := config.LoadConfig(cfgFile)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	if logPathFlag != "" {
		cfg.LogPath = logPathFlag
	}
	return cfg
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tazx.yaml)")
	rootCmd.PersistentFlags().StringVarP(&logPathFlag, "path", "p", "", "path to server log file")

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
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(topCmd)
	rootCmd.AddCommand(usersCmd)
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
