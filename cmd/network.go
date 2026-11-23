package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Hezekiah-Elisha/tazx/internal/network"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var (
	networkCheck bool
	networkHost  string
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "View network interfaces, gateway, DNS, and connectivity",
	Long:  `Display local network interfaces, IP addresses, traffic counters, default gateway, DNS resolvers, and optional non-intrusive connectivity checks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		overview, err := network.GetNetworkOverview(ctx, networkCheck, networkHost)
		if err != nil {
			return err
		}

		if jsonFlag {
			return libs.PrintJSON(os.Stdout, overview)
		}

		libs.Colorize(libs.Bold, "\n🌐 Network Overview\n")

		libs.PrintSection(os.Stdout, "CONFIGURATION")
		if overview.Hostname != "" {
			fmt.Printf("  %-18s : %s\n", "Hostname", libs.SanitizeTerminal(overview.Hostname))
		}
		if overview.DefaultGateway != nil {
			fmt.Printf("  %-18s : %s (via %s)\n", "Default Gateway",
				libs.SanitizeTerminal(overview.DefaultGateway.GatewayIP),
				libs.SanitizeTerminal(overview.DefaultGateway.Interface))
		} else {
			fmt.Printf("  %-18s : %s\n", "Default Gateway", "unavailable")
		}
		if overview.DNS != nil {
			if len(overview.DNS.Nameservers) > 0 {
				fmt.Printf("  %-18s : %s\n", "DNS Nameservers", libs.SanitizeTerminal(strings.Join(overview.DNS.Nameservers, ", ")))
			} else {
				fmt.Printf("  %-18s : %s\n", "DNS Nameservers", "none configured")
			}
			if len(overview.DNS.SearchDomains) > 0 {
				fmt.Printf("  %-18s : %s\n", "Search Domains", libs.SanitizeTerminal(strings.Join(overview.DNS.SearchDomains, ", ")))
			}
		}

		libs.PrintSection(os.Stdout, "INTERFACES")
		if len(overview.Interfaces) == 0 {
			libs.Colorize(libs.Yellow, "No network interfaces detected.\n")
		} else {
			fmt.Printf("%-16s %-8s %-6s %-11s %-11s %s\n", "INTERFACE", "STATE", "MTU", "RX", "TX", "IP ADDRESSES")
			fmt.Println("--------------------------------------------------------------------------------------------")
			for _, iface := range overview.Interfaces {
				stateColor := libs.Yellow
				if iface.State == "UP" {
					stateColor = libs.Green
				}
				allIPs := append([]string{}, iface.IPv4...)
				allIPs = append(allIPs, iface.IPv6...)
				ipList := "-"
				if len(allIPs) > 0 {
					ipList = strings.Join(allIPs, ", ")
				}

				fmt.Printf("%s %s %-6d %s %s %s\n",
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(iface.Name), 16), 16),
					libs.ColorPadRight(stateColor, iface.State, 8),
					iface.MTU,
					libs.PadRight(libs.FormatBytes(iface.RxBytes), 11),
					libs.PadRight(libs.FormatBytes(iface.TxBytes), 11),
					libs.SanitizeTerminal(ipList),
				)
			}
		}

		if len(overview.Connectivity) > 0 {
			libs.PrintSection(os.Stdout, "CONNECTIVITY DIAGNOSTICS")
			fmt.Printf("%-8s %-24s %-8s %-10s %s\n", "TYPE", "TARGET", "STATUS", "LATENCY", "DETAIL")
			fmt.Println("------------------------------------------------------------------------------------")
			for _, chk := range overview.Connectivity {
				statusLabel := "FAIL"
				statusColor := libs.Red
				if chk.Success {
					statusLabel = "OK"
					statusColor = libs.Green
				}
				fmt.Printf("%-8s %s %s %-10s %s\n",
					strings.ToUpper(chk.CheckType),
					libs.PadRight(libs.Truncate(libs.SanitizeTerminal(chk.Target), 24), 24),
					libs.ColorPadRight(statusColor, statusLabel, 8),
					fmt.Sprintf("%dms", chk.LatencyMs),
					libs.SanitizeTerminal(chk.Detail),
				)
			}
		}

		for _, w := range overview.Warnings {
			libs.Colorize(libs.Yellow, fmt.Sprintf("\n⚠️  %s\n", libs.SanitizeTerminal(w)))
		}
		fmt.Println()
		return nil
	},
}

func init() {
	networkCmd.Flags().BoolVar(&networkCheck, "check", false, "run non-intrusive DNS and TCP/443 connectivity diagnostics")
	networkCmd.Flags().StringVar(&networkHost, "host", "example.com", "target host for --check diagnostics")
}
