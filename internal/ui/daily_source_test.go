package ui

import (
	"os/exec"
	"time"

	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
)

// fakeDailySource is a DailySource backed by a real journal on disk, with a
// scripted editor and a scripted git-standup so both can be exercised without a
// terminal or a directory full of repositories.
type fakeDailySource struct {
	journal *journal.Journal

	// report and standupErr are what Standup hands out.
	report     standup.Report
	standupErr error
	// standupCalls counts the runs, which is how a refresh is recognised.
	standupCalls int
	// standupDays is the window of the most recent run.
	standupDays int

	// captureErr fails the capture instead of writing it, when set.
	captureErr error
	// captured holds every line the view captured, in order.
	captured []string

	// editErr is returned by EditCommand instead of a command, when set.
	editErr error
	// editCalls counts how many times EditCommand was asked for a command.
	editCalls int
	// lastEdited is the day of the most recent EditCommand call.
	lastEdited time.Time
}

func (s *fakeDailySource) Today() time.Time { return s.journal.Today() }

func (s *fakeDailySource) Daily(opts journal.DailyOptions) journal.Daily {
	return s.journal.Daily(opts)
}

func (s *fakeDailySource) Standup(days int) (standup.Report, error) {
	s.standupCalls++
	s.standupDays = days
	return s.report, s.standupErr
}

func (s *fakeDailySource) Capture(text string) error {
	if s.captureErr != nil {
		return s.captureErr
	}
	s.captured = append(s.captured, text)
	_, err := s.journal.QuickCapture(s.journal.Today().Format("2006-01-02"), text)
	return err
}

func (s *fakeDailySource) EditCommand(day time.Time) (*exec.Cmd, error) {
	s.editCalls++
	s.lastEdited = day
	if s.editErr != nil {
		return nil, s.editErr
	}
	return exec.Command("true"), nil
}
