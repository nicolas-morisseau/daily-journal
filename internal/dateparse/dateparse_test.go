package dateparse

import (
	"testing"
	"time"
)

// reference is a Tuesday, which makes weekday arithmetic easy to read.
var reference = time.Date(2026, 7, 28, 14, 30, 0, 0, time.UTC)

func TestParse(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2026-07-28", "2026-07-28"},
		{"2026/07/28", "2026-07-28"},
		{"20260728", "2026-07-28"},
		{"Jan 15 2025", "2025-01-15"},
		{"15 January 2025", "2025-01-15"},
		{"today", "2026-07-28"},
		{"yesterday", "2026-07-27"},
		{"tomorrow", "2026-07-29"},
		{"3 days ago", "2026-07-25"},
		{"1 day ago", "2026-07-27"},
		{"2 weeks ago", "2026-07-14"},
		{"1 month ago", "2026-06-28"},
		{"1 year ago", "2025-07-28"},
		{"-3 days", "2026-07-25"},
		{"+2 days", "2026-07-30"},
		{"3 days from now", "2026-07-31"},
		{"last week", "2026-07-21"},
		{"next week", "2026-08-04"},
		{"last month", "2026-06-28"},
		// From a Tuesday, "last friday" is the closest Friday in the past.
		{"last friday", "2026-07-24"},
		{"last Friday", "2026-07-24"},
		{"last monday", "2026-07-27"},
		{"last tuesday", "2026-07-21"},
		{"next monday", "2026-08-03"},
		// A bare weekday keeps today when it matches.
		{"tuesday", "2026-07-28"},
		{"friday", "2026-07-24"},
		// Extra whitespace is tolerated.
		{"  3   days   ago ", "2026-07-25"},
	}

	for _, c := range cases {
		got, err := ParseToString(c.input, reference)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.input, got, c.want)
		}
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, input := range []string{"", "not a date", "2026-13-45", "next fortnight"} {
		if got, err := ParseToString(input, reference); err == nil {
			t.Errorf("Parse(%q) = %s, want an error", input, got)
		}
	}
}

func TestParseTruncatesTime(t *testing.T) {
	got, err := Parse("today", reference)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("Parse should return midnight, got %s", got)
	}
}

func TestWeekStart(t *testing.T) {
	cases := map[string]string{
		"2026-07-27": "2026-07-27", // Monday stays put
		"2026-07-28": "2026-07-27", // Tuesday
		"2026-08-02": "2026-07-27", // Sunday belongs to the week that began Monday
	}
	for input, want := range cases {
		day, err := time.Parse(Layout, input)
		if err != nil {
			t.Fatal(err)
		}
		if got := WeekStart(day).Format(Layout); got != want {
			t.Errorf("WeekStart(%s) = %s, want %s", input, got, want)
		}
	}
}

func TestISOWeek(t *testing.T) {
	day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	if got := ISOWeek(day); got != "02" {
		t.Errorf("ISOWeek = %q, want 02", got)
	}
}
