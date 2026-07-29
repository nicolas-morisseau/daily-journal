package journal

import (
	"testing"
	"time"
)

func TestInsightMonthAndWeek(t *testing.T) {
	j := newTestJournal(t)
	// The frozen clock is Tuesday 2026-07-28, so the week runs Mon 27 to Sun Aug 2.
	write(t, j, "2026-06-30", "# 2026-06-30\n\nprevious month entry\n")
	write(t, j, "2026-07-27", "# 2026-07-27\n\nmonday words\n")
	write(t, j, "2026-07-28", "# 2026-07-28\n\ntuesday has more words than monday\n")
	write(t, j, "2026-08-02", "# 2026-08-02\n\nsunday of the same week\n")

	in := j.Insight(j.Today(), InsightOptions{})

	if in.Month != (Month{Year: 2026, Month: time.July}) {
		t.Errorf("Month = %+v, want July 2026", in.Month)
	}
	// July holds two entries: the 27th and the 28th.
	if in.Entries != 2 {
		t.Errorf("Entries = %d, want 2", in.Entries)
	}
	// Two distinct days written, out of 31.
	if in.DaysWritten != 2 || in.DaysInMonth != 31 {
		t.Errorf("Days = %d/%d, want 2/31", in.DaysWritten, in.DaysInMonth)
	}
	if in.Words == 0 {
		t.Error("Words should count the month's entries")
	}

	// The week is Monday-first and spans the month boundary.
	if got := in.Week[0].Date.Format("2006-01-02"); got != "2026-07-27" {
		t.Errorf("week starts %s, want 2026-07-27", got)
	}
	if got := in.Week[6].Date.Format("2006-01-02"); got != "2026-08-02" {
		t.Errorf("week ends %s, want 2026-08-02", got)
	}
	if !in.Week[0].HasEntry || !in.Week[1].HasEntry || !in.Week[6].HasEntry {
		t.Errorf("Mon, Tue and Sun should be marked: %+v", in.Week)
	}
	if in.Week[2].HasEntry {
		t.Error("Wednesday has no entry")
	}
	if in.Week[1].Words <= in.Week[0].Words {
		t.Errorf("Tuesday (%d words) should out-total Monday (%d)", in.Week[1].Words, in.Week[0].Words)
	}
	if in.MaxWeekWords() != in.Week[1].Words {
		t.Errorf("MaxWeekWords = %d, want Tuesday's %d", in.MaxWeekWords(), in.Week[1].Words)
	}
}

func TestInsightStreaks(t *testing.T) {
	j := newTestJournal(t, "2026-07-26", "2026-07-27", "2026-07-28")
	in := j.Insight(j.Today(), InsightOptions{})
	if in.CurrentStreak != 3 || in.LongestStreak != 3 {
		t.Errorf("streaks = %d current, %d longest, want 3 and 3", in.CurrentStreak, in.LongestStreak)
	}

	// A stale journal keeps its record but has no current streak.
	stale := newTestJournal(t, "2026-07-20", "2026-07-21", "2026-07-22")
	in = stale.Insight(stale.Today(), InsightOptions{})
	if in.CurrentStreak != 0 {
		t.Errorf("CurrentStreak = %d, want 0", in.CurrentStreak)
	}
	if in.LongestStreak != 3 {
		t.Errorf("LongestStreak = %d, want 3", in.LongestStreak)
	}
}

func TestInsightTags(t *testing.T) {
	j := newTestJournal(t)
	// #work in three entries, #home in one, and a repeat that must not inflate.
	write(t, j, "2026-07-20", "# 2026-07-20\n\n#work #work #work again\n")
	write(t, j, "2026-07-21", "# 2026-07-21\n\nsomething [#work] in brackets\n")
	write(t, j, "2026-07-22", "# 2026-07-22\n\n#work. with punctuation and #home\n")
	write(t, j, "2026-07-23", "## A heading\n\n#1234 is an issue, not a tag\n")

	in := j.Insight(j.Today(), InsightOptions{})
	if len(in.Tags) != 2 {
		t.Fatalf("Tags = %+v, want work and home only", in.Tags)
	}
	if in.Tags[0].Tag != "work" || in.Tags[0].Count != 3 {
		t.Errorf("first tag = %+v, want work counted in 3 entries", in.Tags[0])
	}
	if in.Tags[1].Tag != "home" || in.Tags[1].Count != 1 {
		t.Errorf("second tag = %+v, want home once", in.Tags[1])
	}
	if in.MaxTagCount() != 3 {
		t.Errorf("MaxTagCount = %d, want 3", in.MaxTagCount())
	}
}

