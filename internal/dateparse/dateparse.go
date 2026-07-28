// Package dateparse turns the human date expressions daily-journal accepts on the
// command line ("yesterday", "3 days ago", "last friday", "2025-01-15") into
// concrete dates.
//
// The bash original delegated this to GNU `date -d` with a hand-written macOS
// fallback, which meant the accepted vocabulary differed per platform. This
// package implements one consistent grammar on every platform.
package dateparse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Layout is the canonical entry date format.
const Layout = "2006-01-02"

// Rule matches one shape of date expression. Rules are tried in order, so
// adding a new expression is a matter of appending to the table.
type Rule struct {
	// Name documents the rule in error messages and tests.
	Name string
	// Match reports whether the rule applies and, if so, resolves the date
	// relative to now.
	Match func(input string, now time.Time) (time.Time, bool)
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// relativeRe matches "3 days ago", "-2 weeks", "next 4 months" style offsets.
var relativeRe = regexp.MustCompile(`^(?:([+-]?\d+)\s*)?(day|days|week|weeks|month|months|year|years)(?:\s+(ago|from now|hence|later))?$`)

// acceptedLayouts deliberately excludes ambiguous numeric forms such as
// 01/02/2006, where day and month order cannot be inferred safely.
var acceptedLayouts = []string{
	"2006-01-02",
	"2006/01/02",
	"20060102",
	"Jan 2 2006",
	"Jan 2, 2006",
	"January 2 2006",
	"January 2, 2006",
	"2 Jan 2006",
	"2 January 2006",
}

// rules is the ordered grammar. Keep the most specific patterns first.
var rules = []Rule{
	{
		Name: "absolute",
		Match: func(input string, now time.Time) (time.Time, bool) {
			for _, layout := range acceptedLayouts {
				if t, err := time.ParseInLocation(layout, input, now.Location()); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
	{
		Name: "keyword",
		Match: func(input string, now time.Time) (time.Time, bool) {
			switch strings.ToLower(input) {
			case "today", "now":
				return now, true
			case "yesterday":
				return now.AddDate(0, 0, -1), true
			case "tomorrow":
				return now.AddDate(0, 0, 1), true
			}
			return time.Time{}, false
		},
	},
	{
		Name: "last/next unit",
		Match: func(input string, now time.Time) (time.Time, bool) {
			lower := strings.ToLower(input)
			sign := 0
			var rest string
			switch {
			case strings.HasPrefix(lower, "last "):
				sign, rest = -1, strings.TrimPrefix(lower, "last ")
			case strings.HasPrefix(lower, "next "):
				sign, rest = 1, strings.TrimPrefix(lower, "next ")
			default:
				return time.Time{}, false
			}
			rest = strings.TrimSpace(rest)

			if wd, ok := weekdays[rest]; ok {
				return nearestWeekday(now, wd, sign), true
			}
			switch rest {
			case "week":
				return now.AddDate(0, 0, 7*sign), true
			case "month":
				return now.AddDate(0, sign, 0), true
			case "year":
				return now.AddDate(sign, 0, 0), true
			case "day":
				return now.AddDate(0, 0, sign), true
			}
			return time.Time{}, false
		},
	},
	{
		Name: "bare weekday",
		Match: func(input string, now time.Time) (time.Time, bool) {
			wd, ok := weekdays[strings.ToLower(input)]
			if !ok {
				return time.Time{}, false
			}
			// A bare weekday means the most recent one, today included: the
			// useful reading when writing a journal.
			if now.Weekday() == wd {
				return now, true
			}
			return nearestWeekday(now, wd, -1), true
		},
	},
	{
		Name: "relative offset",
		Match: func(input string, now time.Time) (time.Time, bool) {
			m := relativeRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(input)))
			if m == nil {
				return time.Time{}, false
			}
			amount := 1
			if m[1] != "" {
				n, err := strconv.Atoi(m[1])
				if err != nil {
					return time.Time{}, false
				}
				amount = n
			}
			if m[3] == "ago" {
				amount = -amount
			}
			switch strings.TrimSuffix(m[2], "s") {
			case "day":
				return now.AddDate(0, 0, amount), true
			case "week":
				return now.AddDate(0, 0, 7*amount), true
			case "month":
				return now.AddDate(0, amount, 0), true
			case "year":
				return now.AddDate(amount, 0, 0), true
			}
			return time.Time{}, false
		},
	},
}

// Parse resolves input relative to now, returning a date truncated to midnight
// in now's location.
func Parse(input string, now time.Time) (time.Time, error) {
	trimmed := strings.Join(strings.Fields(input), " ")
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	for _, rule := range rules {
		if t, ok := rule.Match(trimmed, now); ok {
			return Truncate(t), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot understand date %q\n\nTry: 2025-01-15, yesterday, \"3 days ago\", \"last friday\"", input)
}

// ParseToString resolves input and formats it as YYYY-MM-DD.
func ParseToString(input string, now time.Time) (string, error) {
	t, err := Parse(input, now)
	if err != nil {
		return "", err
	}
	return t.Format(Layout), nil
}

// Truncate strips the time of day, keeping the location.
func Truncate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// nearestWeekday walks in the given direction (-1 back, +1 forward) to the
// closest strictly different day matching wd.
func nearestWeekday(now time.Time, wd time.Weekday, direction int) time.Time {
	if direction == 0 {
		direction = -1
	}
	t := now
	for i := 0; i < 7; i++ {
		t = t.AddDate(0, 0, direction)
		if t.Weekday() == wd {
			return t
		}
	}
	return now
}

// WeekStart returns the Monday of the week containing t. Monday is the first
// day of the week throughout daily-journal (`--week`, calendar headers).
func WeekStart(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Monday -> 0, Sunday -> 6
	return Truncate(t.AddDate(0, 0, -offset))
}

// ISOWeek formats the ISO 8601 week number, matching `date '+%V'`.
func ISOWeek(t time.Time) string {
	_, week := t.ISOWeek()
	return fmt.Sprintf("%02d", week)
}
