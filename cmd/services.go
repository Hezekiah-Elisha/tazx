package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Hezekiah-Elisha/tazx/internal/services"
	"github.com/Hezekiah-Elisha/tazx/libs"

	"github.com/spf13/cobra"
)

var failedServicesOnly bool

var servicesCmd = &cobra.Command{
	Use:   "services [service]",
	Short: "Inspect systemd services (Linux)",
	Long:  `Inspect Linux systemd services, check failed units, or inspect details for a specific service (for example: tazx services nginx).`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signalContext()
		defer cancel()

		if len(args) == 1 {
			svc, err := services.GetService(ctx, args[0])
			if err != nil {
				if errors.Is(err, services.ErrSystemdUnavailable) {
					return fmt.Errorf("systemd is not available on this host (requires Linux running systemd)")
				}
				return err
			}

			if jsonFlag {
				return libs.PrintJSON(os.Stdout, svc)
			}

			libs.Colorize(libs.Bold, fmt.Sprintf("\n🛠️  Systemd Service: %s\n\n", libs.SanitizeTerminal(svc.Name)))
			stateColor := libs.Green
			if svc.ActiveState == "failed" {
				stateColor = libs.Red
			} else if svc.ActiveState != "active" {
				stateColor = libs.Yellow
			}

			fmt.Printf("  %-15s : %s\n", "Name", libs.SanitizeTerminal(svc.Name))
			fmt.Printf("  %-15s : %s\n", "Load State", libs.SanitizeTerminal(svc.LoadState))
			fmt.Printf("  %-15s : %s (%s)\n", "Active State", libs.SprintColor(stateColor, libs.SanitizeTerminal(svc.ActiveState)), libs.SanitizeTerminal(svc.SubState))
			fmt.Printf("  %-15s : %s\n", "Unit File", libs.SanitizeTerminal(svc.Enabled))
			if svc.MainPID > 0 {
				fmt.Printf("  %-15s : %d\n", "Main PID", svc.MainPID)
			}
			if svc.Since != "" {
				fmt.Printf("  %-15s : %s\n", "Active Since", libs.SanitizeTerminal(svc.Since))
			}
			if svc.Description != "" {
				fmt.Printf("  %-15s : %s\n", "Description", libs.SanitizeTerminal(svc.Description))
			}
			fmt.Println()
			return nil
		}

		list, err := services.ListServices(ctx, failedServicesOnly)
		if err != nil {
			if errors.Is(err, services.ErrSystemdUnavailable) {
				return fmt.Errorf("systemd is not available on this host (requires Linux running systemd)")
			}
			return err
		}

		if jsonFlag {
			if list == nil {
				list = []services.ServiceInfo{}
			}
			return libs.PrintJSON(os.Stdout, list)
		}

		title := "Systemd Services"
		if failedServicesOnly {
			title = "Failed Systemd Services"
		}
		libs.Colorize(libs.Bold, fmt.Sprintf("\n🛠️  %s\n\n", title))

		if len(list) == 0 {
			if failedServicesOnly {
				libs.Colorize(libs.Green, "✅ No failed systemd services detected.\n\n")
			} else {
				libs.Colorize(libs.Yellow, "No systemd service units found.\n\n")
			}
			return nil
		}

		fmt.Printf("%-34s %-10s %-10s %-12s %-12s %s\n", "UNIT", "LOAD", "ACTIVE", "SUB", "ENABLED", "DESCRIPTION")
		fmt.Println("--------------------------------------------------------------------------------------------------------------")

		for _, s := range list {
			activeColor := libs.Green
			if s.ActiveState == "failed" {
				activeColor = libs.Red
			} else if s.ActiveState != "active" {
				activeColor = libs.Yellow
			}

			fmt.Printf("%s %s %s %s %s %s\n",
				libs.PadRight(libs.Truncate(libs.SanitizeTerminal(s.Name), 34), 34),
				libs.PadRight(libs.Truncate(libs.SanitizeTerminal(s.LoadState), 10), 10),
				libs.ColorPadRight(activeColor, libs.SanitizeTerminal(s.ActiveState), 10),
				libs.PadRight(libs.Truncate(libs.SanitizeTerminal(s.SubState), 12), 12),
				libs.PadRight(libs.Truncate(libs.SanitizeTerminal(s.Enabled), 12), 12),
				libs.Truncate(libs.SanitizeTerminal(s.Description), 45),
			)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	servicesCmd.Flags().BoolVar(&failedServicesOnly, "failed", false, "show only failed systemd services")
}
