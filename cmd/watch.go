package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch a resource in real time",
	Long:  `Continuously monitor a resource like CPU usage, logs, or Docker stats in real time.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Please specify a resource to watch (cpu, logs, docker)")
		watchDocker()
	},
}

// var cpuCmd = &cobra.Command{
// 	Use:   "cpu",
// 	Short: "Watch CPU usage",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		watchCPU()
// 	},
// }

// var logsCmd = &cobra.Command{
// 	Use:   "logs",
// 	Short: "Watch logs in real time",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		watchLogs()
// 	},
// }

// var dockerCmd = &cobra.Command{
// 	Use:   "docker",
// 	Short: "Watch docker stats",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		watchDocker()
// 	},
// }

// func init() {
// 	watchCmd.AddCommand(cpuCmd, logsCmd, dockerCmd)
// 	rootCmd.AddCommand(watchCmd)
// }

func watchDocker() {
	for {
		clearScreen()

		fmt.Println("=== DOCKER STATS ===")

		// pseudo (replace with actual Docker SDK later)
		fmt.Println("Container: api-server | CPU: 12% | MEM: 120MB")

		fmt.Println("\nPress Ctrl+C to stop watching...")

		time.Sleep(5 * time.Second)
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
