package ui

import (
	"os/exec"
	"time"

	"daily-journal/internal/journal"
)

// fakeReviewSource is a ReviewSource backed by a real journal on disk, with a
// scripted editor so the edit path can be exercised without a terminal.
type fakeReviewSource struct {
	journal *journal.Journal
	// editErr is returned by EditCommand instead of a command, when set.
	editErr error
	// editCalls counts how many times EditCommand was asked for a command.
	editCalls int
	// lastEdited is the day of the most recent EditCommand call.
	lastEdited time.Time
	// lastOptions is the period of the most recent Review call.
	lastOptions journal.ReviewOptions
}

func (s *fakeReviewSource) Review(opts journal.ReviewOptions) (journal.Review, error) {
	s.lastOptions = opts
	return s.journal.Review(opts)
}

func (s *fakeReviewSource) EditCommand(day time.Time) (*exec.Cmd, error) {
	s.editCalls++
	s.lastEdited = day
	if s.editErr != nil {
		return nil, s.editErr
	}
	return exec.Command("true"), nil
}
