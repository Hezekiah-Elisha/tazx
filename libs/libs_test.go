package libs

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTruncateUnicodeSafe(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"empty", "", 5, ""},
		{"zero_max", "hello", 0, ""},
		{"tiny_max", "hello", 2, "he"},
		{"exact_fit", "hello", 5, "hello"},
		{"truncated", "abcdefgh", 6, "abc..."},
		{"unicode_safe", "αβγδεζηθ", 6, "αβγ..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Truncate(tc.input, tc.max); got != tc.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tc := range tests {
		if got := FormatBytes(tc.bytes); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		sec  uint64
		want string
	}{
		{45, "45s"},
		{125, "2m 5s"},
		{3665, "1h 1m 5s"},
		{90060, "1d 1h 1m"},
	}
	for _, tc := range tests {
		if got := FormatDuration(tc.sec); got != tc.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tc.sec, got, tc.want)
		}
	}
}

func TestSanitizeTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "/api/v1/health", "/api/v1/health"},
		{"csi_escape", "/login\x1b[31mFAKE\x1b[0m", "/loginFAKE"},
		{"osc_title", "\x1b]0;malicious title\x07GET /", "GET /"},
		{"control_chars", "line1\r\n\x00\x7f\tcol2", "line1 col2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeTerminal(tc.in); got != tc.want {
				t.Errorf("SanitizeTerminal(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{"status": "ok", "url": "/a?x=1&y=2"}
	if err := PrintJSON(&buf, payload); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded["url"] != "/a?x=1&y=2" {
		t.Errorf("expected unescaped &, got %v", decoded["url"])
	}
}
