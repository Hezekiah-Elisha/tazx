package libs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		{"plain", "CPU: 10.0%", 10},
		{"ansi", "\033[32mCPU: 10.0%\033[0m", 10},
		{"wide_emoji", "⚡ TAZX", 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := VisibleWidth(tc.line); got != tc.want {
				t.Errorf("VisibleWidth(%q) = %d, want %d", tc.line, got, tc.want)
			}
		})
	}
}

func TestClipANSI(t *testing.T) {
	tests := []struct {
		name string
		line string
		max  int
		want string
	}{
		{"plain_fit", "hello", 5, "hello"},
		{"plain_clip", "hello world", 5, "hello"},
		{"zero_cols", "hello", 0, ""},
		{"ansi_preserved_and_reset", "\033[31mhello world\033[0m", 5, "\033[31mhello\033[0m"},
		{"wide_emoji_width_2", "⚡ TAZX", 4, "⚡ T"},
		{"wide_emoji_exact", "⚡AB", 4, "⚡AB"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClipANSI(tc.line, tc.max); got != tc.want {
				t.Errorf("ClipANSI(%q, %d) = %q, want %q", tc.line, tc.max, got, tc.want)
			}
		})
	}
}

func TestFormatFrame(t *testing.T) {
	frame := "line1\nline2\nline3\nline4\nfooter line\n"
	out := FormatFrame(frame, 10, 4, false)

	if !strings.HasPrefix(out, seqSyncStart+seqCursorHome) {
		t.Errorf("expected synchronized cursor-home prefix, got %q", out)
	}
	if !strings.HasSuffix(out, seqClearBelow+seqSyncEnd) {
		t.Errorf("expected clear-below + sync-end suffix, got %q", out)
	}
	if !strings.Contains(out, "... (+2 mo") {
		t.Errorf("expected overflow ellipsis line clipped to 10 cols, got %q", out)
	}
	if !strings.Contains(out, "footer lin") {
		t.Errorf("expected preserved footer line clipped to 10 cols, got %q", out)
	}

	resized := FormatFrame("abc", 80, 24, true)
	if !strings.HasPrefix(resized, seqSyncStart+seqClearAll+seqCursorHome) {
		t.Errorf("expected clearAll sequence on resize redraw, got %q", resized)
	}
}

func TestFormatDiffFrame(t *testing.T) {
	prevLines, prevWidths := prepareLines("header\nCPU: 12.3%\nfooter", 80, 24)

	// Identical frame -> zero bytes emitted.
	if got := FormatDiffFrame(prevLines, prevWidths, prevLines, prevWidths, false); got != "" {
		t.Errorf("expected empty diff for identical frames, got %q", got)
	}

	// Only row 2 changes and shrinks -> direct row positioning + trailing space overwrite, no full clear.
	newLines, newWidths := prepareLines("header\nCPU: 4.5%\nfooter", 80, 24)
	diff := FormatDiffFrame(prevLines, prevWidths, newLines, newWidths, false)
	if strings.Contains(diff, seqClearAll) || strings.Contains(diff, seqClearBelow) || strings.Contains(diff, seqEraseEOL) {
		t.Errorf("row diff should not emit screen/line erase sequences on same-height update: %q", diff)
	}
	if !strings.Contains(diff, "\033[2;1HCPU: 4.5%"+string(Reset)+" ") {
		t.Errorf("expected direct row-2 update with 1-space tail overwrite, got %q", diff)
	}
	if strings.Contains(diff, "header") || strings.Contains(diff, "footer") {
		t.Errorf("unchanged rows should be skipped, got %q", diff)
	}

	// Shrinking frame clears only removed rows.
	shortLines, shortWidths := prepareLines("header\nCPU: 4.5%", 80, 24)
	shrinkDiff := FormatDiffFrame(prevLines, prevWidths, shortLines, shortWidths, false)
	if !strings.Contains(shrinkDiff, "\033[3;1H"+seqEraseEOL) {
		t.Errorf("expected removed row 3 to be cleared, got %q", shrinkDiff)
	}
}

func TestLiveScreenInteractiveLifecycleAndIdempotency(t *testing.T) {
	var buf bytes.Buffer
	cols, rows := 40, 10
	screen := newLiveScreen(&buf, true, func() (int, int) {
		return cols, rows
	})

	screen.Enter()
	screen.Enter() // idempotent
	screen.Render("CPU: 10%\nFooter")

	cols, rows = 20, 5
	screen.RedrawLast()

	screen.Close()
	screen.Close() // idempotent

	got := buf.String()
	if count := strings.Count(got, seqAltScreenEnter); count != 1 {
		t.Errorf("expected exactly 1 alt-screen enter sequence, got %d", count)
	}
	if count := strings.Count(got, seqAltScreenExit); count != 1 {
		t.Errorf("expected exactly 1 alt-screen exit sequence, got %d", count)
	}
	if !strings.Contains(got, seqCursorHide) || !strings.Contains(got, seqCursorShow) {
		t.Error("expected cursor hide and restore sequences")
	}
	if !strings.Contains(got, seqClearAll+seqCursorHome) {
		t.Error("expected clear-all on resize RedrawLast")
	}
}

func TestLiveScreenNonInteractivePlainOutput(t *testing.T) {
	var buf bytes.Buffer
	screen := NewLiveScreen(&buf)
	if screen.IsInteractive() {
		t.Fatal("bytes.Buffer should not be detected as interactive TTY")
	}

	screen.Enter()
	screen.Render("line one")
	screen.RedrawLast()
	screen.Close()

	got := buf.String()
	if strings.Contains(got, "\033[?1049") || strings.Contains(got, "\033[?2026") || strings.Contains(got, "\033[2J") {
		t.Fatalf("non-interactive LiveScreen must not emit alt-screen or clear sequences: %q", got)
	}
	if got != "line one\n" {
		t.Errorf("got %q, want %q", got, "line one\n")
	}
}

func TestRunLiveLoopNoInitialFlashAndCancel(t *testing.T) {
	var buf bytes.Buffer
	screen := newLiveScreen(&buf, true, func() (int, int) {
		return 80, 24
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	renders := 0
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLiveLoop(ctx, screen, time.Second, "", func(_ context.Context, w io.Writer) error {
			renders++
			fmt.Fprintf(w, "Frame %d\n", renders)
			if renders == 1 {
				select {
				case screen.resizeCh <- nil:
				default:
				}
				go func() {
					time.Sleep(80 * time.Millisecond)
					cancel()
				}()
			}
			return nil
		})
	}()

	if err := <-errCh; err != nil {
		t.Fatalf("RunLiveLoop returned error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Frame 1") {
		t.Error("expected rendered Frame 1")
	}
	if !strings.HasSuffix(got, seqAltScreenExit) {
		t.Errorf("expected alt-screen exit suffix on cancellation, got tail %q", got[max(0, len(got)-24):])
	}
}
