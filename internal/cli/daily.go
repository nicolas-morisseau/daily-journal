package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"daily-journal/internal/config"
	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
	"daily-journal/internal/ui"
)

// Daily collects the journal notes of the stand-up window.
func (s viewSource) Daily(opts journal.DailyOptions) journal.Daily {
	return s.app.Journal.Daily(opts)
}

// Standup reads the git activity of the last days.
func (s viewSource) Standup(days int) (standup.Report, error) {
	return s.app.standup(days)
}

// DayStandup reads the git activity of one day, which is what the calendar's git
// panel shows for the day it is sitting on.
func (s viewSource) DayStandup(day time.Time) (standup.Report, error) {
	dir, err := s.app.projectsDir()
	if err != nil {
		return standup.Report{}, err
	}
	return standup.RunDay(context.Background(), dir, day)
}

// Capture appends a timestamped line to today's entry, which is what typing in
// the stand-up view does.
func (s viewSource) Capture(text string) error {
	_, err := s.app.Journal.QuickCapture(s.app.Now().Format(dateparse.Layout), text)
	return err
}

// projectsDir is the directory holding the repositories git-standup walks.
//
// An unset PROJECTS_DIR is reported rather than guessed at: the directory holding
// someone's repositories is not something to go looking for.
func (a *App) projectsDir() (string, error) {
	dir := strings.TrimSpace(a.Config.ProjectsDir)
	if dir == "" {
		return "", fmt.Errorf(
			"no %s in %s\n\nPoint it at the directory holding your repositories and the commits of\n"+
				"the last day show up here:\n  %s=\"$HOME/projects\"",
			config.KeyProjectsDir, a.Config.Path, config.KeyProjectsDir)
	}
	return dir, nil
}

// standup runs git-standup over the configured projects directory.
func (a *App) standup(days int) (standup.Report, error) {
	dir, err := a.projectsDir()
	if err != nil {
		return standup.Report{}, err
	}
	return standup.Run(context.Background(), dir, days)
}

// dailyDays reads the window length shared by both halves of the stand-up.
func dailyDays(raw string) (int, error) {
	if raw == "" {
		return journal.DefaultDailyDays, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 0 {
		return 0, usagef("--days takes a number of days back, got %q", raw)
	}
	return days, nil
}

// daily backs `journal daily`.
//
// On a terminal it runs the full-screen view, where a note can be typed straight
// in; piped, it prints the same recap as text so it can be pasted into wherever
// the stand-up is written down.
func (a *App) daily(rawDays string) error {
	days, err := dailyDays(rawDays)
	if err != nil {
		return err
	}

	if ui.Interactive() {
		return ui.ShowDaily(a.Theme, viewSource{app: a}, days, a.Config.Mouse)
	}

	recap := a.Journal.Daily(journal.DailyOptions{Days: days})
	// A missing or unconfigured git-standup is not a failure: the journal half of
	// the recap still says what the day held.
	report, standupErr := a.standup(days)
	a.Printer.Printf("%s", ui.RenderDailyStatic(a.Theme, recap, report, standupErr, ui.TerminalWidth()-2))
	return nil
}
