package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "View server RAM and swap memory usage",
	Long:  `View detailed memory metrics including physical RAM, available memory, swap utilization, and top memory-consuming processes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}
		cfg := meta.Config

		sysStatus, err := system.GetSystemStatus(ctx)
		if err != nil {
			return fmt.Errorf("error fetching memory stats: %w", err)
		}
		procs, procErr := system.GetTopProcesses(ctx, 5, true)

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				MemoryTotalGB     float64              `json:"memory_total_gb"`
				MemoryUsedGB      float64              `json:"memory_used_gb"`
				MemoryAvailableGB float64              `json:"memory_available_gb"`
				MemoryFreeGB      float64              `json:"memory_free_gb"`
				MemoryPercent     float64              `json:"memory_percent"`
				SwapAvailable     bool                 `json:"swap_available"`
				SwapTotalGB       float64              `json:"swap_total_gb"`
				SwapUsedGB        float64              `json:"swap_used_gb"`
				SwapFreeGB        float64              `json:"swap_free_gb"`
				SwapPercent       float64              `json:"swap_percent"`
				TopProcesses      []system.ProcessInfo `json:"top_processes,omitempty"`
			}{
				MemoryTotalGB:     sysStatus.MemoryTotalGB,
				MemoryUsedGB:      sysStatus.MemoryUsedGB,
				MemoryAvailableGB: sysStatus.MemoryAvailableGB,
				MemoryFreeGB:      sysStatus.MemoryFreeGB,
				MemoryPercent:     sysStatus.MemoryPercent,
				SwapAvailable:     sysStatus.SwapAvailable,
				SwapTotalGB:       sysStatus.SwapTotalGB,
				SwapUsedGB:        sysStatus.SwapUsedGB,
				SwapFreeGB:        sysStatus.SwapFreeGB,
				SwapPercent:       sysStatus.SwapPercent,
				TopProcesses:      procs,
			})
		}

		libs.Colorize(libs.Bold, "\n🧠 Server Memory Metrics\n")

		memColor := libs.Green
		if sysStatus.MemoryPercent >= 90.0 {
			memColor = libs.Red
		} else if sysStatus.MemoryPercent >= cfg.MemoryThreshold {
			memColor = libs.Yellow
		}

		libs.PrintSection(os.Stdout, "RAM USAGE")
		fmt.Printf("Total Memory     : %.2f GB (%s)\n", sysStatus.MemoryTotalGB, libs.FormatBytes(sysStatus.MemoryTotalBytes))
		fmt.Printf("Used Memory      : %.2f GB (%s)\n", sysStatus.MemoryUsedGB, libs.FormatBytes(sysStatus.MemoryUsedBytes))
		fmt.Printf("Available Memory : %.2f GB (%s)\n", sysStatus.MemoryAvailableGB, libs.FormatBytes(sysStatus.MemoryAvailableBytes))
		fmt.Printf("Free Memory      : %.2f GB\n", sysStatus.MemoryFreeGB)
		libs.Colorize(memColor, fmt.Sprintf("Memory Usage     : %.1f%% (Threshold: %.0f%%)\n", sysStatus.MemoryPercent, cfg.MemoryThreshold))

		if sysStatus.MemoryPercent >= 90.0 {
			libs.Colorize(libs.Red, "⚠️  Critical: Memory usage is critically high (>= 90%)\n")
		} else if sysStatus.MemoryPercent >= cfg.MemoryThreshold {
			libs.Colorize(libs.Yellow, fmt.Sprintf("⚠️  Warning: Memory usage exceeds threshold (>= %.0f%%)\n", cfg.MemoryThreshold))
		} else {
			libs.Colorize(libs.Green, "✅ Memory usage is within healthy limits\n")
		}

		libs.PrintSection(os.Stdout, "SWAP USAGE")
		if !sysStatus.SwapAvailable {
			libs.Colorize(libs.Gray, "Swap metrics unavailable on this host.\n")
		} else if sysStatus.SwapTotalBytes == 0 {
			fmt.Println("No swap space configured (0 B).")
		} else {
			swapColor := libs.Green
			if sysStatus.SwapPercent >= 90.0 {
				swapColor = libs.Red
			} else if sysStatus.SwapPercent >= 80.0 {
				swapColor = libs.Yellow
			}
			fmt.Printf("Total Swap       : %.2f GB (%s)\n", sysStatus.SwapTotalGB, libs.FormatBytes(sysStatus.SwapTotalBytes))
			fmt.Printf("Used Swap        : %.2f GB (%s)\n", sysStatus.SwapUsedGB, libs.FormatBytes(sysStatus.SwapUsedBytes))
			fmt.Printf("Free Swap        : %.2f GB\n", sysStatus.SwapFreeGB)
			libs.Colorize(swapColor, fmt.Sprintf("Swap Usage       : %.1f%%\n", sysStatus.SwapPercent))
		}

		if procErr == nil && len(procs) > 0 {
			libs.PrintSection(os.Stdout, "TOP MEMORY CONSUMERS")
			fmt.Printf("%-8s %-24s %-15s %s\n", "PID", "COMMAND", "USER", "RAM %")
			fmt.Println("---------------------------------------------------------")
			for _, p := range procs {
				fmt.Printf("%-8d %-24s %-15s %.1f%%\n",
					p.PID,
					libs.Truncate(libs.SanitizeTerminal(p.Name), 24),
					libs.Truncate(libs.SanitizeTerminal(p.Username), 15),
					p.MemPercent)
			}
		}
		fmt.Println()
		return nil
	},
}
