package libs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Color string

const (
	Red     Color = "\033[31m"
	Green   Color = "\033[32m"
	Yellow  Color = "\033[33m"
	Blue    Color = "\033[34m"
	Magenta Color = "\033[35m"
	Cyan    Color = "\033[36m"
	Gray    Color = "\033[90m"
	Bold    Color = "\033[1m"
	Reset   Color = "\033[0m"
)

// NoColor globally disables ANSI color output when true.
var NoColor bool

// SetNoColor sets the global color suppression flag.
func SetNoColor(disable bool) {
	NoColor = disable
}

// IsTerminal reports whether the given file is connected to an interactive terminal.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// IsColorEnabled returns true when ANSI colors should be emitted to stdout.
func IsColorEnabled() bool {
	if NoColor {
		return false
	}
	if _, exists := os.LookupEnv("NO_COLOR"); exists && os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return IsTerminal(os.Stdout)
}

// Colorize prints colored text directly to standard output.
func Colorize(color Color, text string) {
	ColorizeTo(os.Stdout, color, text)
}

// ColorizeTo prints colored text to w when color output is enabled.
func ColorizeTo(w io.Writer, color Color, text string) {
	if !IsColorEnabled() {
		fmt.Fprint(w, text)
		return
	}
	fmt.Fprintf(w, "%s%s%s", color, text, Reset)
}

// SprintColor returns a colorized string with ANSI codes when color is enabled.
func SprintColor(color Color, text string) string {
	if !IsColorEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", color, text, Reset)
}

// ColorPadRight truncates/pads text to width visible runes before applying ANSI color.
// This ensures aligned columns even when ANSI escape codes are present.
func ColorPadRight(color Color, text string, width int) string {
	padded := PadRight(Truncate(text, width), width)
	if color == "" || color == Reset {
		return padded
	}
	return SprintColor(color, padded)
}

// PrintSection prints a consistent, bold section heading to w.
func PrintSection(w io.Writer, title string) {
	heading := fmt.Sprintf("\n=== %s ===\n", strings.ToUpper(title))
	ColorizeTo(w, Bold, heading)
}

// PrintJSON encodes v as formatted, machine-readable JSON to w.
func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
