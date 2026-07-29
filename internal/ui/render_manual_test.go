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

// TestRenderReviewPreview prints the sprint review so its layout can be
// eyeballed with `go test -run TestRenderReviewPreview -v ./internal/ui`.
func TestRenderReviewPreview(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/entries", 0o755); err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{
		"2026-07-20": "# 2026-07-20\n\n**09:12** - [#good] Finally understood why the deploy was flaky: the cache warmup raced the health check. Wrote it up for the team.\n\n**11:49** - [#bad] Lost in the MRs. sbt 2 broke the double push through the orb, and fixing it took the whole morning.\n",
		"2026-07-23": "# 2026-07-23\n\n**16:40** - [#mad] The staging database was wiped again by a job nobody owns.\n\n**18:16** - All hands. Good to have the swarms' objectives in one place. [#good]\n",
		"2026-07-27": "# 2026-07-27\n\n**11:35** - Found a public repo exposing the whole internal roadmap. Back to internal with Loic. [#bad]\n",
		"2026-07-28": "# 2026-07-28\n\n**17:49** - A permanent contract at last. Relieved, and far more focused on the work. [#good]\n",
	}
	for slug, body := range entries {
		if err := os.WriteFile(dir+"/entries/"+slug+".md", []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	j := journal.New(dir, func() time.Time { return now })
	source := &fakeReviewSource{journal: j}

	m := reviewModel{theme: Default, source: source, width: 120, height: 32}
	for i, mood := range journal.Moods {
		m.columns[i].mood = mood
		m.columns[i].view = viewport.New(minReviewColumnWidth, 10)
	}
	m.load(journal.ReviewOptions{}, true)
	t.Log("\n" + m.View())

	review, err := j.Review(journal.ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + RenderReviewStatic(Default, review, 118))
}
