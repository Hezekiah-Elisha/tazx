package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	logPathFlag string
	noColorFlag bool
	jsonFlag    bool
)

// AppConfigMeta holds resolved configuration alongside load metadata.
type AppConfigMeta struct {
	Config         config.Config
	Path           string
	LoadedFromFile bool
	LoadErr        error
}

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
	Short: "Tazama your server's pulse instantly",
	Long:  `Tazx is a lightweight CLI tool that helps developers and sysadmins inspect server vitals, analyze logs, check services/ports/network, and inspect Docker containers.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Suppress usage dump on runtime errors after Cobra finishes flag parsing.
		cmd.SilenceUsage = true
		libs.SetNoColor(noColorFlag)

		if !jsonFlag && libs.IsTerminal(os.Stdout) && !isLiveCommand(cmd) {
			libs.Colorize(libs.Cyan, banner)
		}
	},
}

func isLiveCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	switch cmd.Name() {
	case "top", "monitor", "watch", "help", "completion":
		return true
	}
	if cmd.Parent() != nil && cmd.Parent().Name() == "watch" {
		return true
	}
	return false
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// GetAppConfigWithMeta loads configuration from --config or default locations.
func GetAppConfigWithMeta() (AppConfigMeta, error) {
	cfg, path, err := config.LoadConfig(cfgFile)
	meta := AppConfigMeta{
		Config:  cfg,
		Path:    path,
		LoadErr: err,
	}

	if err != nil {
		if cfgFile != "" {
			return meta, fmt.Errorf("failed to load config file %q: %w", cfgFile, err)
		}
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Warning: invalid config at %s (%v); using defaults.\n", path, err)
		}
	} else {
		meta.LoadedFromFile = true
	}

	if logPathFlag != "" {
		meta.Config.LogPath = logPathFlag
	}
	return meta, nil
}

// GetAppConfig returns the resolved configuration or defaults on non-explicit errors.
func GetAppConfig() config.Config {
	meta, err := GetAppConfigWithMeta()
	if err != nil {
		cfg := config.DefaultConfig()
		if logPathFlag != "" {
			cfg.LogPath = logPathFlag
		}
		return cfg
	}
	return meta.Config
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tazx.yaml)")
	rootCmd.PersistentFlags().StringVarP(&logPathFlag, "path", "p", "", "path to server log file")
	rootCmd.PersistentFlags().BoolVar(&noColorFlag, "no-color", false, "disable ANSI color output")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output machine-readable JSON where supported")

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
	rootCmd.AddCommand(portsCmd)
	rootCmd.AddCommand(servicesCmd)
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(updateCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
