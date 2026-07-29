package journal

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"daily-journal/internal/dateparse"
)

// TagCount is how often a tag appears across the journal.
type TagCount struct {
	Tag   string
	Count int
}

// DayActivity is one day's writing, used for the weekly bars.
type DayActivity struct {
	Date time.Time
	// Words is 0 when nothing was written that day.
	Words int
	// HasEntry distinguishes an empty entry from a missing one.
	HasEntry bool
}

// Insight is the summary shown beside the calendar: the selected month, the
// selected week, streaks and the most used tags.
type Insight struct {
	Month Month
	// Entries and Words cover the month.
	Entries int
	Words   int
	// DaysWritten is how many days of the month have an entry, out of DaysInMonth.
	DaysWritten int
	DaysInMonth int

	CurrentStreak int
	LongestStreak int

	// Week holds Monday through Sunday of the week containing the selected day.
	Week [7]DayActivity
	// Tags are the most frequent tags, most frequent first.
	Tags []TagCount
}

// MaxWeekWords is the busiest day of the week, for scaling the bars.
func (i Insight) MaxWeekWords() int {
	most := 0
	for _, day := range i.Week {
		if day.Words > most {
			most = day.Words
		}
	}
	return most
}

// MaxTagCount is the most frequent tag's count, for scaling the bars.
func (i Insight) MaxTagCount() int {
	if len(i.Tags) == 0 {
		return 0
	}
	return i.Tags[0].Count
}

// InsightOptions tunes what Insight computes.
type InsightOptions struct {
	// TagLimit caps how many tags are reported. Zero means DefaultTagLimit.
	TagLimit int
}

// DefaultTagLimit is how many tags the side panel has room for.
const DefaultTagLimit = 5

// Insight summarises the journal around one day in a single pass over the
// entries, so the calendar view can refresh it on every keystroke.
func (j *Journal) Insight(day time.Time, opts InsightOptions) Insight {
	if opts.TagLimit <= 0 {
		opts.TagLimit = DefaultTagLimit
	}
	day = dateparse.Truncate(day)
	month := MonthOf(day)
	loc := j.Now().Location()

	insight := Insight{Month: month, DaysInMonth: month.Days(loc)}

	weekStart := dateparse.WeekStart(day)
	for i := range insight.Week {
		insight.Week[i].Date = weekStart.AddDate(0, 0, i)
	}

	entries, err := j.Entries()
	if err != nil {
		return insight
	}

	tags := map[string]int{}
	monthDays := map[string]bool{}
	var streakDays []time.Time

	for _, entry := range entries {
		content, readErr := Read(entry.Path)
		if readErr != nil {
			continue
		}
		words := WordCount(content)

		for tag := range collectTags(content) {
			tags[tag]++
		}
		if entry.IsDated() {
			streakDays = append(streakDays, entry.Date)
		}
		if entry.Date.IsZero() {
			continue
		}

		if MonthOf(entry.Date) == month {
			insight.Entries++
			insight.Words += words
			monthDays[entry.Date.Format(dateparse.Layout)] = true
		}
		// Several entries can share a day, so accumulate rather than assign.
		for i := range insight.Week {
			if insight.Week[i].Date.Equal(entry.Date) {
				insight.Week[i].HasEntry = true
				insight.Week[i].Words += words
			}
		}
	}

	insight.DaysWritten = len(monthDays)
	if len(streakDays) > 0 {
		insight.LongestStreak = longestStreak(streakDays)
		insight.CurrentStreak = currentStreak(streakDays, j.Today())
	}
	insight.Tags = rankTags(tags, opts.TagLimit)
	return insight
}

// tagRe finds `#tag` anywhere in the text, including inside brackets such as
// `[#bad]` and before punctuation such as `#work.`. Requiring a tag character
// straight after the `#` keeps markdown headings (`# 2026-07-28`, `## Notes`)
// out of the results.
var tagRe = regexp.MustCompile(`#([\p{L}\p{N}][\p{L}\p{N}_/-]*)`)

// collectTags returns the distinct tags in an entry. A tag counts once per
// entry, so a note repeating #work does not dominate the ranking.
func collectTags(content string) map[string]bool {
	found := map[string]bool{}
	for _, match := range tagRe.FindAllStringSubmatch(content, -1) {
		tag := strings.Trim(match[1], "-_/")
		if tag == "" || !hasLetter(tag) {
			continue
		}
		found[strings.ToLower(tag)] = true
	}
	return found
}

// hasLetter rejects purely numeric references such as `#1234`, which are issue
// numbers far more often than tags.
func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// rankTags sorts tags by count then name, and keeps the first limit of them.
func rankTags(counts map[string]int, limit int) []TagCount {
	ranked := make([]TagCount, 0, len(counts))
	for tag, count := range counts {
		ranked = append(ranked, TagCount{Tag: tag, Count: count})
	}
	sort.Slice(ranked, func(a, b int) bool {
		if ranked[a].Count != ranked[b].Count {
			return ranked[a].Count > ranked[b].Count
		}
		return ranked[a].Tag < ranked[b].Tag
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// dayGap returns the number of calendar days between two midnight-truncated
// dates. It compares them as calendar days rather than as instants, because a day
// that daylight saving makes 23 hours long still counts as one.
func dayGap(from, to time.Time) int {
	const day = 24 * time.Hour
	utcFrom := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	utcTo := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(utcTo.Sub(utcFrom) / day)
}

// longestStreak is the longest run of consecutive calendar days in a sorted list
// of entry dates. A single entry counts as a streak of one.
func longestStreak(days []time.Time) int {
	longest, run := 1, 1
	for i := 1; i < len(days); i++ {
		if dayGap(days[i-1], days[i]) == 1 {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 1
		}
	}
	return longest
}

// currentStreak is the run of consecutive days ending today or yesterday. A gap
// of more than one day means the streak is broken, hence zero.
func currentStreak(days []time.Time, today time.Time) int {
	last := days[len(days)-1]
	if dayGap(last, today) > 1 {
		return 0
	}
	current := 1
	for i := len(days) - 2; i >= 0; i-- {
		if dayGap(days[i], days[i+1]) != 1 {
			break
		}
		current++
	}
	return current
}

// EntryContent returns a day's markdown and whether the entry exists.
func (j *Journal) EntryContent(day time.Time) (string, bool) {
	path := j.PathFor(day.Format(dateparse.Layout))
	content, err := Read(path)
	if err != nil {
		return "", false
	}
	return content, true
}
