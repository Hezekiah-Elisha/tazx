package logs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLineCombinedAndCommon(t *testing.T) {
	line := `192.168.1.100 - - [02/Aug/2026:14:05:00 +0300] "GET /api/v1/checkout?token=secret123&page=2 HTTP/1.1" 200 4523 "-" "Mozilla/5.0"`
	entry, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if !entry.Parsed || !entry.HasTimestamp {
		t.Fatal("expected structured entry with parsed timestamp")
	}
	if entry.IP != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", entry.IP)
	}
	if entry.Method != "GET" {
		t.Errorf("expected Method GET, got %s", entry.Method)
	}
	if entry.Route != "/api/v1/checkout?token=[REDACTED]&page=2" {
		t.Errorf("expected secret query param redacted, got %s", entry.Route)
	}
	if entry.Status != 200 || entry.IsError {
		t.Errorf("expected Status 200 and IsError=false, got %d / %v", entry.Status, entry.IsError)
	}
	if entry.Bytes != 4523 {
		t.Errorf("expected Bytes 4523, got %d", entry.Bytes)
	}
}

func TestParseLineUnstructuredAndSanitization(t *testing.T) {
	normalLine := "2026-08-02 info user 5004 connected from 10.0.4.404"
	entry, err := ParseLine(normalLine)
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}
	if entry.Parsed {
		t.Error("expected unstructured line to have Parsed=false")
	}
	if entry.Status != 0 || entry.IsError {
		t.Errorf("numeric substrings like 5004/404 should not trigger fake HTTP errors: %+v", entry)
	}

	errLine := "2026-08-02 ERROR connection failed \x1b[31mBAD\x1b[0m password=hunter2"
	errEntry, err := ParseLine(errLine)
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}
	if !errEntry.IsError {
		t.Error("expected ERROR keyword to mark IsError=true")
	}
	if strings.Contains(errEntry.RawLine, "\x1b") || strings.Contains(errEntry.RawLine, "hunter2") {
		t.Errorf("expected escape sequences and inline password redacted, got %q", errEntry.RawLine)
	}
}

func TestAnalyzeLogsMissingFileDoesNotCreateSeed(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "does_not_exist.log")
	_, err := AnalyzeLogs(context.Background(), missingPath)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
	if _, statErr := os.Stat(missingPath); !os.IsNotExist(statErr) {
		t.Fatalf("AnalyzeLogs must NEVER silently create missing log files!")
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

	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write test log file: %v", err)
	}

	summary, err := AnalyzeLogs(context.Background(), logPath)
	if err != nil {
		t.Fatalf("AnalyzeLogs failed: %v", err)
	}

	if summary.TotalRequests != 5 {
		t.Errorf("expected 5 total requests, got %d", summary.TotalRequests)
	}
	if summary.ErrorCount != 4 || summary.Status4xx != 4 {
		t.Errorf("expected 4 errors/4xx, got %d / %d", summary.ErrorCount, summary.Status4xx)
	}
	if summary.ErrorRate != 80.0 {
		t.Errorf("expected 80.0%% error rate, got %.1f", summary.ErrorRate)
	}
	if summary.FailedLogins != 3 {
		t.Errorf("expected 3 failed logins, got %d", summary.FailedLogins)
	}
	if summary.SuspiciousRoutes["/wp-admin"] != 1 {
		t.Errorf("expected 1 suspicious request to /wp-admin, got %d", summary.SuspiciousRoutes["/wp-admin"])
	}
	if summary.RequestsPerMin != 1.25 {
		t.Errorf("expected 1.25 req/min over 4m span, got %.2f", summary.RequestsPerMin)
	}
}

func TestTailLogsAndErrorsFilter(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_tail.log")

	logContent := `192.168.1.10 - - [02/Aug/2026:14:00:00 +0300] "GET / HTTP/1.1" 200 1000 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:01:00 +0300] "GET /api/err HTTP/1.1" 500 200 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:02:00 +0300] "GET /not-found HTTP/1.1" 404 200 "-" "Mozilla/5.0"`

	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	entries, err := TailLogs(context.Background(), logPath, 1, true)
	if err != nil {
		t.Fatalf("TailLogs failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Status != 404 {
		t.Fatalf("expected last 1 error entry (404), got %+v", entries)
	}
}

func TestFollowLogsAppendedAndContextCancel(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "stream.log")
	if err := os.WriteFile(logPath, []byte("initial line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got := make(chan LogEntry, 4)
	errCh := make(chan error, 1)
	go func() {
		errCh <- FollowLogs(ctx, logPath, true, func(e LogEntry) {
			got <- e
		})
	}()

	time.Sleep(150 * time.Millisecond)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`10.0.0.1 - - [02/Aug/2026:14:05:00 +0300] "GET /ok HTTP/1.1" 200 10 "-" "curl"` + "\n")
	_, _ = f.WriteString(`10.0.0.1 - - [02/Aug/2026:14:05:01 +0300] "GET /boom HTTP/1.1" 502 10 "-" "curl"` + "\n")
	_ = f.Close()

	select {
	case e := <-got:
		if e.Status != 502 || e.Route != "/boom" {
			t.Fatalf("unexpected streamed entry: %+v", e)
		}
		cancel()
	case <-ctx.Done():
		t.Fatal("timed out waiting for FollowLogs entry")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("FollowLogs returned unexpected error on cancel: %v", err)
	}
}
