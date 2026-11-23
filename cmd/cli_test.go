package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildTestBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "tazx")
	build := exec.Command("go", "build", "-o", binPath, "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tazx binary: %v\n%s", err, string(out))
	}
	return binPath
}

func runCLI(t *testing.T, bin, home string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestCLIJSONAndCoreCommands(t *testing.T) {
	bin := buildTestBinary(t)
	home := t.TempDir()

	// 1. version --json (must be valid JSON with no banner)
	out, stderr, err := runCLI(t, bin, home, "version", "--json")
	if err != nil {
		t.Fatalf("version --json failed: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(out, "████") {
		t.Fatalf("JSON output must not contain ASCII banner")
	}
	var verPayload map[string]string
	if err := json.Unmarshal([]byte(out), &verPayload); err != nil {
		t.Fatalf("version --json invalid JSON: %v\n%s", err, out)
	}
	if verPayload["version"] == "" {
		t.Errorf("expected version field in JSON output")
	}

	// 2. init creates 0600 config and does not overwrite without --force
	if _, stderr, err := runCLI(t, bin, home, "init"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, stderr)
	}
	cfgFile := filepath.Join(home, ".tazx.yaml")
	info, err := os.Stat(cfgFile)
	if err != nil {
		t.Fatalf("expected config file at %s: %v", cfgFile, err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions on config, got %04o", info.Mode().Perm())
	}
	if _, _, err := runCLI(t, bin, home, "init"); err == nil {
		t.Fatal("expected second init without --force to fail")
	}
	if _, stderr, err := runCLI(t, bin, home, "init", "--force"); err != nil {
		t.Fatalf("init --force failed: %v\nstderr: %s", err, stderr)
	}

	// 3. logs on missing file must fail and NEVER create the file
	missingLog := filepath.Join(home, "missing_access.log")
	if _, _, err := runCLI(t, bin, home, "logs", missingLog); err == nil {
		t.Fatal("expected logs on missing file to fail")
	}
	if _, statErr := os.Stat(missingLog); !os.IsNotExist(statErr) {
		t.Fatalf("logs command created missing file %s!", missingLog)
	}

	// 4. logs on real fixture with secret query & ANSI escape sequences
	fixtureLog := filepath.Join(home, "real_access.log")
	logData := strings.Join([]string{
		`192.168.1.5 - - [02/Aug/2026:14:00:00 +0300] "GET /auth?token=supersecret&ok=1 HTTP/1.1" 200 100 "-" "Mozilla"`,
		`192.168.1.5 - - [02/Aug/2026:14:01:00 +0300] "GET /err` + "\x1b[31m" + `evil HTTP/1.1" 500 50 "-" "curl"`,
	}, "\n") + "\n"
	if err := os.WriteFile(fixtureLog, []byte(logData), 0644); err != nil {
		t.Fatal(err)
	}

	logsOut, stderr, err := runCLI(t, bin, home, "logs", "--errors", "--json", "-p", fixtureLog)
	if err != nil {
		t.Fatalf("logs --errors --json failed: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(logsOut, "\x1b") {
		t.Fatalf("logs output leaked ANSI escape sequence")
	}
	var logsPayload map[string]any
	if err := json.Unmarshal([]byte(logsOut), &logsPayload); err != nil {
		t.Fatalf("logs --json invalid JSON: %v\n%s", err, logsOut)
	}
	if logsPayload["count"].(float64) != 1 {
		t.Errorf("expected 1 error log entry, got %v", logsPayload["count"])
	}

	routesOut, stderr, err := runCLI(t, bin, home, "logs", "--routes", "--json", "-p", fixtureLog)
	if err != nil {
		t.Fatalf("logs --routes --json failed: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(routesOut, "supersecret") {
		t.Fatalf("logs --routes leaked secret token in JSON output")
	}

	// 5. JSON validation across all snapshot commands
	jsonCommands := [][]string{
		{"config", "--json"},
		{"status", "--json", "-p", fixtureLog},
		{"doctor", "--json", "-p", fixtureLog},
		{"cpu", "--json"},
		{"memory", "--json"},
		{"storage", "--json"},
		{"storage", "--large", "--dir", home, "--depth", "1", "--json"},
		{"users", "--json"},
		{"network", "--json"},
		{"top", "--once", "--json"},
		{"monitor", "--once", "--json", "-p", fixtureLog},
	}
	for _, args := range jsonCommands {
		out, errOut, cmdErr := runCLI(t, bin, home, args...)
		if cmdErr != nil {
			t.Fatalf("%v failed: %v\nstderr: %s", args, cmdErr, errOut)
		}
		if !json.Valid([]byte(out)) {
			t.Fatalf("%v produced invalid JSON:\n%s", args, out)
		}
	}

	// 6. ports --json detects live ephemeral listener
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	portNum := ln.Addr().(*net.TCPAddr).Port
	portsOut, portsErr, err := runCLI(t, bin, home, "ports", "--port", fmt.Sprintf("%d", portNum), "--json")
	if err == nil {
		if !json.Valid([]byte(portsOut)) {
			t.Fatalf("ports --json produced invalid JSON: %s", portsOut)
		}
		if !strings.Contains(portsOut, fmt.Sprintf("%d", portNum)) {
			t.Errorf("expected ports output to include ephemeral port %d, got %s", portNum, portsOut)
		}
	} else {
		t.Logf("ports returned permission/env error: %v (%s)", err, portsErr)
	}

	// 7. Reject --json on continuous / streaming commands
	for _, badArgs := range [][]string{
		{"top", "--json"},
		{"monitor", "--json"},
		{"watch", "cpu", "--json"},
		{"logs", "--follow", "--json", "-p", fixtureLog},
		{"docker", "stats", "--watch", "--json"},
	} {
		if _, _, err := runCLI(t, bin, home, badArgs...); err == nil {
			t.Errorf("expected %v to return an error", badArgs)
		}
	}

	// 8. Piped --once commands must not emit alt-screen or screen-clear escapes
	for _, onceArgs := range [][]string{
		{"top", "--once", "--no-color"},
		{"monitor", "--once", "--no-color", "-p", fixtureLog},
	} {
		out, errOut, cmdErr := runCLI(t, bin, home, onceArgs...)
		if cmdErr != nil {
			t.Fatalf("%v failed: %v\nstderr: %s", onceArgs, cmdErr, errOut)
		}
		if strings.Contains(out, "\033[?1049") || strings.Contains(out, "\033[2J") {
			t.Fatalf("%v leaked alt-screen or clear-screen sequence in non-TTY output", onceArgs)
		}
	}

	// 9. update command help & tag validation
	if out, stderr, err := runCLI(t, bin, home, "update", "--help"); err != nil || !strings.Contains(out, "--check") {
		t.Fatalf("update --help failed: %v\nstderr: %s", err, stderr)
	}
	if _, _, err := runCLI(t, bin, home, "update", "--check", "--version", "invalid;tag"); err == nil {
		t.Fatal("expected update --version invalid;tag to fail validation")
	}
}

func TestCLISignalCancellation(t *testing.T) {
	bin := buildTestBinary(t)
	home := t.TempDir()

	cmd := exec.Command(bin, "top", "--refresh", "1")
	cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tazx top: %v", err)
	}

	time.Sleep(400 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean SIGINT exit from tazx top, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("tazx top did not exit cleanly after SIGINT")
	}
}

func TestCLILogsFollowSignalCancellation(t *testing.T) {
	bin := buildTestBinary(t)
	home := t.TempDir()
	logPath := filepath.Join(home, "follow.log")
	if err := os.WriteFile(logPath, []byte("init\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "logs", "--follow", logPath)
	cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tazx logs --follow: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean SIGINT exit from tazx logs --follow, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("tazx logs --follow did not exit cleanly after SIGINT")
	}
}

func TestCLIMonitorSignalCancellation(t *testing.T) {
	bin := buildTestBinary(t)
	home := t.TempDir()

	cmd := exec.Command(bin, "monitor", "--refresh", "1")
	cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tazx monitor: %v", err)
	}

	time.Sleep(400 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean SIGINT exit from tazx monitor, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("tazx monitor did not exit cleanly after SIGINT")
	}
}
