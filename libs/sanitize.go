package libs

import (
	"regexp"
	"strings"
)

var (
	oscSeqRegex = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?`)
	csiSeqRegex = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	escSeqRegex = regexp.MustCompile(`\x1b[@-Z\\-_]`)
)

// SanitizeTerminal strips ANSI escape sequences, C0/C1 control characters,
// and normalizes tabs to spaces so untrusted data cannot spoof terminal output.
func SanitizeTerminal(s string) string {
	if s == "" {
		return ""
	}
	cleaned := oscSeqRegex.ReplaceAllString(s, "")
	cleaned = csiSeqRegex.ReplaceAllString(cleaned, "")
	cleaned = escSeqRegex.ReplaceAllString(cleaned, "")

	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		switch {
		case r == '\t':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
