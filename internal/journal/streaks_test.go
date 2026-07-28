package journal

import (
	"testing"
	"time"
)

// streakDates turns YYYY-MM-DD strings into the sorted dates the streak helpers
// expect.
func streakDates(t *testing.T, slugs ...string) []time.Time {
	t.Helper()
	days := make([]time.Time, 0, len(slugs))
	for _, slug := range slugs {
		day, err := time.Parse("2006-01-02", slug)
		if err != nil {
			t.Fatal(err)
		}
		days = append(days, day)
	}
	return days
}

func TestLongestStreak(t *testing.T) {
	cases := []struct {
		name  string
		slugs []string
		want  int
	}{
		{"a single day is a streak of one", []string{"2026-07-28"}, 1},
		{"consecutive days", []string{"2026-07-26", "2026-07-27", "2026-07-28"}, 3},
		{"a gap resets the run", []string{"2026-07-20", "2026-07-21", "2026-07-26", "2026-07-27"}, 2},
		{"the longest run wins wherever it sits", []string{
			"2026-07-01", "2026-07-02", "2026-07-03", "2026-07-10", "2026-07-11"}, 3},
		{"no consecutive days at all", []string{"2026-07-01", "2026-07-05", "2026-07-20"}, 1},
		{"a month boundary is not a gap", []string{"2026-07-31", "2026-08-01"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := longestStreak(streakDates(t, c.slugs...)); got != c.want {
				t.Errorf("longestStreak = %d, want %d", got, c.want)
			}
		})
	}
}

func TestCurrentStreak(t *testing.T) {
	today := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		slugs []string
		want  int
	}{
		{"a run ending today", []string{"2026-07-26", "2026-07-27", "2026-07-28"}, 3},
		{"a run ending yesterday still counts", []string{"2026-07-26", "2026-07-27"}, 2},
		{"a two-day gap breaks it", []string{"2026-07-24", "2026-07-25"}, 0},
		{"only today", []string{"2026-07-28"}, 1},
		{"an earlier run does not count", []string{
			"2026-07-01", "2026-07-02", "2026-07-03", "2026-07-28"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := currentStreak(streakDates(t, c.slugs...), today); got != c.want {
				t.Errorf("currentStreak = %d, want %d", got, c.want)
			}
		})
	}
}

func TestDayGap(t *testing.T) {
	from := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	if got := dayGap(from, to); got != 2 {
		t.Errorf("dayGap = %d, want 2", got)
	}
	if got := dayGap(to, from); got != -2 {
		t.Errorf("dayGap backwards = %d, want -2", got)
	}
}
