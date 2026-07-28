package ui

import (
	"os/exec"
	"time"

	"daily-journal/internal/journal"
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

func (s *fakeSource) EditCommand(day time.Time) (*exec.Cmd, error) {
	s.editCalls++
	s.lastEdited = day
	if s.editErr != nil {
		return nil, s.editErr
	}
	return exec.Command("true"), nil
}
