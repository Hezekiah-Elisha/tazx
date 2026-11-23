package cmd

import (
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/ports"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	filterPort  int
	filterProto string
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Inspect listening network ports and owning processes",
	Long:  `Display active listening TCP and bound UDP sockets along with protocol, listening address, PID, and process name.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		report, err := ports.GetListeningPorts(ctx, filterPort, filterProto)
		if err != nil {
			return err
		}

		if jsonFlag {
			if report.Ports == nil {
				report.Ports = []ports.ListeningPort{}
			}
			return libs.PrintJSON(os.Stdout, report)
		}

		libs.Colorize(libs.Bold, "\n🔌 Listening Ports\n\n")

		if len(report.Ports) == 0 {
			if filterPort > 0 {
				libs.Colorize(libs.Yellow, fmt.Sprintf("No listening ports found on port %d.\n\n", filterPort))
			} else {
				libs.Colorize(libs.Yellow, "No listening ports found.\n\n")
			}
			return nil
		}

		fmt.Printf("%-8s %-8s %-26s %-8s %s\n", "PORT", "PROTO", "ADDRESS", "PID", "PROCESS")
		fmt.Println("------------------------------------------------------------------------")

		for _, p := range report.Ports {
			pidStr := "-"
			if p.PID > 0 {
				pidStr = fmt.Sprintf("%d", p.PID)
			}
			fmt.Printf("%-8d %-8s %-26s %-8s %s\n",
				p.Port,
				p.Protocol,
				libs.Truncate(libs.SanitizeTerminal(p.Address), 26),
				pidStr,
				libs.SanitizeTerminal(p.ProcessName),
			)
		}

		if report.Note != "" {
			fmt.Println()
			libs.Colorize(libs.Gray, fmt.Sprintf("ℹ️  %s\n", report.Note))
		}
		fmt.Println()
		return nil
	},
}

func init() {
	// Note: -p is bound globally to --path on rootCmd, so --port does not use -p.
	portsCmd.Flags().IntVar(&filterPort, "port", 0, "filter by specific port number")
	portsCmd.Flags().StringVar(&filterProto, "protocol", "all", "filter by protocol (all, tcp, tcp4, tcp6, udp, udp4, udp6)")
}
