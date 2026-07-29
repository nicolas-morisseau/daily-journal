package journal

import (
	"strings"
	"testing"
	"time"
)

// items renders a column as "date time|text" rows, which is compact enough to
// assert on directly.
func items(t *testing.T, review Review, mood Mood) []string {
	t.Helper()
	var out []string
	for _, item := range review.Column(mood).Items {
		out = append(out, item.Date.Format("2006-01-02")+" "+item.Time+"|"+item.Text)
	}
	return out
}

// TestReviewDefaultWindow is the behaviour the command exists for: the two weeks
// ending today, and nothing on either side of them.
func TestReviewDefaultWindow(t *testing.T) {
	// The frozen clock is 2026-07-28, so the window runs 2026-07-15 to 07-28.
	j := newTestJournal(t)
	write(t, j, "2026-07-14", "# 2026-07-14\n\n**09:00** - the day before the window [#good]\n")
	write(t, j, "2026-07-15", "# 2026-07-15\n\n**09:00** - first day in [#good]\n")
	write(t, j, "2026-07-28", "# 2026-07-28\n\n**09:00** - last day in [#bad]\n")
	write(t, j, "2026-07-29", "# 2026-07-29\n\n**09:00** - tomorrow [#mad]\n")

	review, err := j.Review(ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := review.From.Format("2006-01-02"); got != "2026-07-15" {
		t.Errorf("From = %s, want 2026-07-15", got)
	}
	if got := review.To.Format("2006-01-02"); got != "2026-07-28" {
		t.Errorf("To = %s, want 2026-07-28", got)
	}
	if review.Days != 14 {
		t.Errorf("Days = %d, want 14", review.Days)
	}
	if review.Entries != 2 {
		t.Errorf("Entries = %d, want the 2 inside the window", review.Entries)
	}

	if got := items(t, review, Good); len(got) != 1 || !strings.Contains(got[0], "first day in") {
		t.Errorf("good = %v, want only the 15th", got)
	}
	if got := items(t, review, Bad); len(got) != 1 || !strings.Contains(got[0], "last day in") {
		t.Errorf("bad = %v, want only the 28th", got)
	}
	if got := items(t, review, Mad); len(got) != 0 {
		t.Errorf("mad = %v, want nothing: tomorrow is outside the window", got)
	}
}

func TestReviewExplicitPeriod(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-06-10", "**09:00** - june [#good]\n")
	write(t, j, "2026-07-20", "**09:00** - july [#good]\n")

	loc := testNow.Location()
	june := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, 6, 30, 0, 0, 0, 0, loc)

	review, err := j.Review(ReviewOptions{From: june, To: end})
	if err != nil {
		t.Fatal(err)
	}
	if review.Days != 30 {
		t.Errorf("Days = %d, want 30", review.Days)
	}
	if got := items(t, review, Good); len(got) != 1 || !strings.Contains(got[0], "june") {
		t.Errorf("good = %v, want only the June note", got)
	}

	// A window given backwards is still a window.
	if flipped, _ := j.Review(ReviewOptions{From: end, To: june}); flipped.Days != 30 {
		t.Errorf("reversed window Days = %d, want 30", flipped.Days)
	}

	// --weeks counts back from --to, not from today.
	weeks, err := j.Review(ReviewOptions{To: end, Weeks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := weeks.From.Format("2006-01-02"); got != "2026-06-24" {
		t.Errorf("From = %s, want 2026-06-24", got)
	}
}

// TestReviewNotesAndOrder covers what a note is: a timestamped block, whatever
// its shape, kept in the order it was written.
func TestReviewNotesAndOrder(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-20", strings.Join([]string{
		"# 2026-07-20",
		"",
		"**09:00** - [#good] shipped the migration",
		"",
		"**10:30** - a note with no tag at all",
		"",
		"**11:00** - lost in the MRs [#bad]",
		"  and it took the whole morning",
		"",
	}, "\n"))
	write(t, j, "2026-07-21", "# 2026-07-21\n\n**08:00** - the day after [#good]\n")

	review, err := j.Review(ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}

	good := items(t, review, Good)
	want := []string{
		"2026-07-20 09:00|shipped the migration",
		"2026-07-21 08:00|the day after",
	}
	if strings.Join(good, ";") != strings.Join(want, ";") {
		t.Errorf("good = %v, want %v (oldest first, tags stripped)", good, want)
	}

	bad := items(t, review, Bad)
	if len(bad) != 1 {
		t.Fatalf("bad = %v, want one note", bad)
	}
	if !strings.HasPrefix(bad[0], "2026-07-20 11:00|lost in the MRs") {
		t.Errorf("bad note = %q, want the 11:00 one", bad[0])
	}
	// A note continued on the next line stays one note.
	if !strings.Contains(bad[0], "took the whole morning") {
		t.Errorf("bad note = %q, should keep its continuation line", bad[0])
	}
}

