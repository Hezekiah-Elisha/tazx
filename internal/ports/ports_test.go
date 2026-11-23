package ports

import (
	"context"
	"net"
	"testing"
)

func TestFilterAndSortPorts(t *testing.T) {
	raw := []ListeningPort{
		{Port: 443, Protocol: "tcp6", Address: "::", PID: 100, ProcessName: "nginx"},
		{Port: 80, Protocol: "tcp", Address: "0.0.0.0", PID: 100, ProcessName: "nginx"},
		{Port: 80, Protocol: "tcp", Address: "0.0.0.0", PID: 100, ProcessName: "nginx"}, // duplicate
		{Port: 53, Protocol: "udp", Address: "127.0.0.53", PID: 50, ProcessName: "systemd-resolved"},
	}

	all := FilterAndSortPorts(raw, 0, "all")
	if len(all) != 3 {
		t.Fatalf("expected 3 deduplicated ports, got %d", len(all))
	}
	if all[0].Port != 53 || all[1].Port != 80 || all[2].Port != 443 {
		t.Errorf("unexpected port sort order: %+v", all)
	}

	only80 := FilterAndSortPorts(raw, 80, "tcp")
	if len(only80) != 1 || only80[0].Port != 80 {
		t.Errorf("unexpected port 80 filter result: %+v", only80)
	}

	onlyUDP := FilterAndSortPorts(raw, 0, "udp")
	if len(onlyUDP) != 1 || onlyUDP[0].Port != 53 {
		t.Errorf("unexpected udp filter result: %+v", onlyUDP)
	}
}

func TestValidateProtocol(t *testing.T) {
	if err := ValidateProtocol("tcp6"); err != nil {
		t.Errorf("expected tcp6 valid, got %v", err)
	}
	if err := ValidateProtocol("icmp"); err == nil {
		t.Error("expected unsupported protocol error for icmp")
	}
}

func TestGetListeningPortsDetectsEphemeralListener(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind ephemeral TCP listener: %v", err)
	}
	defer ln.Close()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	report, err := GetListeningPorts(context.Background(), tcpAddr.Port, "tcp")
	if err != nil {
		t.Skipf("socket inspection unavailable in environment: %v", err)
	}
	if len(report.Ports) == 0 {
		t.Fatalf("expected ephemeral listener on port %d to be reported", tcpAddr.Port)
	}
	if int(report.Ports[0].Port) != tcpAddr.Port {
		t.Errorf("expected port %d, got %d", tcpAddr.Port, report.Ports[0].Port)
	}
}
