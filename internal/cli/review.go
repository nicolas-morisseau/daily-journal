package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"daily-journal/internal/config"
	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
	"daily-journal/internal/ui"
)

// reviewWindow is what `journal review` was asked to cover.
type reviewWindow struct {
	// Weeks is the rolling window length; empty means the default.
	Weeks string
	// From and To are date expressions; empty means the rolling window.
	From, To string
}

// sprintAnchor parses SPRINT_START, the first day of any one sprint, which turns
// the review's rolling window into fixed sprint boundaries. The format is
// deliberately strict: an anchor that moved with the day would defeat the point
// of writing it down.
func (a *App) sprintAnchor() (time.Time, error) {
	raw := strings.TrimSpace(a.Config.SprintStart)
	if raw == "" {
		return time.Time{}, nil
	}
	day, err := time.ParseInLocation(dateparse.Layout, raw, a.Now().Location())
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid %s %q in %s\n\nIt takes one date in YYYY-MM-DD form, the first day of any one\nsprint, for example %s=2026-07-06",
			config.KeySprintStart, raw, a.Config.Path, config.KeySprintStart)
	}
	return day, nil
}

// options resolves the window into review options, reporting a usage error when
// a date or a count cannot be read.
func (w reviewWindow) options(now time.Time) (journal.ReviewOptions, error) {
	opts := journal.ReviewOptions{}

	if w.Weeks != "" {
		weeks, err := strconv.Atoi(w.Weeks)
		if err != nil || weeks < 1 {
			return opts, usagef("--weeks takes a positive number of weeks, got %q", w.Weeks)
		}
		opts.Weeks = weeks
	}
	for _, field := range []struct {
		value string
		into  *time.Time
	}{{w.From, &opts.From}, {w.To, &opts.To}} {
		if field.value == "" {
			continue
		}
		date, err := dateparse.Parse(field.value, now)
		if err != nil {
			return opts, usagef("%s", err.Error())
		}
		*field.into = date
	}
	return opts, nil
}

// review backs `journal review`.
//
// On a terminal it runs the full-screen three-column view; piped, it prints the
// same columns as plain text so the period can be pasted into a retro doc.
func (a *App) review(window reviewWindow) error {
	opts, err := window.options(a.Now())
	if err != nil {
		return err
	}
	if opts.Anchor, err = a.sprintAnchor(); err != nil {
		return err
	}
	if _, err := a.entriesOrHint(journal.OldestFirst); err != nil {
		return err
	}

	if !ui.Interactive() {
		result, err := a.Journal.Review(opts)
		if err != nil {
			return err
		}
		a.Printer.Printf("%s", ui.RenderReviewStatic(a.Theme, result, ui.TerminalWidth()-2))
		return nil
	}
	return ui.ShowReview(a.Theme, viewSource{app: a}, opts, a.Config.Mouse)
}
