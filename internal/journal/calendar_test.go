package journal

import (
	"testing"
	"time"
)

func TestCalendarLayout(t *testing.T) {
	// July 2026 starts on a Wednesday and has 31 days.
	j := newTestJournal(t, "2026-07-01", "2026-07-28")
	cal := j.Calendar(Month{Year: 2026, Month: time.July})

	if len(cal.Weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(cal.Weeks))
	}
	for i, week := range cal.Weeks {
		if len(week) != 7 {
			t.Errorf("week %d has %d cells, want 7", i, len(week))
		}
	}

	// Monday and Tuesday of the first week are padding.
	if !cal.Weeks[0][0].Padding() || !cal.Weeks[0][1].Padding() {
		t.Error("the first two cells should be padding")
	}
	first := cal.Weeks[0][2]
	if first.Padding() || first.Date.Day() != 1 {
		t.Errorf("Wednesday should hold the 1st, got %+v", first)
	}
	if !first.HasEntry {
		t.Error("2026-07-01 has an entry and should be marked")
	}
	if first.Words == 0 {
		t.Error("word count should be filled in for days with an entry")
	}

	// The 28th is today for the frozen clock, and it has an entry.
	var today Day
	for _, week := range cal.Weeks {
		for _, day := range week {
			if !day.Padding() && day.Date.Day() == 28 {
				today = day
			}
		}
	}
	if !today.IsToday || !today.HasEntry {
		t.Errorf("the 28th should be today with an entry, got %+v", today)
	}

	// Trailing cells of the last week are padding.
	last := cal.Weeks[4]
	if !last[5].Padding() || !last[6].Padding() {
		t.Error("July 2026 should end with two padding cells")
	}
}

func TestMonthHelpers(t *testing.T) {
	m, err := ParseMonth("2026-02")
	if err != nil {
		t.Fatal(err)
	}
	if m.Year != 2026 || m.Month != time.February {
		t.Errorf("ParseMonth = %+v", m)
	}
	if got := m.Days(time.UTC); got != 28 {
		t.Errorf("February 2026 has %d days, want 28", got)
	}
	if got := (Month{Year: 2024, Month: time.February}).Days(time.UTC); got != 29 {
		t.Errorf("February 2024 has %d days, want 29", got)
	}
	if got := m.String(); got != "2026-02" {
		t.Errorf("String = %q", got)
	}
	if got := m.Title(); got != "February 2026" {
		t.Errorf("Title = %q", got)
	}
	if got := m.Add(-2); got.Year != 2025 || got.Month != time.December {
		t.Errorf("Add(-2) = %+v, want 2025-12", got)
	}
	if got := m.Add(11); got.Year != 2027 || got.Month != time.January {
		t.Errorf("Add(11) = %+v, want 2027-01", got)
	}
	if _, err := ParseMonth("nope"); err == nil {
		t.Error("ParseMonth should reject garbage")
	}
}
