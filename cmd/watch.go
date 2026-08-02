package cmd

import (
	"fmt"
	"time"

	"tazx/internal/docker"
	"tazx/internal/system"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [resource]",
	Short: "Watch a resource in real time (cpu, logs, docker)",
	Long:  `Continuously monitor a server resource like CPU usage, logs, or Docker stats in real time.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			libs.Colorize(libs.Yellow, "Please specify a resource to watch: cpu, logs, or docker\nExample: tazx watch cpu\n")
			return
		}

		target := args[0]
		switch target {
		case "cpu":
			watchCPU()
		case "logs":
			cfg := GetAppConfig()
			followLogFile(cfg.LogPath, false)
		case "docker":
			watchDocker()
		default:
			libs.Colorize(libs.Red, fmt.Sprintf("Unknown resource '%s'. Available: cpu, logs, docker\n", target))
		}
	},
}

var watchCpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "Watch CPU utilization in real time",
	Run: func(cmd *cobra.Command, args []string) {
		watchCPU()
	},
}

var watchLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Watch server log stream in real time",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetAppConfig()
		followLogFile(cfg.LogPath, false)
	},
}

var watchDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Watch Docker containers in real time",
	Run: func(cmd *cobra.Command, args []string) {
		watchDocker()
	},
}

func watchCPU() {
	for {
		clearScreen()
		sysStatus, err := system.GetSystemStatus()
		libs.Colorize(libs.Bold, "⚡ TAZX WATCH - Real-Time CPU Monitor\n\n")

		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error: %v\n", err))
		} else {
			cpuColor := libs.Green
			if sysStatus.CPUPercent > 80 {
				cpuColor = libs.Red
			} else if sysStatus.CPUPercent > 50 {
				cpuColor = libs.Yellow
			}
			libs.Colorize(cpuColor, fmt.Sprintf("CPU Load: %.1f%%\n\n", sysStatus.CPUPercent))

			procs, _ := system.GetTopProcesses(8, false)
			fmt.Printf("%-8s %-20s %s\n", "PID", "NAME", "CPU %")
			fmt.Println("---------------------------------")
			for _, p := range procs {
				fmt.Printf("%-8d %-20s %.1f%%\n", p.PID, trunc(p.Name, 20), p.CPUPercent)
			}
		}

		libs.Colorize(libs.Gray, "\nPress Ctrl+C to stop watching...\n")
		time.Sleep(2 * time.Second)
	}
}

func watchDocker() {
	for {
		clearScreen()
		libs.Colorize(libs.Bold, "🐳 TAZX WATCH - Docker Containers\n\n")
		containers, err := docker.GetContainers(true)
		if err != nil {
			libs.Colorize(libs.Yellow, "Docker daemon not reachable.\n")
		} else if len(containers) == 0 {
			libs.Colorize(libs.Yellow, "No Docker containers found.\n")
		} else {
			fmt.Printf("%-14s %-25s %-25s %s\n", "ID", "NAMES", "IMAGE", "STATUS")
			fmt.Println("----------------------------------------------------------------------")
			for _, c := range containers {
				fmt.Printf("%-14s %-25s %-25s %s\n", c.ID, trunc(c.Names, 25), trunc(c.Image, 25), c.Status)
			}
		}

		libs.Colorize(libs.Gray, "\nPress Ctrl+C to stop watching...\n")
		time.Sleep(3 * time.Second)
	}
}

func init() {
	watchCmd.AddCommand(watchCpuCmd, watchLogsCmd, watchDockerCmd)
}
