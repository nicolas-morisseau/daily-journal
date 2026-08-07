package ui

import (
	"os/exec"
	"time"

	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
)

// fakeSource is a CalendarSource backed by a real journal on disk, with a
// scripted editor so the edit path can be exercised without a terminal.
type fakeSource struct {
	journal *journal.Journal
	// editErr is returned by EditCommand instead of a command, when set.
	editErr error
	// editCalls counts how many times EditCommand was asked for a command.
	editCalls int
	// lastEdited is the day of the most recent EditCommand call.
	lastEdited time.Time
	// periodErr is returned by ReviewPeriod instead of a period, when set. It
	// stands in for an unreadable SPRINT_START.
	periodErr error
	// anchor is the sprint start ReviewPeriod hands out, zero for a rolling window.
	anchor time.Time

	// dayReports is the git activity DayStandup hands out, by YYYY-MM-DD. A day
	// that is not in it saw no commit.
	dayReports map[string]standup.Report
	// dayStandupErr fails DayStandup instead of answering it, when set.
	dayStandupErr error
	// dayStandupCalls counts the runs, and dayStandupDays lists the days they were
	// asked for, in order: the panel must follow the selection without starting a
	// run per keystroke.
	dayStandupCalls int
	dayStandupDays  []string
}

func newFakeSource(j *journal.Journal) *fakeSource { return &fakeSource{journal: j} }

func (s *fakeSource) Calendar(m journal.Month) journal.Calendar { return s.journal.Calendar(m) }
func (s *fakeSource) Today() time.Time                          { return s.journal.Today() }

func (s *fakeSource) EntryContent(day time.Time) (string, bool) {
	return s.journal.EntryContent(day)
}

func (s *fakeSource) Insight(day time.Time, opts journal.InsightOptions) journal.Insight {
	return s.journal.Insight(day, opts)
}

func (s *fakeSource) Review(opts journal.ReviewOptions) (journal.Review, error) {
	return s.journal.Review(opts)
}

func (s *fakeSource) ReviewPeriod(day time.Time) (journal.ReviewOptions, error) {
	if s.periodErr != nil {
		return journal.ReviewOptions{}, s.periodErr
	}
	return journal.ReviewOptions{To: day, Anchor: s.anchor}, nil
}

func (s *fakeSource) DayStandup(day time.Time) (standup.Report, error) {
	slug := day.Format("2006-01-02")
	s.dayStandupCalls++
	s.dayStandupDays = append(s.dayStandupDays, slug)
	if s.dayStandupErr != nil {
		return standup.Report{}, s.dayStandupErr
	}
	return s.dayReports[slug], nil
}

func (s *fakeSource) EditCommand(day time.Time) (*exec.Cmd, error) {
	s.editCalls++
	s.lastEdited = day
	if s.editErr != nil {
		return nil, s.editErr
	}
	return exec.Command("true"), nil
}
