package journal

import (
	"strings"
	"time"

	"daily-journal/internal/dateparse"
)

// DefaultDailyDays is how far back the stand-up recap reaches: one day, so the
// window holds yesterday and today. It matches `git standup -d 1`, which is what
// the git half of the same view is asked for.
const DefaultDailyDays = 1

// DailyNote is one thing written in an entry: a timestamped quick capture, or a
// block of text that carries no time of its own.
type DailyNote struct {
	// Date is the entry the note lives in.
	Date time.Time
	// Time is the note's `HH:MM` stamp, empty for text written without one.
	Time string
	// Text is the note as written, markdown and tags included: a recap is meant
	// to show what the day holds, not a tidied version of it.
	Text string
}

// DailyDay is one day of the window.
type DailyDay struct {
	Date time.Time
	// HasEntry distinguishes a day nothing was written on from an empty entry.
	HasEntry bool
	// Words is the entry's word count.
	Words int
	// Notes are the day's notes, in the order they were written.
	Notes []DailyNote
}

// Daily is the journal side of the stand-up recap.
type Daily struct {
	// From and To bound the window, inclusive.
	From, To time.Time
	// Days holds one entry per day of the window, newest first, so the recap
	// reads from what just happened backwards.
	Days []DailyDay
}

// Notes counts every note in the window.
func (d Daily) Notes() int {
	n := 0
	for _, day := range d.Days {
		n += len(day.Notes)
	}
	return n
}

// Words counts every word in the window.
func (d Daily) Words() int {
	n := 0
	for _, day := range d.Days {
		n += day.Words
	}
	return n
}

// Entries is how many days of the window were written on.
func (d Daily) Entries() int {
	n := 0
	for _, day := range d.Days {
		if day.HasEntry {
			n++
		}
	}
	return n
}

// DailyOptions bounds the window a recap covers.
//
// The zero value means the default: yesterday and today.
type DailyOptions struct {
	// To is the last day of the window. Zero means today.
	To time.Time
	// Days is how many days back the window reaches from To, so 1 covers
	// yesterday and today. Zero means DefaultDailyDays.
	Days int
}

func (o DailyOptions) resolve(today time.Time) (from, to time.Time) {
	to = today
	if !o.To.IsZero() {
		to = dateparse.Truncate(o.To)
	}
	days := DefaultDailyDays
	if o.Days != 0 {
		days = max(o.Days, 0)
	}
	return to.AddDate(0, 0, -days), to
}

// Daily collects what was written over the last days, newest day first.
//
// Days without an entry are listed too: a stand-up that skipped them would hide
// the fact that nothing was written.
func (j *Journal) Daily(opts DailyOptions) Daily {
	from, to := opts.resolve(j.Today())
	daily := Daily{From: from, To: to}

	for day := to; !day.Before(from); day = day.AddDate(0, 0, -1) {
		entry := DailyDay{Date: day}
		if content, ok := j.EntryContent(day); ok {
			entry.HasEntry = true
			entry.Words = WordCount(content)
			for _, note := range splitEntry(content) {
				entry.Notes = append(entry.Notes, DailyNote{
					Date: day,
					Time: note.time,
					Text: note.text,
				})
			}
		}
		daily.Days = append(daily.Days, entry)
	}
	return daily
}

// splitEntry cuts an entry into the blocks it holds.
//
// It differs from splitNotes, which the sprint review uses, in keeping the text
// that carries no mood tag: a recap of the day has to show everything that was
// written, not only what was tagged. A timestamp opens a block, a heading closes
// one, and prose written outside any timestamp stands as a block of its own.
func splitEntry(content string) []note {
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
		if strings.TrimSpace(line) == "" {
			continue
		}
		current = &note{text: line}
	}
	flush()
	return notes
}
