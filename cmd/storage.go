package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/system"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	storageShowAll   bool
	storageFindLarge bool
	storageScanDir   string
	storageDepth     int
	storageLimit     int
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "View server disk partitions and storage usage",
	Long:  `View capacity, filesystem types, mount points, and usage percentages across mounted partitions. Optionally identify large directories with --large.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}
		cfg := meta.Config

		partitions, warnings, err := system.GetDiskPartitions(ctx, storageShowAll, cfg.DiskThreshold)
		if err != nil {
			return fmt.Errorf("error fetching storage metrics: %w", err)
		}

		var largeDirs []system.DirUsage
		if storageFindLarge {
			dirs, dirWarnings, dirErr := system.FindLargeDirs(ctx, storageScanDir, storageDepth, storageLimit)
			if dirErr != nil {
				return fmt.Errorf("error scanning directories under %s: %w", storageScanDir, dirErr)
			}
			largeDirs = dirs
			warnings = append(warnings, dirWarnings...)
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, struct {
				Partitions []system.DiskPartition `json:"partitions"`
				LargeDirs  []system.DirUsage      `json:"large_dirs,omitempty"`
				Warnings   []string               `json:"warnings,omitempty"`
			}{
				Partitions: partitions,
				LargeDirs:  largeDirs,
				Warnings:   warnings,
			})
		}

		libs.Colorize(libs.Bold, "\n💾 Disk Storage & Filesystems\n")
		libs.PrintSection(os.Stdout, "PARTITIONS")

		if len(partitions) == 0 {
			libs.Colorize(libs.Yellow, "No matching filesystems found.\n")
		} else {
			fmt.Printf("%-22s %-8s %-20s %-10s %-10s %-10s %-8s %s\n",
				"FILESYSTEM", "TYPE", "MOUNT", "TOTAL", "USED", "AVAIL", "USE%", "STATUS")
			fmt.Println("----------------------------------------------------------------------------------------------------")

			for _, p := range partitions {
				statusColor := libs.Green
				if p.Status == "CRITICAL" {
					statusColor = libs.Red
				} else if p.Status == "WARNING" {
					statusColor = libs.Yellow
				}

				fmt.Printf("%s %s %s %s %s %s %s %s\n",
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(p.Filesystem), 22), 22),
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(p.FSType), 8), 8),
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(p.Mountpoint), 20), 20),
					libs.PadRight(libs.FormatBytes(p.TotalBytes), 10),
					libs.PadRight(libs.FormatBytes(p.UsedBytes), 10),
					libs.PadRight(libs.FormatBytes(p.AvailableBytes), 10),
					libs.PadRight(fmt.Sprintf("%.1f%%", p.UsedPercent), 8),
					libs.SprintColor(statusColor, p.Status),
				)
			}
		}

		if storageFindLarge {
			libs.PrintSection(os.Stdout, fmt.Sprintf("LARGEST DIRECTORIES (%s, depth %d)", libs.SanitizeTerminal(storageScanDir), storageDepth))
			if len(largeDirs) == 0 {
				fmt.Println("No subdirectories found.")
			} else {
				fmt.Printf("%-12s %s\n", "SIZE", "DIRECTORY")
				fmt.Println("------------------------------------------------------------")
				for _, d := range largeDirs {
					fmt.Printf("%-12s %s\n", libs.FormatBytes(d.SizeBytes), libs.SanitizeTerminal(d.Path))
				}
			}
		}

		for _, w := range warnings {
			libs.Colorize(libs.Yellow, fmt.Sprintf("⚠️  %s\n", libs.SanitizeTerminal(w)))
		}
		fmt.Println()
		return nil
	},
}

func init() {
	storageCmd.Flags().BoolVarP(&storageShowAll, "all", "a", false, "include virtual and pseudo filesystems")
	storageCmd.Flags().BoolVar(&storageFindLarge, "large", false, "scan for largest subdirectories (bounded)")
	storageCmd.Flags().StringVar(&storageScanDir, "dir", "/", "root directory to scan when --large is set")
	storageCmd.Flags().IntVar(&storageDepth, "depth", 2, "maximum directory depth when --large is set (1-5)")
	storageCmd.Flags().IntVar(&storageLimit, "limit", 10, "maximum directories to display when --large is set")
}
