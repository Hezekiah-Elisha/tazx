package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLineCombinedLogFormat(t *testing.T) {
	line := `192.168.1.100 - - [02/Aug/2026:14:05:00 +0300] "GET /api/v1/checkout HTTP/1.1" 200 4523 "-" "Mozilla/5.0"`
	entry, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if entry.IP != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", entry.IP)
	}
	if entry.Method != "GET" {
		t.Errorf("expected Method GET, got %s", entry.Method)
	}
	if entry.Route != "/api/v1/checkout" {
		t.Errorf("expected Route /api/v1/checkout, got %s", entry.Route)
	}
	if entry.Status != 200 {
		t.Errorf("expected Status 200, got %d", entry.Status)
	}
	if entry.Bytes != 4523 {
		t.Errorf("expected Bytes 4523, got %d", entry.Bytes)
	}
}

func TestAnalyzeLogs(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_access.log")

	logContent := `192.168.1.10 - - [02/Aug/2026:14:00:00 +0300] "GET / HTTP/1.1" 200 1000 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:01:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:02:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:03:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
45.33.32.156 - - [02/Aug/2026:14:04:00 +0300] "GET /wp-admin HTTP/1.1" 404 162 "-" "BotAgent"`

	err := os.WriteFile(logPath, []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test log file: %v", err)
	}

	summary, err := AnalyzeLogs(logPath)
	if err != nil {
		t.Fatalf("AnalyzeLogs failed: %v", err)
	}

	if summary.TotalRequests != 5 {
		t.Errorf("expected 5 total requests, got %d", summary.TotalRequests)
	}
	if summary.ErrorCount != 4 {
		t.Errorf("expected 4 errors, got %d", summary.ErrorCount)
	}
	if summary.FailedLogins != 3 {
		t.Errorf("expected 3 failed logins, got %d", summary.FailedLogins)
	}
	if summary.SuspiciousRoutes["/wp-admin"] != 1 {
		t.Errorf("expected 1 suspicious request to /wp-admin, got %d", summary.SuspiciousRoutes["/wp-admin"])
	}
}

func TestTailLogs(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_tail.log")

	logContent := `192.168.1.10 - - [02/Aug/2026:14:00:00 +0300] "GET / HTTP/1.1" 200 1000 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:01:00 +0300] "GET /api/err HTTP/1.1" 500 200 "-" "Mozilla/5.0"`

	err := os.WriteFile(logPath, []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	entries, _, err := TailLogs(logPath, 10, true, false)
	if err != nil {
		t.Fatalf("TailLogs failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 error entry, got %d", len(entries))
	}
	if entries[0].Status != 500 {
		t.Errorf("expected error entry status 500, got %d", entries[0].Status)
	}
}