// TestReviewUntimestampedAndDoubleTagged covers hand-written entries: a tagged
// line outside any timestamped note still counts, and a note saying two things
// belongs in both columns.
func TestReviewUntimestampedAndDoubleTagged(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-20", strings.Join([]string{
		"# 2026-07-20",
		"",
		"- deployed twice [#good]",
		"- nothing to report here",
		"",
		"## Retro",
		"",
		"the release went out [#good] but the pipeline is still red [#bad]",
		"",
	}, "\n"))

	review, err := j.Review(ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}

	good := items(t, review, Good)
	if len(good) != 2 {
		t.Fatalf("good = %v, want the bullet and the retro line", good)
	}
	// The card is the bullet, so the marker goes.
	if good[0] != "2026-07-20 |deployed twice" {
		t.Errorf("first good = %q, want the untimestamped bullet", good[0])
	}
	bad := items(t, review, Bad)
	if len(bad) != 1 {
		t.Fatalf("bad = %v, want the retro line", bad)
	}
	// The same sentence, in both columns, with both tags out of the way.
	if !strings.Contains(bad[0], "release went out but the pipeline is still red") {
		t.Errorf("bad note = %q, want both tags stripped", bad[0])
	}
	if review.Total() != 3 {
		t.Errorf("Total = %d, want 3", review.Total())
	}
}

func TestReviewTagMatchingIsExact(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-20", strings.Join([]string{
		"**09:00** - a #badge is not a mood",
		"**10:00** - neither is #goodwill",
		"**11:00** - nor #madness",
		"**12:00** - but #GOOD is, whatever the case",
		"",
	}, "\n"))

	review, err := j.Review(ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := items(t, review, Good); len(got) != 1 || !strings.Contains(got[0], "whatever the case") {
		t.Errorf("good = %v, want only the #GOOD note", got)
	}
	if review.Total() != 1 {
		t.Errorf("Total = %d, want 1: the near-misses are not moods", review.Total())
	}
}

// TestReviewSprintAnchor is why the anchor exists: the window must not depend on
// the day the review is opened on.
func TestReviewSprintAnchor(t *testing.T) {
	j := newTestJournal(t)
	loc := testNow.Location()
	// Sprints start on Monday 2026-07-06, so they run 06-19, 20-02, 03-16.
	anchor := time.Date(2026, 7, 6, 0, 0, 0, 0, loc)

	// The frozen clock is Tuesday 2026-07-28, in the sprint of 20 July.
	review, err := j.Review(ReviewOptions{Anchor: anchor})
	if err != nil {
		t.Fatal(err)
	}
	if got := review.Period(); got != "20 Jul → 2 Aug 2026" {
		t.Errorf("Period = %q, want the sprint of 20 July", got)
	}
	if review.Days != 14 {
		t.Errorf("Days = %d, want 14", review.Days)
	}

	// Every day of that sprint gives the same window, which is the whole point.
	for _, day := range []string{"2026-07-20", "2026-07-25", "2026-07-28", "2026-08-02"} {
		to, err := time.ParseInLocation("2006-01-02", day, loc)
		if err != nil {
			t.Fatal(err)
		}
		got, err := j.Review(ReviewOptions{Anchor: anchor, To: to})
		if err != nil {
			t.Fatal(err)
		}
		if got.Period() != review.Period() {
			t.Errorf("opened on %s the period is %q, want %q", day, got.Period(), review.Period())
		}
	}
	// The day after is the next sprint, not a window shifted by one.
	next, err := j.Review(ReviewOptions{Anchor: anchor, To: time.Date(2026, 8, 3, 0, 0, 0, 0, loc)})
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Period(); got != "3 Aug → 16 Aug 2026" {
		t.Errorf("the sprint after is %q, want 3 Aug → 16 Aug 2026", got)
	}
}

func TestReviewSprintAnchorEdges(t *testing.T) {
	j := newTestJournal(t)
	loc := testNow.Location()

	cases := []struct {
		name   string
		anchor string
		weeks  int
		want   string
	}{
		// Counting back from an anchor in the future works the same way, so a
		// sprint start written down today still describes last month.
		{"anchor ahead of today", "2026-08-03", 0, "20 Jul → 2 Aug 2026"},
		{"anchor is today", "2026-07-28", 0, "28 Jul → 10 Aug 2026"},
		{"anchor long past", "2025-01-06", 0, "20 Jul → 2 Aug 2026"},
		// --weeks is the sprint length, so it changes where the bounds fall.
		{"three-week sprints", "2026-07-06", 3, "27 Jul → 16 Aug 2026"},
		{"one-week sprints", "2026-07-06", 1, "27 Jul → 2 Aug 2026"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			anchor, err := time.ParseInLocation("2006-01-02", c.anchor, loc)
			if err != nil {
				t.Fatal(err)
			}
			review, err := j.Review(ReviewOptions{Anchor: anchor, Weeks: c.weeks})
			if err != nil {
				t.Fatal(err)
			}
			if got := review.Period(); got != c.want {
				t.Errorf("Period = %q, want %q", got, c.want)
			}
		})
	}
}

