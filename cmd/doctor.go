package cmd

import (
	"fmt"

	"tazx/internal/doctor"
	"tazx/libs"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and fix common server issues",
	Long:  `Run a series of smart checks to diagnose server health, detect anomalies, bot traffic, and resource bottlenecks.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetAppConfig()
		libs.Colorize(libs.Bold, "\n🩺 Running Tazx Doctor Diagnostics...\n\n")

		report, err := doctor.RunDiagnostics(cfg)
		if err != nil {
			libs.Colorize(libs.Red, fmt.Sprintf("Error running diagnostics: %v\n", err))
			return
		}

		if len(report.Issues) == 0 {
			libs.Colorize(libs.Green, "✅ All systems normal! No health issues, spikes, or suspicious activity detected.\n\n")
			return
		}

		for _, issue := range report.Issues {
			headerColor := libs.Yellow
			if issue.Severity == doctor.SeverityCritical {
				headerColor = libs.Red
			} else if issue.Severity == doctor.SeverityInfo {
				headerColor = libs.Cyan
			}

			libs.Colorize(headerColor, fmt.Sprintf("%s %s\n", issue.Icon, issue.Title))
			if issue.Explanation != "" {
				libs.Colorize(libs.Gray, fmt.Sprintf("  %s\n", issue.Explanation))
			}
			if issue.Recommendation != "" {
				libs.Colorize(libs.Blue, fmt.Sprintf("  → Recommendation: %s\n", issue.Recommendation))
			}
			fmt.Println()
		}
	},
}
