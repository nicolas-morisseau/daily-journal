package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/journal"
)

// reviewFixture builds a review over a journal holding three #good notes, two
// #bad and one #mad, all inside the default fortnight.
func reviewFixture(t *testing.T) (reviewModel, *fakeReviewSource) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/entries", 0o755); err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{
		"2026-07-20": "# 2026-07-20\n\n**09:00** - shipped the migration [#good]\n\n**11:00** - lost in the MRs [#bad]\n",
		"2026-07-24": "# 2026-07-24\n\n**14:00** - the all hands was worth it [#good]\n\n**16:00** - the printer, again [#mad]\n",
		"2026-07-28": "# 2026-07-28\n\n**10:00** - found a public repo with the roadmap in it [#bad]\n\n**18:00** - a permanent contract [#good]\n",
	}
	for slug, body := range entries {
		if err := os.WriteFile(dir+"/entries/"+slug+".md", []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	source := &fakeReviewSource{journal: journal.New(dir, func() time.Time { return now })}

	m := reviewModel{theme: Default, source: source, width: 140, height: 32, mouse: true}
	for i, mood := range journal.Moods {
		m.columns[i].mood = mood
		m.columns[i].view = viewport.New(minReviewColumnWidth, 10)
	}
	m.load(journal.ReviewOptions{}, true)
	return m, source
}

// TestReviewShowsThreeMoodsAtOnce is what the view exists for: the three columns,
// their counts and the period, all on one screen.
func TestReviewShowsThreeMoodsAtOnce(t *testing.T) {
	m, _ := reviewFixture(t)

	if got := len(m.columns[0].items); got != 3 {
		t.Errorf("good = %d notes, want 3", got)
	}
	if got := len(m.columns[1].items); got != 2 {
		t.Errorf("bad = %d notes, want 2", got)
	}
	if got := len(m.columns[2].items); got != 1 {
		t.Errorf("mad = %d notes, want 1", got)
	}

	view := m.View()
	for _, want := range []string{
		"SPRINT REVIEW", "15 Jul → 28 Jul 2026", "14 days", "3 entries", "6 notes",
		"GOOD", "BAD", "MAD",
		"Mon 20 Jul", "shipped the migration", "the printer, again",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	// The tags label the columns; repeating them on every card is noise.
	if strings.Contains(view, "[#good]") {
		t.Error("cards should not carry their mood tag")
	}
}

func TestReviewMoodAndNoteNavigation(t *testing.T) {
	m, _ := reviewFixture(t)

	if m.focus != 0 || m.columns[0].selected != 0 {
		t.Fatalf("initial focus = column %d, note %d, want the first of both", m.focus, m.columns[0].selected)
	}

	m = send(t, m, "down")
	if m.columns[0].selected != 1 {
		t.Errorf("down = note %d, want 1", m.columns[0].selected)
	}
	// The selection stops at the ends rather than wrapping.
	m = send(t, m, "down", "down", "down")
	if m.columns[0].selected != 2 {
		t.Errorf("down past the end = note %d, want 2", m.columns[0].selected)
	}
	m = send(t, m, "g")
	if m.columns[0].selected != 0 {
		t.Errorf("g = note %d, want 0", m.columns[0].selected)
	}
	m = send(t, m, "G")
	if m.columns[0].selected != 2 {
		t.Errorf("G = note %d, want the last", m.columns[0].selected)
	}

	// Moods wrap, so three columns are always two keys apart.
	m = send(t, m, "right")
	if m.focus != 1 {
		t.Errorf("right = column %d, want 1", m.focus)
	}
	m = send(t, m, "right", "right")
	if m.focus != 0 {
		t.Errorf("right should wrap round to 0, got %d", m.focus)
	}
	m = send(t, m, "left")
	if m.focus != 2 {
		t.Errorf("left should wrap back to 2, got %d", m.focus)
	}

	// Each mood keeps its own place in its own list.
	if m.columns[0].selected != 2 {
		t.Errorf("good lost its selection: %d", m.columns[0].selected)
	}
	if m.columns[2].selected != 0 {
		t.Errorf("mad = note %d, want 0", m.columns[2].selected)
	}
}

// TestReviewSelectionScrollsIntoView locks down the reason the cards are
// measured: a selection below the fold must scroll the column.
func TestReviewSelectionScrollsIntoView(t *testing.T) {
	m, _ := reviewFixture(t)
	// A short pane, so three cards cannot all fit.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 16})
	m = model.(reviewModel)

	if m.columns[0].view.YOffset != 0 {
		t.Fatalf("the first note should be at the top, offset = %d", m.columns[0].view.YOffset)
	}
	m = send(t, m, "G")
	if m.columns[0].view.YOffset == 0 {
		t.Error("selecting the last note should have scrolled the column")
	}
	m = send(t, m, "g")
	if m.columns[0].view.YOffset != 0 {
		t.Errorf("going back to the first note should scroll back, offset = %d", m.columns[0].view.YOffset)
	}
}

func TestReviewPeriodPaging(t *testing.T) {
	m, source := reviewFixture(t)

	m = send(t, m, "p")
	if got := m.review.To.Format("2006-01-02"); got != "2026-07-14" {
		t.Errorf("p = period ending %s, want 2026-07-14 (the fortnight before)", got)
	}
	if got := m.review.From.Format("2006-01-02"); got != "2026-07-01" {
		t.Errorf("p = period starting %s, want 2026-07-01", got)
	}
	if m.review.Total() != 0 {
		t.Errorf("the previous fortnight holds nothing, got %d notes", m.review.Total())
	}
	// An empty period explains itself rather than showing three blank columns.
	if !strings.Contains(m.View(), "No entry in this period") {
		t.Error("an empty period should say so")
	}

	m = send(t, m, "n")
	if got := m.review.To.Format("2006-01-02"); got != "2026-07-28" {
		t.Errorf("n should page back forward, got %s", got)
	}

	m = send(t, m, "p", "p", "t")
	if got := m.review.To.Format("2006-01-02"); got != "2026-07-28" {
		t.Errorf("t = period ending %s, want today's window back", got)
	}
	if source.lastOptions.From != (time.Time{}) {
		t.Errorf("t should reload the period the command was given, got %+v", source.lastOptions)
	}
}

func TestReviewEditsTheSelectedNotesDay(t *testing.T) {
	for _, pressed := range []string{"e", "enter"} {
		m, source := reviewFixture(t)
		// Second #bad note, which is the one from the 28th.
		m = send(t, m, "right", "down")
		_, cmd := m.Update(keyMsg(pressed))
		if cmd == nil {
			t.Fatalf("%q should return a command that runs the editor", pressed)
		}
		if source.editCalls != 1 {
			t.Errorf("%q asked for %d edit commands, want 1", pressed, source.editCalls)
		}
		if got := source.lastEdited.Format("2006-01-02"); got != "2026-07-28" {
			t.Errorf("%q edited %s, want the selected note's day 2026-07-28", pressed, got)
		}
	}
}

func TestReviewReportsEditorOutcome(t *testing.T) {
	m, _ := reviewFixture(t)

	model, _ := m.Update(editorFinishedMsg{})
	if !strings.Contains(model.(reviewModel).View(), "Saved 2026-07-20.md") {
		t.Error("a successful edit should name the entry it saved")
	}

	model, _ = m.Update(editorFinishedMsg{err: errTest})
	if !strings.Contains(model.(reviewModel).View(), "boom") {
		t.Error("an editor failure should be shown")
	}
}

// TestReviewEditIsANoOpOnAnEmptyMood guards the empty column: there is no note
// to open, and pressing e must not reach for the editor.
func TestReviewEditIsANoOpOnAnEmptyMood(t *testing.T) {
	m, source := reviewFixture(t)
	m = send(t, m, "p") // an empty fortnight

	if _, cmd := m.Update(keyMsg("e")); cmd != nil {
		t.Error("e on an empty mood should do nothing")
	}
	if source.editCalls != 0 {
		t.Errorf("editor asked for %d times, want 0", source.editCalls)
	}
	if m.View() == "" {
		t.Error("an empty review should still render")
	}
}

// TestReviewNarrowTerminalKeepsOneMood is the responsive fallback: three columns
// while they fit, the focused one alone when they do not.
func TestReviewNarrowTerminalKeepsOneMood(t *testing.T) {
	m, _ := reviewFixture(t)

	cases := []struct {
		width int
		all   bool
	}{{140, true}, {100, true}, {76, true}, {60, false}, {40, false}}
	for _, c := range cases {
		model, _ := m.Update(tea.WindowSizeMsg{Width: c.width, Height: 30})
		sized := model.(reviewModel)
		if sized.showAllColumns() != c.all {
			t.Errorf("width %d: all columns = %v, want %v", c.width, sized.showAllColumns(), c.all)
		}
		if sized.columnWidth() < minReviewColumnWidth {
			t.Errorf("width %d: column shrank to %d", c.width, sized.columnWidth())
		}

		view := sized.View()
		if view == "" {
			t.Fatalf("width %d rendered nothing", c.width)
		}
		// Whatever the width, the layout must not spill sideways.
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > c.width {
				t.Errorf("width %d: line %d is %d columns wide", c.width, i, got)
			}
		}
		if !c.all && !strings.Contains(view, "GOOD") {
			t.Errorf("width %d should still show the focused mood:\n%s", c.width, view)
		}
	}
}

// TestReviewSurvivesAnUnreportedSize covers the pty that says 0 by 0 before it
// knows its own size, which used to take the whole view down.
func TestReviewSurvivesAnUnreportedSize(t *testing.T) {
	m, _ := reviewFixture(t)
	model, _ := m.Update(tea.WindowSizeMsg{})
	sized := model.(reviewModel)

	if sized.width < minReviewWidth || sized.height < minReviewHeight {
		t.Errorf("size = %dx%d, want it clamped to at least %dx%d",
			sized.width, sized.height, minReviewWidth, minReviewHeight)
	}
	if sized.View() == "" {
		t.Error("a size-less terminal should still render something")
	}
}

func TestReviewQuit(t *testing.T) {
	m, _ := reviewFixture(t)
	m = send(t, m, "q")
	if !m.quit {
		t.Error("q should quit")
	}
	if m.View() != "" {
		t.Error("a quitting model should render nothing")
	}
}

func TestRenderReviewStatic(t *testing.T) {
	m, _ := reviewFixture(t)

	wide := RenderReviewStatic(Default, m.review, 120)
	for _, want := range []string{"Sprint review", "15 Jul → 28 Jul 2026", "GOOD", "BAD", "MAD", "shipped the migration"} {
		if !strings.Contains(wide, want) {
			t.Errorf("static review is missing %q:\n%s", want, wide)
		}
	}
	for i, line := range strings.Split(wide, "\n") {
		if got := lipgloss.Width(line); got > 120 {
			t.Errorf("line %d is %d columns wide, want at most 120", i, got)
		}
	}

	// Too narrow for three columns: they stack instead of being squeezed.
	narrow := RenderReviewStatic(Default, m.review, 40)
	for _, want := range []string{"GOOD", "BAD", "MAD", "the printer, again"} {
		if !strings.Contains(narrow, want) {
			t.Errorf("stacked review is missing %q:\n%s", want, narrow)
		}
	}
	for i, line := range strings.Split(narrow, "\n") {
		if got := lipgloss.Width(line); got > 40 {
			t.Errorf("stacked line %d is %d columns wide, want at most 40", i, got)
		}
	}
}
