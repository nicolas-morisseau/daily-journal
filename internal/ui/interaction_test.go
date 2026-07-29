package ui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"daily-journal/internal/journal"
)

// errTest stands in for an editor failure.
var errTest = errors.New("boom")

// keyMsg is key() under a clearer name for tests that need the message itself
// rather than the model that results from it.
func keyMsg(name string) tea.KeyMsg { return key(name) }

// appendTo adds text to an existing file.
func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

// key builds the message Bubble Tea would deliver for a keystroke.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send feeds keystrokes to a model and returns the final state.
func send[M tea.Model](t *testing.T, m M, keys ...string) M {
	t.Helper()
	var model tea.Model = m
	for _, k := range keys {
		model, _ = model.Update(key(k))
	}
	typed, ok := model.(M)
	if !ok {
		t.Fatalf("model changed type: %T", model)
	}
	return typed
}

func newCalendarFixture(t *testing.T) (calendarModel, *fakeSource, time.Time) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/entries", 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# 2026-07-28\n\nsome words about #work\n"
	if err := os.WriteFile(dir+"/entries/2026-07-28.md", []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	j := journal.New(dir, func() time.Time { return now })
	source := newFakeSource(j)

	m := calendarModel{theme: Default, source: source, entry: viewport.New(40, 10), width: 120, height: 32}
	m.setMonth(journal.MonthOf(now))
	m.selectDay(now)
	return m, source, j.Today()
}

func TestCalendarDayAndMonthNavigation(t *testing.T) {
	m, _, today := newCalendarFixture(t)
	if !m.selected.Equal(today) {
		t.Fatalf("initial selection = %s, want today", m.selected)
	}

	m = send(t, m, "right")
	if got := m.selected.Format("2006-01-02"); got != "2026-07-29" {
		t.Errorf("right = %s, want 2026-07-29", got)
	}
	m = send(t, m, "down")
	if got := m.selected.Format("2006-01-02"); got != "2026-08-05" {
		t.Errorf("down should move a week and cross into August, got %s", got)
	}
	// Crossing a boundary must reload the month.
	if m.month.Month != time.August {
		t.Errorf("month = %v, want August", m.month.Month)
	}

	m = send(t, m, "t")
	if !m.selected.Equal(today) || m.month.Month != time.July {
		t.Errorf("t should return to today, got %s in %v", m.selected, m.month.Month)
	}

	// n and p page months, clamping the day: 31 July -> 31 August -> 30 September.
	m = send(t, m, "right", "right", "right") // 31 July
	m = send(t, m, "n")                       // 31 August
	m = send(t, m, "n")                       // September has 30 days
	if got := m.selected.Format("2006-01-02"); got != "2026-09-30" {
		t.Errorf("month paging should clamp the day, got %s", got)
	}
	m = send(t, m, "p")
	if m.month.Month != time.August {
		t.Errorf("p should page back, got %v", m.month.Month)
	}
}

// TestCalendarPreviewFollowsSelection is the behaviour the view exists for:
// moving on the grid shows that day's markdown in the centre pane.
func TestCalendarPreviewFollowsSelection(t *testing.T) {
	m, _, _ := newCalendarFixture(t)

	if !m.hasEntry {
		t.Fatal("today has an entry and should be previewed")
	}
	if m.entryWords == 0 {
		t.Error("the word count should be filled in")
	}
	if !strings.Contains(m.View(), "Tuesday, 28 July 2026") {
		t.Error("the centre pane should title the selected day")
	}

	// A day without an entry gets the empty state rather than stale content.
	m = send(t, m, "right")
	if m.hasEntry {
		t.Error("2026-07-29 has no entry")
	}
	if m.entryWords != 0 {
		t.Errorf("word count = %d, want 0", m.entryWords)
	}
	view := m.View()
	if !strings.Contains(view, "Nothing written") {
		t.Errorf("expected the empty state:\n%s", view)
	}
	if !strings.Contains(view, "Wednesday, 29 July 2026") {
		t.Error("the header should follow the selection")
	}
}

// TestCalendarEditShortcut checks that `e` and `enter` both reach the editor.
func TestCalendarEditShortcut(t *testing.T) {
	for _, pressed := range []string{"e", "enter"} {
		m, source, today := newCalendarFixture(t)
		model, cmd := m.Update(keyMsg(pressed))
		if cmd == nil {
			t.Fatalf("%q should return a command that runs the editor", pressed)
		}
		_ = model
		if source.editCalls != 1 {
			t.Errorf("%q asked for %d edit commands, want 1", pressed, source.editCalls)
		}
		if !source.lastEdited.Equal(today) {
			t.Errorf("%q edited %s, want the selected day %s", pressed, source.lastEdited, today)
		}
	}
}

func TestCalendarReloadsAfterEditing(t *testing.T) {
	m, source, _ := newCalendarFixture(t)

	// Simulate the editor adding text, then the view being told it finished.
	path := m.selected.Format("2006-01-02") + ".md"
	appendTo(t, source.journal.EntriesDir()+"/"+path, "\nmore words added by the editor\n")

	before := m.entryWords
	model, _ := m.Update(editorFinishedMsg{})
	after := model.(calendarModel)

	if after.entryWords <= before {
		t.Errorf("word count = %d, should have grown from %d", after.entryWords, before)
	}
	if !strings.Contains(after.View(), "Saved") {
		t.Error("a successful edit should report it")
	}
}

func TestCalendarReportsEditorFailure(t *testing.T) {
	m, _, _ := newCalendarFixture(t)
	model, _ := m.Update(editorFinishedMsg{err: errTest})
	if !strings.Contains(model.(calendarModel).View(), "boom") {
		t.Error("an editor failure should be shown")
	}
}

