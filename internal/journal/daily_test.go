package journal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newDailyJournal builds a journal with a frozen clock and the given entries,
// keyed by slug.
func newDailyJournal(t *testing.T, now time.Time, entries map[string]string) *Journal {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, EntriesDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	for slug, body := range entries {
		path := filepath.Join(dir, EntriesDirName, slug+".md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return New(dir, func() time.Time { return now })
}

func TestDailyCoversYesterdayAndToday(t *testing.T) {
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := newDailyJournal(t, now, map[string]string{
		"2026-07-30": "# 2026-07-30\n\n**09:12** - opened the parser\n\n**10:40** - fixed it\n",
		"2026-07-29": "# 2026-07-29\n\n**17:05** - shipped the migration\n",
		// Two days back is outside the default window.
		"2026-07-28": "# 2026-07-28\n\n**08:00** - last week's business\n",
	})

	daily := j.Daily(DailyOptions{})

	if got := len(daily.Days); got != 2 {
		t.Fatalf("Days = %d, want today and yesterday", got)
	}
	if got := daily.Days[0].Date.Format("2006-01-02"); got != "2026-07-30" {
		t.Errorf("first day = %s, want today first", got)
	}
	if got := daily.Days[1].Date.Format("2006-01-02"); got != "2026-07-29" {
		t.Errorf("second day = %s, want yesterday", got)
	}
	if got := daily.Notes(); got != 3 {
		t.Errorf("Notes = %d, want the 3 notes of the window", got)
	}
	if daily.Entries() != 2 {
		t.Errorf("Entries = %d, want 2", daily.Entries())
	}
	if daily.Words() == 0 {
		t.Error("the window should count its words")
	}

	notes := daily.Days[0].Notes
	if len(notes) != 2 {
		t.Fatalf("today holds %d notes, want 2", len(notes))
	}
	// Notes keep the order they were written in, and their own time.
	if notes[0].Time != "09:12" || notes[0].Text != "opened the parser" {
		t.Errorf("first note = %+v", notes[0])
	}
	if notes[1].Time != "10:40" {
		t.Errorf("second note = %+v", notes[1])
	}
}

func TestDailyWindowLength(t *testing.T) {
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := newDailyJournal(t, now, nil)

	for _, tc := range []struct {
		days int
		want int
	}{{0, 2}, {1, 2}, {3, 4}} {
		daily := j.Daily(DailyOptions{Days: tc.days})
		if got := len(daily.Days); got != tc.want {
			t.Errorf("--days %d covered %d days, want %d", tc.days, got, tc.want)
		}
	}

	// A window of nothing still holds the day it ends on: a stand-up is about a
	// day, never about none.
	if got := len(j.Daily(DailyOptions{Days: -1}).Days); got != 1 {
		t.Errorf("a negative window covered %d days, want just today", got)
	}
}

func TestDailyListsDaysWithoutAnEntry(t *testing.T) {
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := newDailyJournal(t, now, map[string]string{
		"2026-07-30": "# 2026-07-30\n\n**09:00** - only today\n",
	})

	daily := j.Daily(DailyOptions{})
	if len(daily.Days) != 2 {
		t.Fatalf("Days = %d, want the empty day listed too", len(daily.Days))
	}
	yesterday := daily.Days[1]
	if yesterday.HasEntry {
		t.Error("yesterday has no entry")
	}
	if len(yesterday.Notes) != 0 || yesterday.Words != 0 {
		t.Errorf("an entry-less day should be empty, got %+v", yesterday)
	}
}

// TestDailyKeepsUntimedText is what splitEntry exists for: a recap of the day
// must show prose written without a timestamp, which the sprint review skips.
func TestDailyKeepsUntimedText(t *testing.T) {
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := newDailyJournal(t, now, map[string]string{
		"2026-07-30": "# 2026-07-30\n\n" +
			"- reviewed the migration plan\n- paired on the parser\n\n" +
			"## Notes\n\n**14:00** - stand-up moved to 10am [#bad]\n",
	})

	notes := j.Daily(DailyOptions{}).Days[0].Notes
	if len(notes) != 2 {
		t.Fatalf("notes = %+v, want the untimed block and the timed one", notes)
	}
	if notes[0].Time != "" {
		t.Errorf("the list has no time of its own, got %q", notes[0].Time)
	}
	// The whole block stays one note, headings excluded.
	if want := "- reviewed the migration plan\n- paired on the parser"; notes[0].Text != want {
		t.Errorf("Text = %q, want %q", notes[0].Text, want)
	}
	if notes[1].Time != "14:00" {
		t.Errorf("second note = %+v", notes[1])
	}
	// Tags are part of what was written, so the recap keeps them.
	if want := "stand-up moved to 10am [#bad]"; notes[1].Text != want {
		t.Errorf("Text = %q, want %q", notes[1].Text, want)
	}
}

func TestDailyKeepsAMultiLineNoteTogether(t *testing.T) {
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := newDailyJournal(t, now, map[string]string{
		"2026-07-30": "# 2026-07-30\n\n**09:12** - the parser\n  - broke on empty input\n  - fixed\n",
	})

	notes := j.Daily(DailyOptions{}).Days[0].Notes
	if len(notes) != 1 {
		t.Fatalf("notes = %+v, want one note over three lines", notes)
	}
	if want := "the parser\n  - broke on empty input\n  - fixed"; notes[0].Text != want {
		t.Errorf("Text = %q, want %q", notes[0].Text, want)
	}
}

func TestDailyEndsOnTheRequestedDay(t *testing.T) {
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := newDailyJournal(t, now, map[string]string{
		"2026-07-28": "# 2026-07-28\n\n**08:00** - two days ago\n",
	})

	daily := j.Daily(DailyOptions{To: time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)})
	if got := daily.To.Format("2006-01-02"); got != "2026-07-28" {
		t.Errorf("To = %s, want the requested day", got)
	}
	if daily.Notes() != 1 {
		t.Errorf("Notes = %d, want the note of that day", daily.Notes())
	}
}
