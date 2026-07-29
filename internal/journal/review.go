package journal

import (
	"regexp"
	"strings"
	"time"

	"daily-journal/internal/dateparse"
)

// Mood is one of the three buckets a sprint review sorts notes into, after the
// good/bad/mad retrospective: what worked, what did not, what got in the way.
type Mood int

const (
	Good Mood = iota
	Bad
	Mad
)

// Moods is the bucket order used everywhere, left to right.
var Moods = [3]Mood{Good, Bad, Mad}

// Tag is the tag word the mood is written with in an entry, without the `#`.
func (m Mood) Tag() string {
	switch m {
	case Good:
		return "good"
	case Bad:
		return "bad"
	case Mad:
		return "mad"
	}
	return ""
}

// Title is the mood's column heading.
func (m Mood) Title() string { return strings.ToUpper(m.Tag()) }

// ReviewItem is one tagged note pulled out of an entry.
type ReviewItem struct {
	Date time.Time
	// Time is the note's `HH:MM` stamp, empty for a note written without one.
	Time string
	// Text is the note body with the mood tags removed: they label the column.
	Text string
}

// ReviewColumn is one mood's notes, oldest first, so a column reads as the story
// of the period.
type ReviewColumn struct {
	Mood  Mood
	Items []ReviewItem
}

// Review is the good/bad/mad breakdown of a period.
type Review struct {
	// From and To bound the period, inclusive.
	From, To time.Time
	// Columns holds one column per mood, in Moods order.
	Columns [3]ReviewColumn
	Days    int
	// Entries is how many day entries the period holds, tagged or not, so an
	// empty review can be told apart from an empty fortnight.
	Entries int
}

// Total is how many tagged notes the review found.
func (r Review) Total() int {
	n := 0
	for _, column := range r.Columns {
		n += len(column.Items)
	}
	return n
}

// Column returns one mood's column.
func (r Review) Column(m Mood) ReviewColumn { return r.Columns[int(m)] }

// Period formats the review's window for a heading, e.g.
// "16 Jul → 29 Jul 2026".
func (r Review) Period() string {
	from := r.From.Format("2 Jan")
	if r.From.Year() != r.To.Year() {
		from = r.From.Format("2 Jan 2006")
	}
	return from + " → " + r.To.Format("2 Jan 2006")
}

// DefaultReviewWeeks is the rolling window a sprint review covers when nothing
// else is asked for: the last two weeks.
const DefaultReviewWeeks = 2

// ReviewOptions bounds the period a review covers.
//
// The zero value means the default: the two weeks ending today.
type ReviewOptions struct {
	// From is the first day of the period. Zero means the window is derived from
	// To, Weeks and Anchor.
	From time.Time
	// To is the last day of the period, or the day whose sprint to show when
	// Anchor is set. Zero means today.
	To time.Time
	// Weeks is the window length. Zero means DefaultReviewWeeks.
	Weeks int
	// Anchor is the first day of any one sprint. Set, the period is the sprint
	// that day belongs to, so the window has the same bounds whichever day it is
	// opened on. Zero leaves the window rolling: it ends on To.
	Anchor time.Time
}

// resolve fills a partial window in. An explicit From wins over everything:
// asking for a period is asking for that period, not for the sprint around it.
func (o ReviewOptions) resolve(today time.Time) (from, to time.Time) {
	to = today
	if !o.To.IsZero() {
		to = dateparse.Truncate(o.To)
	}
	weeks := o.Weeks
	if weeks <= 0 {
		weeks = DefaultReviewWeeks
	}
	days := weeks * 7

	switch {
	case !o.From.IsZero():
		from = dateparse.Truncate(o.From)
	case !o.Anchor.IsZero():
		from = sprintStart(dateparse.Truncate(o.Anchor), to, days)
		to = from.AddDate(0, 0, days-1)
	default:
		from = to.AddDate(0, 0, -(days - 1))
	}

	if from.After(to) {
		from, to = to, from
	}
	return from, to
}

// sprintStart is the first day of the sprint that day falls in, counting fixed
// runs of length days from anchor. It reaches back before the anchor too, so a
// sprint start recorded today still describes last month's sprints.
func sprintStart(anchor, day time.Time, length int) time.Time {
	offset := dayGap(anchor, day) % length
	if offset < 0 {
		offset += length
	}
	return day.AddDate(0, 0, -offset)
}

