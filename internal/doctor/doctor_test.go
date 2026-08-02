package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"tazx/internal/config"
)

func TestRunDiagnostics(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "doctor_test.log")

	logContent := `192.168.1.10 - - [02/Aug/2026:14:00:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:01:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
192.168.1.10 - - [02/Aug/2026:14:02:00 +0300] "POST /login HTTP/1.1" 401 100 "-" "Mozilla/5.0"
45.33.32.156 - - [02/Aug/2026:14:03:00 +0300] "GET /wp-admin HTTP/1.1" 404 162 "-" "BotAgent"
45.33.32.156 - - [02/Aug/2026:14:04:00 +0300] "GET /.env HTTP/1.1" 404 162 "-" "BotAgent"`

	err := os.WriteFile(logPath, []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	cfg := config.Config{
		LogPath:         logPath,
		RefreshRate:     2,
		CpuThreshold:    10.0, // Force CPU issue check trigger if high
		MemoryThreshold: 10.0,
		DiskThreshold:   10.0,
	}

	report, err := RunDiagnostics(cfg)
	if err != nil {
		t.Fatalf("RunDiagnostics failed: %v", err)
	}

	if report == nil {
		t.Fatal("expected non-nil DiagnosticReport")
	}

	if len(report.Issues) == 0 {
		t.Error("expected issues in DiagnosticReport, got 0")
	}

	foundBotScan := false
	foundFailedLogins := false

	for _, issue := range report.Issues {
		if issue.Title != "" && issue.Recommendation != "" {
			if len(issue.Title) > 0 {
				if issue.Title[0] != 0 {
					// Good title
				}
			}
		}
		if issue.Title != "" {
			if containsString(issue.Title, "bot/scan") || containsString(issue.Title, "wp-admin") {
				foundBotScan = true
			}
			if containsString(issue.Title, "failed logins") {
				foundFailedLogins = true
			}
		}
	}

	if !foundBotScan {
		t.Errorf("expected bot scan issue to be detected in doctor report")
	}
	if !foundFailedLogins {
		t.Errorf("expected failed logins issue to be detected in doctor report")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