func TestInsightTagLimit(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-20", "#a #b #c #d #e #f #g\n")

	if got := len(j.Insight(j.Today(), InsightOptions{}).Tags); got != DefaultTagLimit {
		t.Errorf("Tags = %d, want the default limit of %d", got, DefaultTagLimit)
	}
	if got := len(j.Insight(j.Today(), InsightOptions{TagLimit: 2}).Tags); got != 2 {
		t.Errorf("Tags = %d, want 2", got)
	}
}

func TestCollectTags(t *testing.T) {
	cases := map[string][]string{
		"#work":                   {"work"},
		"[#bad] and (#good)":      {"bad", "good"},
		"#work. #home, #life!":    {"work", "home", "life"},
		"# 2026-07-28":            nil, // a markdown heading
		"## Notes":                nil,
		"#1234":                   nil, // an issue reference
		"#deep/nested #with-dash": {"deep/nested", "with-dash"},
		"#Work and #work are one": {"work"},
		"no tags at all":          nil,
	}
	for content, want := range cases {
		got := collectTags(content)
		if len(got) != len(want) {
			t.Errorf("collectTags(%q) = %v, want %v", content, keysOf(got), want)
			continue
		}
		for _, tag := range want {
			if !got[tag] {
				t.Errorf("collectTags(%q) missing %q, got %v", content, tag, keysOf(got))
			}
		}
	}
}

func TestInsightOnEmptyJournal(t *testing.T) {
	dir := t.TempDir()
	j := New(dir, func() time.Time { return testNow })
	// No entries directory at all: the panel must still render something sane.
	in := j.Insight(j.Today(), InsightOptions{})
	if in.Entries != 0 || in.Words != 0 || len(in.Tags) != 0 {
		t.Errorf("empty journal insight = %+v", in)
	}
	if in.DaysInMonth != 31 {
		t.Errorf("DaysInMonth = %d, want 31 even with no entries", in.DaysInMonth)
	}
	if in.MaxWeekWords() != 0 || in.MaxTagCount() != 0 {
		t.Error("maxima should be zero on an empty journal")
	}
}

func TestEntryContent(t *testing.T) {
	j := newTestJournal(t, "2026-07-28")
	if content, ok := j.EntryContent(j.Today()); !ok || content == "" {
		t.Errorf("EntryContent = (%q, %v), want the entry", content, ok)
	}
	if _, ok := j.EntryContent(j.Today().AddDate(0, 0, 1)); ok {
		t.Error("a missing day should report no entry")
	}
}

// TestDayGapCountsCalendarDays covers the daylight-saving trap: a 23-hour day is
// still a day, and counting hours used to lose it — breaking both streaks and the
// sprint boundaries that are counted in days.
func TestDayGapCountsCalendarDays(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skipf("no timezone database: %v", err)
	}
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, paris)
	}

	cases := []struct {
		name     string
		from, to time.Time
		want     int
	}{
		// Paris springs forward on Sunday 2026-03-29 and falls back on 2026-10-25.
		{"across the short day", day(2026, time.March, 29), day(2026, time.March, 30), 1},
		{"fortnight across it", day(2026, time.March, 23), day(2026, time.April, 6), 14},
		{"across the long day", day(2026, time.October, 25), day(2026, time.October, 26), 1},
		{"fortnight across that", day(2026, time.October, 19), day(2026, time.November, 2), 14},
		{"same day", day(2026, time.July, 28), day(2026, time.July, 28), 0},
		{"backwards", day(2026, time.April, 6), day(2026, time.March, 23), -14},
	}
	for _, c := range cases {
		if got := dayGap(c.from, c.to); got != c.want {
			t.Errorf("%s: dayGap = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestInsightStreakAcrossDaylightSaving is the same trap seen from the feature
// that has always depended on it.
func TestInsightStreakAcrossDaylightSaving(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skipf("no timezone database: %v", err)
	}
	// The clocks go forward on the 29th, so the 29th is 23 hours long.
	now := time.Date(2026, 3, 30, 14, 0, 0, 0, paris)
	dir := t.TempDir()
	j := New(dir, func() time.Time { return now })
	if err := j.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"2026-03-28", "2026-03-29", "2026-03-30"} {
		write(t, j, slug, "# "+slug+"\n\nwrote something\n")
	}

	in := j.Insight(j.Today(), InsightOptions{})
	if in.CurrentStreak != 3 || in.LongestStreak != 3 {
		t.Errorf("streaks = %d current, %d longest, want 3 and 3 across the clock change",
			in.CurrentStreak, in.LongestStreak)
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
