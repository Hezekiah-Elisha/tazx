package cmd

import (
	"fmt"

	"tazx/internal/docker"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var showAllContainers bool

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Check the status of your Docker containers",
	Long:  `Get a quick overview of your Docker containers' health, status, and resource usage.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDockerPs(showAllContainers)
	},
}

var dockerPsCmd = &cobra.Command{
	Use:   "ps",
	Short: "List Docker containers",
	Run: func(cmd *cobra.Command, args []string) {
		runDockerPs(showAllContainers)
	},
}

var dockerStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display container resource usage statistics",
	Run: func(cmd *cobra.Command, args []string) {
		runDockerPs(true)
	},
}

func runDockerPs(all bool) {
	libs.Colorize(libs.Bold, "\n🐳 Docker Containers Overview\n\n")

	containers, err := docker.GetContainers(all)
	if err != nil {
		libs.Colorize(libs.Yellow, "⚠️ Docker daemon is not running or not accessible.\n")
		libs.Colorize(libs.Gray, fmt.Sprintf("   Detail: %v\n\n", err))
		libs.Colorize(libs.Cyan, "💡 Tip: Ensure Docker engine is installed and started (e.g. 'sudo systemctl start docker').\n\n")
		return
	}

	if len(containers) == 0 {
		libs.Colorize(libs.Yellow, "No Docker containers found.\n\n")
		return
	}

	fmt.Printf("%-14s %-25s %-30s %-20s %s\n", "CONTAINER ID", "NAMES", "IMAGE", "STATUS", "PORTS")
	fmt.Println("---------------------------------------------------------------------------------------------------------")

	for _, c := range containers {
		statusColor := libs.Green
		if c.State != "running" {
			statusColor = libs.Yellow
		}

		statusStr := libs.SprintColor(statusColor, c.Status)
		fmt.Printf("%-14s %-25s %-30s %-20s %s\n", c.ID, trunc(c.Names, 25), trunc(c.Image, 30), statusStr, c.Ports)
	}
	fmt.Println()
}

func trunc(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func init() {
	dockerCmd.Flags().BoolVarP(&showAllContainers, "all", "a", true, "Show all containers (default shows all)")
	dockerPsCmd.Flags().BoolVarP(&showAllContainers, "all", "a", true, "Show all containers")
	dockerCmd.AddCommand(dockerPsCmd)
	dockerCmd.AddCommand(dockerStatsCmd)
}
