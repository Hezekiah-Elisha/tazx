package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/doctor"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common server, service, Docker, and log issues",
	Long:  `Run read-only diagnostic checks across CPU, memory, swap, disk partitions, systemd services, Docker containers, DNS, and access logs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		meta, err := GetAppConfigWithMeta()
		if err != nil {
			return err
		}

		cfgPath := ""
		if meta.LoadedFromFile {
			cfgPath = meta.Path
		}

		report, err := doctor.RunDiagnosticsWithProviders(ctx, meta.Config, cfgPath, doctor.DefaultProviders())
		if err != nil {
			return fmt.Errorf("error running diagnostics: %w", err)
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, report)
		}

		libs.Colorize(libs.Bold, "\n🩺 Running Tazx Doctor Diagnostics (Read-Only)...\n\n")

		if len(report.Issues) == 0 {
			libs.Colorize(libs.Green, "✅ All checks passed! No health issues, resource bottlenecks, or suspicious activity detected.\n\n")
			return nil
		}

		for _, issue := range report.Issues {
			headerColor := libs.Yellow
			if issue.Severity == doctor.SeverityCritical {
				headerColor = libs.Red
			} else if issue.Severity == doctor.SeverityInfo {
				headerColor = libs.Cyan
			}

			libs.Colorize(headerColor, fmt.Sprintf("%s [%s] %s\n", issue.Icon, issue.Severity, issue.Title))
			if issue.Impact != "" {
				fmt.Printf("  Why it matters:  %s\n", issue.Impact)
			}
			if issue.Explanation != "" {
				libs.Colorize(libs.Gray, fmt.Sprintf("  Details:         %s\n", issue.Explanation))
			}
			if issue.Recommendation != "" {
				libs.Colorize(libs.Blue, fmt.Sprintf("  Investigate:     %s\n", issue.Recommendation))
			}
			fmt.Println()
		}
		return nil
	},
}
