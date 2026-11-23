package network

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

// InterfaceInfo describes a local network interface and its traffic counters.
type InterfaceInfo struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	MAC     string   `json:"mac,omitempty"`
	MTU     int      `json:"mtu"`
	Flags   []string `json:"flags,omitempty"`
	IPv4    []string `json:"ipv4,omitempty"`
	IPv6    []string `json:"ipv6,omitempty"`
	RxBytes uint64   `json:"rx_bytes"`
	TxBytes uint64   `json:"tx_bytes"`
}

// GatewayInfo represents the host's default IPv4 route gateway.
type GatewayInfo struct {
	Interface string `json:"interface"`
	GatewayIP string `json:"gateway_ip"`
}

// DNSConfig represents parsed resolver configuration.
type DNSConfig struct {
	Nameservers   []string `json:"nameservers,omitempty"`
	SearchDomains []string `json:"search_domains,omitempty"`
}

// ConnectivityCheck holds the result of a non-intrusive network diagnostic check.
type ConnectivityCheck struct {
	Target    string `json:"target"`
	CheckType string `json:"check_type"`
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latency_ms"`
	Detail    string `json:"detail"`
}

// NetworkOverview summarizes interfaces, routing gateway, DNS, and optional connectivity checks.
type NetworkOverview struct {
	Hostname       string              `json:"hostname"`
	Interfaces     []InterfaceInfo     `json:"interfaces"`
	DefaultGateway *GatewayInfo        `json:"default_gateway,omitempty"`
	DNS            *DNSConfig          `json:"dns,omitempty"`
	Connectivity   []ConnectivityCheck `json:"connectivity,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
}

var validCheckHostRegex = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

// ValidateCheckHost ensures a diagnostic target host is a clean hostname or IP (no ports, paths, or control chars).
func ValidateCheckHost(host string) (string, error) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return "example.com", nil
	}
	if len(trimmed) > 253 {
		return "", fmt.Errorf("target host exceeds maximum length")
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return trimmed, nil
	}
	if !validCheckHostRegex.MatchString(trimmed) || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, ".") {
		return "", fmt.Errorf("invalid target host %q", host)
	}
	return trimmed, nil
}

// ParseProcNetRoute parses /proc/net/route format to extract the default IPv4 gateway.
func ParseProcNetRoute(r io.Reader) (*GatewayInfo, error) {
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || lineNo == 1 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		iface := fields[0]
		destHex := fields[1]
		gwHex := fields[2]
		flagsHex := fields[3]

		if destHex != "00000000" {
			continue
		}
		flagsVal, err := strconv.ParseUint(flagsHex, 16, 32)
		if err != nil {
			continue
		}
		// RTF_UP (0x1) | RTF_GATEWAY (0x2)
		if (flagsVal & 0x3) != 0x3 {
			continue
		}

		rawBytes, err := hex.DecodeString(gwHex)
		if err != nil || len(rawBytes) != 4 {
			continue
		}
		// /proc/net/route encodes IPv4 addresses as little-endian 32-bit values on x86/arm64 Linux.
		ipNum := binary.LittleEndian.Uint32(rawBytes)
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, ipNum)

		return &GatewayInfo{
			Interface: iface,
			GatewayIP: ip.String(),
		}, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

// ParseResolvConf extracts nameservers and search domains from resolv.conf content.
func ParseResolvConf(r io.Reader) (*DNSConfig, error) {
	cfg := &DNSConfig{}
	seenNS := make(map[string]bool)
	seenSearch := make(map[string]bool)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if idx := strings.IndexAny(line, "#;"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			ns := fields[1]
			if !seenNS[ns] {
				seenNS[ns] = true
				cfg.Nameservers = append(cfg.Nameservers, ns)
			}
		case "search", "domain":
			for _, d := range fields[1:] {
				if !seenSearch[d] {
					seenSearch[d] = true
					cfg.SearchDomains = append(cfg.SearchDomains, d)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ReadSystemDNS reads /etc/resolv.conf when present.
func ReadSystemDNS() (*DNSConfig, error) {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseResolvConf(f)
}

// GetNetworkOverview gathers local interface details, gateway, DNS, and optional connectivity checks.
func GetNetworkOverview(ctx context.Context, runCheck bool, checkHost string) (*NetworkOverview, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	overview := &NetworkOverview{}
	if hn, err := os.Hostname(); err == nil {
		overview.Hostname = hn
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect network interfaces: %w", err)
	}

	ioByName := make(map[string]gnet.IOCountersStat)
	if counters, ioErr := gnet.IOCountersWithContext(ctx, true); ioErr == nil {
		for _, c := range counters {
			ioByName[c.Name] = c
		}
	} else {
		overview.Warnings = append(overview.Warnings, fmt.Sprintf("Interface byte counters unavailable: %v", ioErr))
	}

	for _, iface := range ifaces {
		state := "DOWN"
		if iface.Flags&net.FlagUp != 0 {
			state = "UP"
		}
		var flagNames []string
		if iface.Flags&net.FlagLoopback != 0 {
			flagNames = append(flagNames, "loopback")
		}
		if iface.Flags&net.FlagBroadcast != 0 {
			flagNames = append(flagNames, "broadcast")
		}
		if iface.Flags&net.FlagMulticast != 0 {
			flagNames = append(flagNames, "multicast")
		}

		var ipv4s, ipv6s []string
		addrs, addrErr := iface.Addrs()
		if addrErr == nil {
			for _, a := range addrs {
				ipStr := a.String()
				ip, _, parseErr := net.ParseCIDR(ipStr)
				if parseErr != nil {
					ipv4s = append(ipv4s, ipStr)
					continue
				}
				if ip.To4() != nil {
					ipv4s = append(ipv4s, ipStr)
				} else {
					ipv6s = append(ipv6s, ipStr)
				}
			}
		}

		c := ioByName[iface.Name]
		overview.Interfaces = append(overview.Interfaces, InterfaceInfo{
			Name:    iface.Name,
			State:   state,
			MAC:     iface.HardwareAddr.String(),
			MTU:     iface.MTU,
			Flags:   flagNames,
			IPv4:    ipv4s,
			IPv6:    ipv6s,
			RxBytes: c.BytesRecv,
			TxBytes: c.BytesSent,
		})
	}

	sort.Slice(overview.Interfaces, func(i, j int) bool {
		return overview.Interfaces[i].Name < overview.Interfaces[j].Name
	})

	if gw, gwErr := getDefaultGateway(ctx); gwErr == nil && gw != nil {
		overview.DefaultGateway = gw
	} else if gwErr != nil {
		overview.Warnings = append(overview.Warnings, fmt.Sprintf("Default gateway unavailable: %v", gwErr))
	}

	if dnsCfg, dnsErr := ReadSystemDNS(); dnsErr == nil {
		overview.DNS = dnsCfg
	} else if !os.IsNotExist(dnsErr) {
		overview.Warnings = append(overview.Warnings, fmt.Sprintf("DNS configuration unavailable: %v", dnsErr))
	}

	if runCheck {
		target, err := ValidateCheckHost(checkHost)
		if err != nil {
			return nil, err
		}
		overview.Connectivity = runConnectivityChecks(ctx, target)
	}

	return overview, nil
}

func runConnectivityChecks(ctx context.Context, host string) []ConnectivityCheck {
	checks := make([]ConnectivityCheck, 0, 2)

	// 1. Non-intrusive DNS resolution check.
	dnsCtx, dnsCancel := context.WithTimeout(ctx, 3*time.Second)
	start := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(dnsCtx, host)
	dnsLatency := time.Since(start).Milliseconds()
	dnsCancel()

	if err != nil {
		checks = append(checks, ConnectivityCheck{
			Target:    host,
			CheckType: "dns",
			Success:   false,
			LatencyMs: dnsLatency,
			Detail:    err.Error(),
		})
	} else {
		detail := fmt.Sprintf("Resolved %d address(es)", len(addrs))
		if len(addrs) > 0 {
			detail = fmt.Sprintf("Resolved to %s", addrs[0])
		}
		checks = append(checks, ConnectivityCheck{
			Target:    host,
			CheckType: "dns",
			Success:   true,
			LatencyMs: dnsLatency,
			Detail:    detail,
		})
	}

	// 2. Single outbound TCP port 443 check (never scans multiple ports).
	tcpCtx, tcpCancel := context.WithTimeout(ctx, 3*time.Second)
	dialer := net.Dialer{Timeout: 3 * time.Second}
	targetAddr := net.JoinHostPort(host, "443")
	start = time.Now()
	conn, err := dialer.DialContext(tcpCtx, "tcp", targetAddr)
	tcpLatency := time.Since(start).Milliseconds()
	tcpCancel()

	if err != nil {
		checks = append(checks, ConnectivityCheck{
			Target:    targetAddr,
			CheckType: "tcp",
			Success:   false,
			LatencyMs: tcpLatency,
			Detail:    err.Error(),
		})
	} else {
		_ = conn.Close()
		checks = append(checks, ConnectivityCheck{
			Target:    targetAddr,
			CheckType: "tcp",
			Success:   true,
			LatencyMs: tcpLatency,
			Detail:    "TCP handshake succeeded",
		})
	}

	return checks
}
