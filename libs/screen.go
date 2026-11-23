package libs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	seqAltScreenEnter = "\033[?1049h"
	seqAltScreenExit  = "\033[?1049l"
	seqCursorHide     = "\033[?25l"
	seqCursorShow     = "\033[?25h"
	seqSyncStart      = "\033[?2026h"
	seqSyncEnd        = "\033[?2026l"
	seqCursorHome     = "\033[H"
	seqClearAll       = "\033[2J"
	seqEraseEOL       = "\033[K"
	seqClearBelow     = "\033[J"
)

func runeWidth(r rune) int {
	if r >= 0x1F300 || (r >= 0x2600 && r <= 0x27BF) {
		return 2
	}
	return 1
}

// VisibleWidth returns the visible terminal column width of line, ignoring ANSI CSI escapes.
func VisibleWidth(line string) int {
	cols := 0
	for i := 0; i < len(line); {
		if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && (line[j] < 0x40 || line[j] > 0x7E) {
				j++
			}
			if j < len(line) {
				j++
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		cols += runeWidth(r)
		i += size
	}
	return cols
}

// ClipANSI truncates a single line to at most maxCols visible terminal columns
// while preserving ANSI CSI/SGR escape sequences and avoiding color bleed.
func ClipANSI(line string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(line))

	cols := 0
	sawANSI := false
	truncated := false

	for i := 0; i < len(line); {
		if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
			sawANSI = true
			j := i + 2
			for j < len(line) && (line[j] < 0x40 || line[j] > 0x7E) {
				j++
			}
			if j < len(line) {
				j++
			}
			b.WriteString(line[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		w := runeWidth(r)
		if cols+w > maxCols {
			truncated = true
			break
		}
		b.WriteString(line[i : i+size])
		cols += w
		i += size
	}

	if truncated && sawANSI {
		b.WriteString(string(Reset))
	}
	return b.String()
}

func prepareLines(frame string, cols, rows int) ([]string, []int) {
	normalized := strings.ReplaceAll(frame, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	lines := strings.Split(normalized, "\n")

	if rows > 0 && len(lines) > rows {
		if rows == 1 {
			lines = lines[:1]
		} else {
			omitted := len(lines) - (rows - 1)
			footer := lines[len(lines)-1]
			clamped := make([]string, 0, rows)
			clamped = append(clamped, lines[:rows-2]...)
			clamped = append(clamped, fmt.Sprintf("... (+%d more lines)", omitted))
			clamped = append(clamped, footer)
			lines = clamped
		}
	}

	widths := make([]int, len(lines))
	for i, line := range lines {
		if cols > 0 {
			line = ClipANSI(line, cols)
			lines[i] = line
		}
		widths[i] = VisibleWidth(line)
	}
	return lines, widths
}

// FormatDiffFrame generates minimal ANSI escape updates between prevLines and newLines.
// Unchanged rows are skipped; shorter changed rows overwrite trailing characters with spaces
// instead of flashing with erase-line escapes. Returns "" when nothing changed.
func FormatDiffFrame(prevLines []string, prevWidths []int, newLines []string, newWidths []int, clearAll bool) string {
	if clearAll || len(prevLines) == 0 {
		var out strings.Builder
		out.Grow(len(newLines)*48 + 32)
		out.WriteString(seqSyncStart)
		if clearAll {
			out.WriteString(seqClearAll)
		}
		out.WriteString(seqCursorHome)

		for i, line := range newLines {
			out.WriteString(line)
			out.WriteString(string(Reset))
			out.WriteString(seqEraseEOL)
			if i < len(newLines)-1 {
				out.WriteByte('\n')
			}
		}

		out.WriteString(seqClearBelow)
		out.WriteString(seqSyncEnd)
		return out.String()
	}

	var diff strings.Builder
	for i, line := range newLines {
		if i < len(prevLines) && line == prevLines[i] {
			continue
		}
		fmt.Fprintf(&diff, "\033[%d;1H%s%s", i+1, line, string(Reset))
		if i < len(prevWidths) && newWidths[i] < prevWidths[i] {
			diff.WriteString(strings.Repeat(" ", prevWidths[i]-newWidths[i]))
		}
	}

	for i := len(newLines); i < len(prevLines); i++ {
		fmt.Fprintf(&diff, "\033[%d;1H%s", i+1, seqEraseEOL)
	}

	if diff.Len() == 0 {
		return ""
	}
	return seqSyncStart + diff.String() + seqSyncEnd
}

// FormatFrame clamps a frame to terminal dimensions (preserving the bottom footer line on overflow)
// and wraps it in synchronized-update and in-place erase sequences to prevent flicker.
func FormatFrame(frame string, cols, rows int, clearAll bool) string {
	lines, widths := prepareLines(frame, cols, rows)
	return FormatDiffFrame(nil, nil, lines, widths, clearAll)
}

// LiveScreen manages non-destructive alternate-screen rendering for interactive terminals
// and clean plain-output fallback when stdout is non-interactive.
type LiveScreen struct {
	out         io.Writer
	interactive bool
	sizeFn      func() (int, int)

	mu         sync.Mutex
	entered    bool
	lastFrame  string
	prevLines  []string
	prevWidths []int
	resizeCh   chan os.Signal
}

// NewLiveScreen creates a LiveScreen targeting out.
func NewLiveScreen(out io.Writer) *LiveScreen {
	f, isFile := out.(*os.File)
	interactive := isFile && IsTerminal(f)
	var sizeFn func() (int, int)
	if interactive {
		sizeFn = func() (int, int) {
			return terminalSize(f)
		}
	}
	return newLiveScreen(out, interactive, sizeFn)
}

func newLiveScreen(out io.Writer, interactive bool, sizeFn func() (int, int)) *LiveScreen {
	var resizeCh chan os.Signal
	if interactive {
		resizeCh = make(chan os.Signal, 1)
	}
	return &LiveScreen{
		out:         out,
		interactive: interactive,
		sizeFn:      sizeFn,
		resizeCh:    resizeCh,
	}
}

// IsInteractive reports whether the screen is attached to an interactive TTY.
func (s *LiveScreen) IsInteractive() bool {
	return s.interactive
}

// ResizeEvents returns a channel signaled on terminal resize (nil when non-interactive).
func (s *LiveScreen) ResizeEvents() <-chan os.Signal {
	return s.resizeCh
}

// Enter switches an interactive terminal to the alternate screen buffer and hides the cursor.
func (s *LiveScreen) Enter() {
	if !s.interactive {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entered {
		return
	}
	s.entered = true
	s.prevLines = nil
	s.prevWidths = nil
	if s.resizeCh != nil {
		registerResize(s.resizeCh)
	}
	_, _ = io.WriteString(s.out, seqAltScreenEnter+seqCursorHide+seqClearAll+seqCursorHome)
}

// Close restores the original terminal screen, resets SGR attributes, and shows the cursor.
func (s *LiveScreen) Close() {
	if !s.interactive {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.entered {
		return
	}
	s.entered = false
	s.lastFrame = ""
	s.prevLines = nil
	s.prevWidths = nil
	if s.resizeCh != nil {
		unregisterResize(s.resizeCh)
	}
	_, _ = io.WriteString(s.out, string(Reset)+seqCursorShow+seqAltScreenExit)
}

func (s *LiveScreen) dimensions() (int, int) {
	if s.sizeFn == nil {
		return 0, 0
	}
	return s.sizeFn()
}

// Render paints only changed rows atomically in-place (or writes directly in non-interactive mode).
func (s *LiveScreen) Render(frame string) {
	if !s.interactive {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, _ = io.WriteString(s.out, frame)
		if !strings.HasSuffix(frame, "\n") {
			_, _ = io.WriteString(s.out, "\n")
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFrame = frame
	cols, rows := s.dimensions()
	newLines, newWidths := prepareLines(frame, cols, rows)
	formatted := FormatDiffFrame(s.prevLines, s.prevWidths, newLines, newWidths, false)
	s.prevLines = newLines
	s.prevWidths = newWidths
	if formatted != "" {
		_, _ = io.WriteString(s.out, formatted)
	}
}

// RedrawLast re-clips and repaints the most recently rendered frame after a resize.
func (s *LiveScreen) RedrawLast() {
	if !s.interactive {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.entered || s.lastFrame == "" {
		return
	}
	cols, rows := s.dimensions()
	newLines, newWidths := prepareLines(s.lastFrame, cols, rows)
	formatted := FormatDiffFrame(nil, nil, newLines, newWidths, true)
	s.prevLines = newLines
	s.prevWidths = newWidths
	_, _ = io.WriteString(s.out, formatted)
}

// RunLiveLoop executes a flicker-free, resize-aware live refresh loop until ctx is canceled.
func RunLiveLoop(
	ctx context.Context,
	screen *LiveScreen,
	interval time.Duration,
	initialMsg string,
	render func(context.Context, io.Writer) error,
) error {
	if interval < time.Second {
		interval = time.Second
	}

	defer screen.Close()

	if screen.IsInteractive() && initialMsg != "" {
		screen.Enter()
		screen.Render(initialMsg)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	resizeCh := screen.ResizeEvents()

	for {
		var buf bytes.Buffer
		if err := render(ctx, &buf); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		screen.Enter()
		screen.Render(buf.String())

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-resizeCh:
				for len(resizeCh) > 0 {
					<-resizeCh
				}
				screen.RedrawLast()
			case <-ticker.C:
				goto nextSample
			}
		}
	nextSample:
	}
}
