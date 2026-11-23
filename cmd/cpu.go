package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var cpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "View server CPU utilization, load averages, and core details",
	Long:  `View server CPU usage percentage, per-core utilization, load averages, hardware model info, and top CPU processes.`,
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
			return fmt.Errorf("error fetching CPU metrics: %w", err)
		}
		if perCore, err := system.GetPerCoreCPUUsage(ctx); err == nil {
			sysStatus.PerCorePercent = perCore
		}
		procs, procErr := system.GetTopProcesses(ctx, 5, false)

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				CPUPercent       float64              `json:"cpu_percent"`
				PerCorePercent   []float64            `json:"per_core_percent,omitempty"`
				PhysicalCores    int                  `json:"physical_cores"`
				LogicalCores     int                  `json:"logical_cores"`
				LoadAvg1         float64              `json:"load_avg_1"`
				LoadAvg5         float64              `json:"load_avg_5"`
				LoadAvg15        float64              `json:"load_avg_15"`
				LoadAvgAvailable bool                 `json:"load_avg_available"`
				CPUInfo          []system.CPUCoreInfo `json:"cpu_info,omitempty"`
				TopProcesses     []system.ProcessInfo `json:"top_processes,omitempty"`
				Warnings         []string             `json:"warnings,omitempty"`
			}{
				CPUPercent:       sysStatus.CPUPercent,
				PerCorePercent:   sysStatus.PerCorePercent,
				PhysicalCores:    sysStatus.PhysicalCores,
				LogicalCores:     sysStatus.LogicalCores,
				LoadAvg1:         sysStatus.LoadAvg1,
				LoadAvg5:         sysStatus.LoadAvg5,
				LoadAvg15:        sysStatus.LoadAvg15,
				LoadAvgAvailable: sysStatus.LoadAvgAvailable,
				CPUInfo:          sysStatus.CPUInfo,
				TopProcesses:     procs,
				Warnings:         sysStatus.Warnings,
			})
		}

		libs.Colorize(libs.Bold, "\n💻 CPU Utilization & Hardware Specs\n\n")

		color := libs.Green
		if sysStatus.CPUPercent >= 90.0 {
			color = libs.Red
		} else if sysStatus.CPUPercent >= cfg.CpuThreshold {
			color = libs.Yellow
		}

		libs.Colorize(color, fmt.Sprintf("Overall CPU Usage: %.1f%% (Threshold: %.0f%%)\n", sysStatus.CPUPercent, cfg.CpuThreshold))
		if sysStatus.PhysicalCores > 0 || sysStatus.LogicalCores > 0 {
			fmt.Printf("Cores:             %d physical / %d logical\n", sysStatus.PhysicalCores, sysStatus.LogicalCores)
		}
		if sysStatus.LoadAvgAvailable {
			fmt.Printf("Load Average:      %.2f (1m), %.2f (5m), %.2f (15m)\n", sysStatus.LoadAvg1, sysStatus.LoadAvg5, sysStatus.LoadAvg15)
		}

		if len(sysStatus.CPUInfo) > 0 {
			libs.PrintSection(os.Stdout, "HARDWARE")
			seenModels := make(map[string]int)
			for _, info := range sysStatus.CPUInfo {
				key := fmt.Sprintf("%s|%.0f", info.ModelName, info.Mhz)
				seenModels[key]++
			}
			printed := make(map[string]bool)
			for _, info := range sysStatus.CPUInfo {
				key := fmt.Sprintf("%s|%.0f", info.ModelName, info.Mhz)
				if printed[key] {
					continue
				}
				printed[key] = true
				fmt.Printf("  Model: %s | Speed: %.0f MHz | Entries: %d\n",
					libs.SanitizeTerminal(info.ModelName), info.Mhz, seenModels[key])
			}
		}

		if len(sysStatus.PerCorePercent) > 0 {
			libs.PrintSection(os.Stdout, "PER-CORE USAGE")
			for i, pct := range sysStatus.PerCorePercent {
				cColor := libs.Green
				if pct >= 90.0 {
					cColor = libs.Red
				} else if pct >= cfg.CpuThreshold {
					cColor = libs.Yellow
				}
				fmt.Printf("  Core #%-2d: %s\n", i, libs.SprintColor(cColor, fmt.Sprintf("%5.1f%%", pct)))
			}
		}

		if procErr == nil && len(procs) > 0 {
			libs.PrintSection(os.Stdout, "TOP CPU PROCESSES")
			fmt.Printf("%-8s %-24s %-15s %s\n", "PID", "COMMAND", "USER", "CPU %")
			fmt.Println("---------------------------------------------------------")
			for _, p := range procs {
				fmt.Printf("%-8d %-24s %-15s %.1f%%\n",
					p.PID,
					libs.Truncate(libs.SanitizeTerminal(p.Name), 24),
					libs.Truncate(libs.SanitizeTerminal(p.Username), 15),
					p.CPUPercent)
			}
		}
		fmt.Println()
		return nil
	},
}