// TestReviewExplicitPeriodBeatsTheAnchor: asking for a period is asking for that
// period, not for the sprint around it.
func TestReviewExplicitPeriodBeatsTheAnchor(t *testing.T) {
	j := newTestJournal(t)
	loc := testNow.Location()
	anchor := time.Date(2026, 7, 6, 0, 0, 0, 0, loc)

	review, err := j.Review(ReviewOptions{
		Anchor: anchor,
		From:   time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		To:     time.Date(2026, 7, 3, 0, 0, 0, 0, loc),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := review.Period(); got != "1 Jul → 3 Jul 2026" {
		t.Errorf("Period = %q, want the window that was asked for", got)
	}
	if review.Days != 3 {
		t.Errorf("Days = %d, want 3", review.Days)
	}
}

func TestSprintStart(t *testing.T) {
	loc := testNow.Location()
	day := func(s string) time.Time {
		t.Helper()
		parsed, err := time.ParseInLocation("2006-01-02", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}

	anchor := day("2026-07-06")
	cases := map[string]string{
		"2026-07-06": "2026-07-06", // the anchor itself opens a sprint
		"2026-07-19": "2026-07-06", // its last day
		"2026-07-20": "2026-07-20", // the next one
		"2026-07-05": "2026-06-22", // the sprint before the anchor
		"2026-06-22": "2026-06-22",
	}
	for input, want := range cases {
		if got := sprintStart(anchor, day(input), 14).Format("2006-01-02"); got != want {
			t.Errorf("sprintStart(%s) = %s, want %s", input, got, want)
		}
	}
}

func TestStripListMarker(t *testing.T) {
	cases := map[string]string{
		"- deployed twice":      "deployed twice",
		"* deployed twice":      "deployed twice",
		"+ deployed twice":      "deployed twice",
		"  - indented":          "indented",
		"deployed twice":        "deployed twice",
		"- [ ] not done yet":    "- [ ] not done yet",
		"- [x] done":            "- [x] done",
		"- one\n  - nested two": "one\n  - nested two",
		"-no space, no bullet":  "-no space, no bullet",
	}
	for input, want := range cases {
		if got := stripListMarker(input); got != want {
			t.Errorf("stripListMarker(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStripMoodTags(t *testing.T) {
	cases := map[string]string{
		"[#good] shipped it":            "shipped it",
		"shipped it [#good]":            "shipped it",
		"shipped [#good] it":            "shipped it",
		"shipped it (#bad)":             "shipped it",
		"#mad the printer again":        "the printer again",
		"a #badge stays":                "a #badge stays",
		"#work stays, #good goes":       "#work stays, goes",
		"[#good] and [#bad] both leave": "and both leave",
	}
	for input, want := range cases {
		if got := stripMoodTags(input); got != want {
			t.Errorf("stripMoodTags(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReviewOnEmptyJournal(t *testing.T) {
	dir := t.TempDir()
	j := New(dir, func() time.Time { return testNow })

	// No entries directory at all: the view must still get a usable period.
	review, err := j.Review(ReviewOptions{})
	if err == nil {
		t.Error("a missing entries directory should be reported")
	}
	if review.Days != 14 || review.Total() != 0 {
		t.Errorf("review = %d days, %d notes, want 14 and 0", review.Days, review.Total())
	}
	for i, mood := range Moods {
		if review.Columns[i].Mood != mood {
			t.Errorf("column %d = %v, want %v", i, review.Columns[i].Mood, mood)
		}
	}
}

func TestReviewPeriodLabel(t *testing.T) {
	loc := testNow.Location()
	same := Review{
		From: time.Date(2026, 7, 15, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 7, 28, 0, 0, 0, 0, loc),
	}
	if got := same.Period(); got != "15 Jul → 28 Jul 2026" {
		t.Errorf("Period = %q", got)
	}
	across := Review{
		From: time.Date(2025, 12, 29, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 1, 11, 0, 0, 0, 0, loc),
	}
	if got := across.Period(); got != "29 Dec 2025 → 11 Jan 2026" {
		t.Errorf("Period across years = %q", got)
	}
}