// Review collects the notes tagged #good, #bad and #mad over a period.
//
// A note carrying two of the tags shows up in both columns: the entry said both
// things, and dropping one of them would be editing the journal.
func (j *Journal) Review(opts ReviewOptions) (Review, error) {
	from, to := opts.resolve(j.Today())
	review := Review{From: from, To: to, Days: dayGap(from, to) + 1}
	for i, mood := range Moods {
		review.Columns[i].Mood = mood
	}

	entries, err := j.DatedEntries()
	if err != nil {
		return review, err
	}

	for _, entry := range entries {
		if entry.Date.Before(from) || entry.Date.After(to) {
			continue
		}
		content, readErr := Read(entry.Path)
		if readErr != nil {
			continue
		}
		review.Entries++

		for _, note := range splitNotes(content) {
			tags := collectTags(note.text)
			for i, mood := range Moods {
				if !tags[mood.Tag()] {
					continue
				}
				review.Columns[i].Items = append(review.Columns[i].Items, ReviewItem{
					Date: entry.Date,
					Time: note.time,
					Text: stripListMarker(stripMoodTags(note.text)),
				})
			}
		}
	}
	return review, nil
}

// note is one unit of an entry: a timestamped block, or a stray tagged line.
type note struct {
	// time is the `HH:MM` stamp, empty when the note has none.
	time string
	text string
}

var (
	// noteStartRe matches the head of a quick-capture note, `**09:12** - text`,
	// tolerating a missing bold marker or dash for hand-written entries.
	noteStartRe = regexp.MustCompile(`^\s*\*{0,2}(\d{1,2}:\d{2})\*{0,2}\s*[-–—:]?[ \t]*(.*)$`)
	// headingRe matches a markdown heading, which closes the block above it.
	headingRe = regexp.MustCompile(`^#{1,6}\s`)
	// moodTagRe matches a mood tag and whatever wraps it: `[#bad]`, `(#good)`,
	// `#mad.`. The trailing group is what keeps `#badge` out, and it is captured
	// so that removing the tag does not swallow the punctuation after it.
	moodTagRe = regexp.MustCompile(`[\[(]?#(?i:good|bad|mad)[)\]]?([^\p{L}\p{N}_/-]|$)`)
	// doubleSpaceRe closes the gap a removed tag leaves mid-sentence, while
	// leaving the indentation of a nested list alone.
	doubleSpaceRe = regexp.MustCompile(`(\S)[ \t]{2,}`)
)

// splitNotes cuts an entry into the notes it holds.
//
// A timestamp starts a note and everything up to the next timestamp or heading
// belongs to it, so a note edited into several lines stays one note. Outside any
// block, a line carrying a mood tag stands as a note of its own: entries written
// by hand do not always use the quick-capture shape.
func splitNotes(content string) []note {
	var notes []note
	var current *note

	flush := func() {
		if current == nil {
			return
		}
		if text := strings.TrimSpace(current.text); text != "" {
			current.text = text
			notes = append(notes, *current)
		}
		current = nil
	}

	for _, line := range strings.Split(content, "\n") {
		if m := noteStartRe.FindStringSubmatch(line); m != nil {
			flush()
			current = &note{time: m[1], text: m[2]}
			continue
		}
		if headingRe.MatchString(strings.TrimSpace(line)) {
			flush()
			continue
		}
		if current != nil {
			current.text += "\n" + line
			continue
		}
		if trimmed := strings.TrimSpace(line); moodTagRe.MatchString(trimmed) {
			notes = append(notes, note{text: trimmed})
		}
	}
	flush()
	return notes
}

// stripMoodTags removes the bucket tags from a note's text, tidying the spaces
// they leave behind. Other tags stay: they are part of what was written.
func stripMoodTags(text string) string {
	lines := strings.Split(moodTagRe.ReplaceAllString(text, "$1"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(doubleSpaceRe.ReplaceAllString(line, "$1 "), " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

var (
	listMarkerRe = regexp.MustCompile(`^\s*[-*+]\s+`)
	checkboxRe   = regexp.MustCompile(`^\[[ xX]\]`)
)

// stripListMarker drops the bullet a note was written as, since a review card is
// already one item. Nested lines keep theirs, and so does a task box: an unticked
// checkbox says something a bullet does not.
func stripListMarker(text string) string {
	head, rest, multiline := strings.Cut(text, "\n")
	trimmed := listMarkerRe.ReplaceAllString(head, "")
	if trimmed == head || checkboxRe.MatchString(trimmed) {
		return text
	}
	if multiline {
		return trimmed + "\n" + rest
	}
	return trimmed
}