func TestCalendarScrollKeysDoNotMoveTheSelection(t *testing.T) {
	m, _, today := newCalendarFixture(t)
	m = send(t, m, "j", "j", "k", "ctrl+d", "ctrl+u", "g", "G")
	if !m.selected.Equal(today) {
		t.Errorf("scrolling the entry must not change the day, got %s", m.selected)
	}
}

// TestCalendarOpensAndLeavesTheReview covers the round trip: r switches to the
// sprint review of the selected day, and leaving it comes back to the month.
func TestCalendarOpensAndLeavesTheReview(t *testing.T) {
	m, _, today := newCalendarFixture(t)

	m = send(t, m, "r")
	if m.review == nil {
		t.Fatal("r should open the review")
	}
	view := m.View()
	if !strings.Contains(view, "SPRINT REVIEW") {
		t.Errorf("the review should be on screen:\n%s", view)
	}
	// The period ends on the selected day, and the footer says how to get back.
	if got := m.review.review.To; !got.Equal(today) {
		t.Errorf("period ends %s, want the selected day %s", got, today)
	}
	if !strings.Contains(view, "back to the calendar") {
		t.Errorf("a nested review should not offer to quit:\n%s", view)
	}

	// Keys go to the review while it is up, not to the calendar underneath.
	m = send(t, m, "right")
	if m.review == nil {
		t.Fatal("the review should still be up")
	}
	if m.review.focus != 1 {
		t.Errorf("right should move the review's mood, focus = %d", m.review.focus)
	}
	if !m.selected.Equal(today) {
		t.Errorf("the calendar's day moved to %s while the review was up", m.selected)
	}

	m = send(t, m, "q")
	if m.review != nil {
		t.Fatal("q should leave the review")
	}
	if m.quit {
		t.Error("leaving the review must not quit the program")
	}
	if !strings.Contains(m.View(), "July 2026") {
		t.Errorf("the calendar should be back:\n%s", m.View())
	}
	// And now q means what it says.
	if m = send(t, m, "q"); !m.quit {
		t.Error("q on the calendar should quit")
	}
}

func TestCalendarReviewFollowsTheSelectedDay(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	source.anchor = time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	// Move to 30 July, still inside the sprint that opens on 20 July.
	m = send(t, m, "right", "right", "r")
	if m.review == nil {
		t.Fatal("r should open the review")
	}
	if got := m.review.review.Period(); got != "20 Jul → 2 Aug 2026" {
		t.Errorf("Period = %q, want the sprint the selected day falls in", got)
	}
}

// TestCalendarReportsABrokenReviewPeriod: a misconfigured SPRINT_START must not
// take the calendar down.
func TestCalendarReportsABrokenReviewPeriod(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	source.periodErr = errTest

	m = send(t, m, "r")
	if m.review != nil {
		t.Error("the review should not open when the period cannot be resolved")
	}
	if !strings.Contains(m.View(), "boom") {
		t.Errorf("the failure should be shown:\n%s", m.View())
	}
}

// TestCalendarReviewEditReloadsBoth: editing from the nested review must refresh
// the calendar too, since the entry it just changed is the one on screen.
func TestCalendarReviewEditReloadsBoth(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	// Tag today's entry so the review has a note to sit on.
	appendTo(t, source.journal.EntriesDir()+"/2026-07-28.md", "\n**09:00** - shipped it [#good]\n")

	m = send(t, m, "r")
	if m.review == nil || len(m.review.columns[0].items) != 1 {
		t.Fatalf("the review should hold the tagged note: %+v", m.review)
	}

	before := m.entryWords
	appendTo(t, source.journal.EntriesDir()+"/2026-07-28.md", "\nmore words from the editor\n")
	model, _ := m.Update(editorFinishedMsg{})
	after := model.(calendarModel)

	if !strings.Contains(after.View(), "Saved") {
		t.Error("the review should report the save")
	}
	after = send(t, after, "q")
	if after.entryWords <= before {
		t.Errorf("the calendar should have reloaded: %d words, was %d", after.entryWords, before)
	}
}

func TestCalendarQuit(t *testing.T) {
	m, _, _ := newCalendarFixture(t)
	m = send(t, m, "q")
	if !m.quit {
		t.Error("q should quit")
	}
	if m.View() != "" {
		t.Error("a quitting model should render nothing")
	}
}

// TestCalendarDropsPanelsWhenNarrow checks the responsive fallbacks.
func TestCalendarDropsPanelsWhenNarrow(t *testing.T) {
	m, _, _ := newCalendarFixture(t)

	cases := []struct {
		width    int
		calendar bool
		insight  bool
	}{
		{140, true, true},
		{110, true, true},
		{80, true, false},
		{60, false, false},
	}
	for _, c := range cases {
		model, _ := m.Update(tea.WindowSizeMsg{Width: c.width, Height: 30})
		sized := model.(calendarModel)
		if sized.showCalendarPane() != c.calendar {
			t.Errorf("width %d: calendar pane = %v, want %v", c.width, sized.showCalendarPane(), c.calendar)
		}
		if sized.showInsight() != c.insight {
			t.Errorf("width %d: insight pane = %v, want %v", c.width, sized.showInsight(), c.insight)
		}
		view := sized.View()
		if view == "" {
			t.Errorf("width %d rendered nothing", c.width)
		}
		// Whatever is dropped, the entry pane must stay usable.
		if sized.entry.Width < minEntryPaneWidth {
			t.Errorf("width %d: entry pane shrank to %d", c.width, sized.entry.Width)
		}
	}
}
