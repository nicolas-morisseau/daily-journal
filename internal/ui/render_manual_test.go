package ui

import (
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"

	"daily-journal/internal/journal"
)

// TestRenderPreview prints the interactive views to stdout so their layout can
// be eyeballed with `go test -run TestRenderPreview -v ./internal/ui`. It only
// asserts that rendering does not panic and produces output.
func TestRenderPreview(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/entries", 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# 2026-07-28\n\n" +
		"Shipped the Go rewrite. Notes:\n\n- bubbletea for the views\n- [x] calendar aligned\n- [ ] write the README\n\n" +
		"> Ownership beats convenience.\n\nTagged #work and `daily-journal`.\n"
	for _, slug := range []string{"2026-07-20", "2026-07-26", "2026-07-27", "2026-07-28"} {
		if err := os.WriteFile(dir+"/entries/"+slug+".md", []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	j := journal.New(dir, func() time.Time { return now })

	cal := calendarModel{theme: Default, source: newFakeSource(j), entry: viewport.New(40, 10), width: 120, height: 32}
	cal.setMonth(journal.MonthOf(now))
	cal.selectDay(now)
	if got := cal.View(); got == "" {
		t.Fatal("calendar rendered nothing")
	} else {
		t.Log("\n" + got)
	}
}
