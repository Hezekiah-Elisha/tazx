package cmd

import (
	"flag"
	"fmt"

	"github.com/shirou/gopsutil/cpu"
	"github.com/spf13/cobra"
)

type Color string

const (
	Red    Color = "\033[31m"
	Green  Color = "\033[32m"
	Yellow Color = "\033[33m"
	Blue   Color = "\033[34m"
	Reset  Color = "\033[0m"
)

var cpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "Manage your server CPU",
	Long:  `View and manage your server's CPU usage, including load averages and core utilization.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for CPU command logic
		c, _ := cpu.Percent(0, false)
		fmt.Printf("CPU: %v%%\n", c)
		Colorize(Green, fmt.Sprintf("CPU: %v%%", c))

	},
}

func cpuFunc() {
	// Define flags
	cpuFlag := flag.Bool("cpu", false, "Get CPU stats")

	// Parse flags
	flag.Parse()

	// Use flags to control gopsutil
	if *cpuFlag {
		c, _ := cpu.Percent(0, false)
		Colorize(Green, fmt.Sprintf("CPU: %v%%", c))
	}
}

func Colorize(color Color, text string) {
	fmt.Printf("%s%s%s", color, text, Reset)
}
