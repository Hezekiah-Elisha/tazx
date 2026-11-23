package ports

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"syscall"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// ListeningPort represents an active listening TCP or bound UDP socket.
type ListeningPort struct {
	Port        uint32 `json:"port"`
	Protocol    string `json:"protocol"`
	Address     string `json:"address"`
	PID         int32  `json:"pid"`
	ProcessName string `json:"process_name"`
}

// PortsReport aggregates discovered listening sockets and permission hints.
type PortsReport struct {
	Ports             []ListeningPort `json:"ports"`
	PermissionLimited bool            `json:"permission_limited"`
	Note              string          `json:"note,omitempty"`
}

func classifyProtocol(family, sockType uint32) string {
	switch sockType {
	case syscall.SOCK_STREAM:
		if family == syscall.AF_INET6 {
			return "tcp6"
		}
		return "tcp"
	case syscall.SOCK_DGRAM:
		if family == syscall.AF_INET6 {
			return "udp6"
		}
		return "udp"
	default:
		return "unknown"
	}
}

func formatListenAddr(ip string, family uint32) string {
	if ip != "" {
		return ip
	}
	if family == syscall.AF_INET6 {
		return "::"
	}
	return "0.0.0.0"
}

func matchesProtocol(proto, filter string) bool {
	f := strings.ToLower(strings.TrimSpace(filter))
	if f == "" || f == "all" {
		return true
	}
	switch f {
	case "tcp":
		return proto == "tcp" || proto == "tcp6"
	case "tcp4":
		return proto == "tcp"
	case "tcp6":
		return proto == "tcp6"
	case "udp":
		return proto == "udp" || proto == "udp6"
	case "udp4":
		return proto == "udp"
	case "udp6":
		return proto == "udp6"
	default:
		return proto == f
	}
}

// ValidateProtocol validates the user-supplied protocol filter.
func ValidateProtocol(proto string) error {
	f := strings.ToLower(strings.TrimSpace(proto))
	switch f {
	case "", "all", "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		return nil
	default:
		return fmt.Errorf("unsupported protocol %q (use all, tcp, tcp4, tcp6, udp, udp4, or udp6)", proto)
	}
}

// FilterAndSortPorts deduplicates, filters, and sorts listening port entries deterministically.
func FilterAndSortPorts(raw []ListeningPort, filterPort int, filterProto string) []ListeningPort {
	type dedupKey struct {
		port  uint32
		proto string
		addr  string
		pid   int32
	}
	seen := make(map[dedupKey]bool)
	filtered := make([]ListeningPort, 0, len(raw))

	for _, item := range raw {
		if filterPort > 0 && int(item.Port) != filterPort {
			continue
		}
		if !matchesProtocol(item.Protocol, filterProto) {
			continue
		}
		key := dedupKey{
			port:  item.Port,
			proto: item.Protocol,
			addr:  item.Address,
			pid:   item.PID,
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, item)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Port != filtered[j].Port {
			return filtered[i].Port < filtered[j].Port
		}
		if filtered[i].Protocol != filtered[j].Protocol {
			return filtered[i].Protocol < filtered[j].Protocol
		}
		if filtered[i].Address != filtered[j].Address {
			return filtered[i].Address < filtered[j].Address
		}
		return filtered[i].PID < filtered[j].PID
	})

	return filtered
}

// GetListeningPorts inspects active listening TCP and bound UDP sockets.
func GetListeningPorts(ctx context.Context, filterPort int, filterProto string) (*PortsReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ValidateProtocol(filterProto); err != nil {
		return nil, err
	}
	if filterPort < 0 || filterPort > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535 (got %d)", filterPort)
	}

	conns, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return nil, fmt.Errorf("failed to inspect listening sockets: %w", err)
	}

	report := &PortsReport{}
	pidNames := make(map[int32]string)
	var raw []ListeningPort

	for _, c := range conns {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		isTCPListen := c.Type == syscall.SOCK_STREAM && strings.EqualFold(c.Status, "LISTEN")
		isUDPListen := c.Type == syscall.SOCK_DGRAM && c.Laddr.Port > 0 && c.Raddr.Port == 0
		if !isTCPListen && !isUDPListen {
			continue
		}

		proto := classifyProtocol(c.Family, c.Type)
		addr := formatListenAddr(c.Laddr.IP, c.Family)

		procName := "-"
		if c.Pid > 0 {
			if cached, ok := pidNames[c.Pid]; ok {
				procName = cached
			} else {
				if p, pErr := process.NewProcessWithContext(ctx, c.Pid); pErr == nil {
					if name, nErr := p.NameWithContext(ctx); nErr == nil && name != "" {
						procName = name
					} else {
						report.PermissionLimited = true
					}
				} else {
					report.PermissionLimited = true
				}
				pidNames[c.Pid] = procName
			}
		} else {
			report.PermissionLimited = true
		}

		raw = append(raw, ListeningPort{
			Port:        c.Laddr.Port,
			Protocol:    proto,
			Address:     addr,
			PID:         c.Pid,
			ProcessName: procName,
		})
	}

	report.Ports = FilterAndSortPorts(raw, filterPort, filterProto)
	if report.PermissionLimited {
		report.Note = "Some PIDs or process names are hidden due to OS socket permissions; run with elevated privileges only if process mapping is required."
	}

	return report, nil
}
