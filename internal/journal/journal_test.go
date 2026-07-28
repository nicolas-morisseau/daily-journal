package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 7, 28, 14, 30, 0, 0, time.UTC)

// newTestJournal builds a journal in a temporary directory with a frozen clock,
// pre-populated with the given entry slugs.
func newTestJournal(t *testing.T, slugs ...string) *Journal {
	t.Helper()
	dir := t.TempDir()
	j := New(dir, func() time.Time { return testNow })
	if err := j.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, slug := range slugs {
		body := "# " + slug + "\n\nSome words about #work here.\n"
		if err := os.WriteFile(filepath.Join(j.EntriesDir(), slug+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return j
}

func TestEntriesOrderAndClassification(t *testing.T) {
	j := newTestJournal(t, "2026-07-26", "2026-07-28", "2026-07-27", "not-a-date")

	entries, err := j.Entries()
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, e := range entries {
		slugs = append(slugs, e.Slug)
	}
	want := []string{"2026-07-26", "2026-07-27", "2026-07-28", "not-a-date"}
	if strings.Join(slugs, ",") != strings.Join(want, ",") {
		t.Errorf("Entries order = %v, want %v", slugs, want)
	}

	byslug := map[string]Entry{}
	for _, e := range entries {
		byslug[e.Slug] = e
	}
	// A day entry carries its date; any other markdown file in entries/ is
	// listed but is not a day.
	if !byslug["2026-07-28"].IsDated() {
		t.Error("2026-07-28 should be a dated entry")
	}
	if byslug["not-a-date"].IsDated() {
		t.Error("not-a-date should not count as a day")
	}

	newest, err := j.EntriesSorted(NewestFirst)
	if err != nil {
		t.Fatal(err)
	}
	if newest[0].Slug != "not-a-date" {
		t.Errorf("NewestFirst should reverse the list, got %s first", newest[0].Slug)
	}
}

func TestEntriesMissingDir(t *testing.T) {
	dir := t.TempDir()
	j := New(dir, func() time.Time { return testNow })
	if _, err := j.Entries(); err == nil {
		t.Fatal("expected an error when entries/ is missing")
	}
}

func TestLastSkipsNonDatedFiles(t *testing.T) {
	j := newTestJournal(t, "2026-07-27", "2026-07-28", "zzz-notes")
	last, err := j.Last()
	if err != nil {
		t.Fatal(err)
	}
	if last.Slug != "2026-07-28" {
		t.Errorf("Last = %s, want 2026-07-28", last.Slug)
	}
}

func TestSearch(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-27", "# 2026-07-27\n\nline one\nfound CONFIDENCE here\nline three\nline four\n")
	write(t, j, "2026-07-28", "# 2026-07-28\n\nnothing to see\n")

	result, err := j.Search(SearchOptions{Query: "confidence"})
	if err != nil {
		t.Fatal(err)
	}
	if result.EntryCount != 1 {
		t.Errorf("EntryCount = %d, want 1", result.EntryCount)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("Matches = %d, want 1", len(result.Matches))
	}
	match := result.Matches[0]
	if match.LineNumber != 4 {
		t.Errorf("LineNumber = %d, want 4", match.LineNumber)
	}
	if len(match.Before) != 2 || len(match.After) != 2 {
		t.Errorf("context = %d before / %d after, want 2/2", len(match.Before), len(match.After))
	}
	if len(match.Ranges) != 1 || match.Line[match.Ranges[0].Start:match.Ranges[0].End] != "CONFIDENCE" {
		t.Errorf("ranges do not point at the match: %+v", match.Ranges)
	}
}

func TestSearchMultipleHitsPerLine(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-28", "aa bb aa\n")
	result, err := j.Search(SearchOptions{Query: "aa"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 || len(result.Matches[0].Ranges) != 2 {
		t.Errorf("expected one line with two ranges, got %+v", result.Matches)
	}
}

// TestSearchWithTags: the query looks at the words, the tags look at the note as
// a whole, and adding a tag narrows the result.
func TestSearchWithTags(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-25", "# 2026-07-25\n\nmeeting about the roadmap\n")
	write(t, j, "2026-07-26", "# 2026-07-26\n\nroadmap review [#work]\n")
	write(t, j, "2026-07-27", "# 2026-07-27\n\nroadmap and #work and #urgent\n")
	write(t, j, "2026-07-28", "# 2026-07-28\n\nnothing about it, just #work\n")

	cases := []struct {
		name     string
		tags     []string
		entries  int
		searched int
	}{
		{"no tag searches everything", nil, 3, 4},
		{"one tag narrows it", []string{"work"}, 2, 3},
		{"a leading # is accepted", []string{"#work"}, 2, 3},
		{"tags are case-insensitive", []string{"WORK"}, 2, 3},
		{"two tags narrow further", []string{"work", "urgent"}, 1, 1},
		{"an unknown tag matches nothing", []string{"nope"}, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := j.Search(SearchOptions{Query: "roadmap", Tags: c.tags})
			if err != nil {
				t.Fatal(err)
			}
			if result.EntryCount != c.entries {
				t.Errorf("EntryCount = %d, want %d", result.EntryCount, c.entries)
			}
			if result.Searched != c.searched {
				t.Errorf("Searched = %d, want %d", result.Searched, c.searched)
			}
		})
	}
}

// TestSearchTagMatchingIsExact guards against #work matching #workshop.
func TestSearchTagMatchingIsExact(t *testing.T) {
	j := newTestJournal(t)
	write(t, j, "2026-07-27", "# 2026-07-27\n\nroadmap at the #workshop\n")
	write(t, j, "2026-07-28", "# 2026-07-28\n\nroadmap at #work\n")

	result, err := j.Search(SearchOptions{Query: "roadmap", Tags: []string{"work"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.EntryCount != 1 {
		t.Fatalf("EntryCount = %d, want 1", result.EntryCount)
	}
	if result.Matches[0].Entry.Slug != "2026-07-28" {
		t.Errorf("matched %s, want 2026-07-28", result.Matches[0].Entry.Slug)
	}
}

func TestSearchNeedsAQuery(t *testing.T) {
	j := newTestJournal(t, "2026-07-28")
	result, err := j.Search(SearchOptions{Tags: []string{"work"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 0 {
		t.Errorf("an empty query should match nothing, got %+v", result.Matches)
	}
}

func TestNormaliseTags(t *testing.T) {
	got := normaliseTags([]string{" #Work ", "URGENT", "#", "", "  "})
	if len(got) != 2 || got[0] != "work" || got[1] != "urgent" {
		t.Errorf("normaliseTags = %v, want [work urgent]", got)
	}
	if normaliseTags(nil) != nil {
		t.Error("no tags should stay nil")
	}
}

func TestWeekEntries(t *testing.T) {
	// The week of Tuesday 2026-07-28 runs Monday 27 to Sunday 2026-08-02.
	j := newTestJournal(t, "2026-07-26", "2026-07-27", "2026-07-28", "2026-08-02", "2026-08-03")
	entries, err := j.WeekEntries(j.Today())
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, e := range entries {
		slugs = append(slugs, e.Slug)
	}
	want := "2026-07-27,2026-07-28,2026-08-02"
	if strings.Join(slugs, ",") != want {
		t.Errorf("WeekEntries = %v, want %s", slugs, want)
	}
}

func TestWordCount(t *testing.T) {
	cases := map[string]int{
		"":                 0,
		"one":              1,
		"one two  three\n": 3,
		"\n\n  \n":         0,
		"a\tb\nc":          3,
	}
	for input, want := range cases {
		if got := WordCount(input); got != want {
			t.Errorf("WordCount(%q) = %d, want %d", input, got, want)
		}
	}
}

func write(t *testing.T, j *Journal, slug, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(j.EntriesDir(), slug+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
