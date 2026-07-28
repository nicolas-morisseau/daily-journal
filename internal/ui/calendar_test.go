package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/journal"
)

// TestCalendarGridAlignment checks that every rendered row is the same width,
// with and without a selected day. Misaligned cells were the one visual bug
// worth locking down with a test.
func TestCalendarGridAlignment(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	j := journal.New(dir, func() time.Time { return now })
	if err := j.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	cal := j.Calendar(journal.MonthOf(now))

	for _, selected := range []time.Time{{}, now, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)} {
		grid := RenderCalendarGrid(Default, cal, selected)
		lines := strings.Split(grid, "\n")
		if len(lines) != len(cal.Weeks)+1 {
			t.Fatalf("expected %d rows, got %d", len(cal.Weeks)+1, len(lines))
		}
		width := lipgloss.Width(lines[0])
		if width != 28 {
			t.Errorf("header width = %d, want 28 (7 cells of 4 columns)", width)
		}
		for i, line := range lines {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("selected=%v row %d width = %d, want %d\n%q", selected, i, got, width, line)
			}
		}
	}
}

func TestRenderCalendarStaticHasLegend(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	j := journal.New(dir, func() time.Time { return now })
	if err := j.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	out := RenderCalendarStatic(Default, j.Calendar(journal.MonthOf(now)))
	for _, want := range []string{"February 2026", "Mo", "Su", "today + entry", "entry", "today"} {
		if !strings.Contains(out, want) {
			t.Errorf("static calendar missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMarkdownKeepsLineCount(t *testing.T) {
	content := "# Heading\n\ntext **bold** and `code`\n\n- bullet\n- [x] done\n\n> quote\n\n---\n"
	got := RenderMarkdown(Default, content, 40)
	wantLines := len(strings.Split(strings.TrimRight(content, "\n"), "\n"))
	if gotLines := len(strings.Split(got, "\n")); gotLines != wantLines {
		t.Errorf("line count = %d, want %d (rendering must not reflow)", gotLines, wantLines)
	}
	if strings.Contains(got, "**") || strings.Contains(got, "`") {
		t.Errorf("markers should be consumed:\n%s", got)
	}
}

func TestThousands(t *testing.T) {
	cases := map[int]string{0: "0", 42: "42", 999: "999", 1000: "1 000", 12345: "12 345", 1234567: "1 234 567"}
	for input, want := range cases {
		if got := Thousands(input); got != want {
			t.Errorf("Thousands(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestHRule(t *testing.T) {
	if got := lipgloss.Width(Default.HRule(10)); got != 10 {
		t.Errorf("HRule(10) width = %d, want 10", got)
	}
	if Default.HRule(0) != "" {
		t.Error("HRule(0) should be empty")
	}
}
