package ui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
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
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
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

	m := newCalendarModel(Default, source, 120, 32)
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

// longEntryFixture is the calendar on a day whose notes run well past the pane,
// so the centre pane has somewhere to scroll to.
func longEntryFixture(t *testing.T) (calendarModel, time.Time) {
	t.Helper()
	m, source, today := newCalendarFixture(t)
	appendTo(t, source.journal.EntriesDir()+"/2026-07-28.md",
		strings.Repeat("\nanother line of the day\n", 60))
	m.reload()
	return m, today
}

func TestCalendarScrollKeysDoNotMoveTheSelection(t *testing.T) {
	m, today := longEntryFixture(t)
	m = send(t, m, "tab", "j", "j", "k", "ctrl+d", "ctrl+u", "g", "G")
	if !m.selected.Equal(today) {
		t.Errorf("scrolling the entry must not change the day, got %s", m.selected)
	}
	if m.entry.YOffset == 0 {
		t.Error("G should have scrolled the entry to the bottom")
	}
}

// TestCalendarJKMoveAWeekOnTheGrid is issue #1: the grid has the focus to begin
// with, and there j and k are the vim spelling of ↓ and ↑.
func TestCalendarJKMoveAWeekOnTheGrid(t *testing.T) {
	m, today := longEntryFixture(t)
	if m.focus != calendarGridPane {
		t.Fatalf("focus = %d, want the grid first", m.focus)
	}

	m = send(t, m, "j")
	if got := m.selected.Format("2006-01-02"); got != "2026-08-04" {
		t.Errorf("j should move a week and cross into August, got %s", got)
	}
	if m.month.Month != time.August {
		t.Errorf("month = %v, want August", m.month.Month)
	}
	m = send(t, m, "k")
	if !m.selected.Equal(today) {
		t.Errorf("k should come back a week, got %s", m.selected)
	}
	if m.entry.YOffset != 0 {
		t.Errorf("j k on the grid scrolled the entry to %d", m.entry.YOffset)
	}

	// The keys the grid has no use for still scroll the notes from there.
	m = send(t, m, "ctrl+d")
	if m.entry.YOffset == 0 || !m.selected.Equal(today) {
		t.Errorf("ctrl+d on the grid should scroll the entry, offset %d, day %s", m.entry.YOffset, m.selected)
	}
	m = send(t, m, "g")
	if m.entry.YOffset != 0 {
		t.Errorf("g should take the entry back to the top, offset %d", m.entry.YOffset)
	}
}

// TestCalendarTabHandsJKToTheNotes: tab moves the focus between the grid and the
// notes, and j k follow it while the arrows keep moving on the grid.
func TestCalendarTabHandsJKToTheNotes(t *testing.T) {
	m, today := longEntryFixture(t)

	m = send(t, m, "tab")
	if m.focus != calendarEntryPane {
		t.Fatalf("tab should give the notes the focus, focus = %d", m.focus)
	}
	m = send(t, m, "j", "j", "j", "k")
	if m.entry.YOffset != 2 {
		t.Errorf("j j j k on the notes left them at %d, want 2", m.entry.YOffset)
	}
	if !m.selected.Equal(today) {
		t.Errorf("j k on the notes moved the day to %s", m.selected)
	}

	// The arrows are the grid's whatever has the focus.
	moved := send(t, m, "down")
	if got := moved.selected.Format("2006-01-02"); got != "2026-08-04" {
		t.Errorf("down with the notes focused should still move a week, got %s", got)
	}
	if moved.focus != calendarEntryPane {
		t.Error("moving on the grid should not take the focus back")
	}

	for _, back := range []string{"tab", "shift+tab"} {
		grid := send(t, m, back)
		if grid.focus != calendarGridPane {
			t.Errorf("%s should give the grid the focus back, focus = %d", back, grid.focus)
		}
		if got := send(t, grid, "j").selected.Format("2006-01-02"); got != "2026-08-04" {
			t.Errorf("j after %s should move a week again, got %s", back, got)
		}
	}
}

// TestCalendarJKScrollWithoutTheGrid: too narrow for the grid, the notes are all
// there is, so j k scroll them whatever the focus says.
func TestCalendarJKScrollWithoutTheGrid(t *testing.T) {
	m, today := longEntryFixture(t)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 32})
	m = model.(calendarModel)
	if m.showCalendarPane() {
		t.Fatal("60 columns should not hold the grid")
	}

	m = send(t, m, "j", "j")
	if m.entry.YOffset != 2 {
		t.Errorf("j j left the notes at %d, want 2", m.entry.YOffset)
	}
	if !m.selected.Equal(today) {
		t.Errorf("j with no grid on screen moved the day to %s", m.selected)
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

// --- the git activity panel --------------------------------------------------

// sampleDayReport is one day's git activity: two repositories, three commits,
// timed the way RunDay asks git to print them.
func sampleDayReport() standup.Report {
	return standup.Report{
		Dir:  "/Users/dev/projects",
		Days: 1,
		Repos: []standup.Repo{
			{Path: "/Users/dev/projects/stats-service", Commits: []standup.Commit{
				{Hash: "676ee18", Subject: "chore(log): bump the library", When: "09:41", Author: "dev"},
				{Hash: "468a5d8", Subject: "fix: the classpath", When: "14:02", Author: "dev"},
			}},
			{Path: "/Users/dev/projects/mailer-service", Commits: []standup.Commit{
				{Hash: "9329a3a", Subject: "feat: migrate the registry", When: "17:20", Author: "dev"},
			}},
		},
	}
}

// standupAnswer runs a command and returns the git-standup answer it carries. The
// view batches the run with the spinner's first tick, so the batch is walked.
func standupAnswer(t *testing.T, cmd tea.Cmd) (dayStandupLoadedMsg, bool) {
	t.Helper()
	if cmd == nil {
		return dayStandupLoadedMsg{}, false
	}
	switch msg := cmd().(type) {
	case dayStandupLoadedMsg:
		return msg, true
	case tea.BatchMsg:
		for _, inner := range msg {
			if answer, ok := standupAnswer(t, inner); ok {
				return answer, true
			}
		}
	}
	return dayStandupLoadedMsg{}, false
}

// openGitPanel puts the panel up and lets its first run land, which is the state
// every test below starts from.
func openGitPanel(t *testing.T, m calendarModel) calendarModel {
	t.Helper()
	model, cmd := m.Update(keyMsg("s"))
	answer, ok := standupAnswer(t, cmd)
	if !ok {
		t.Fatal("opening the panel should ask for the day's git activity")
	}
	model, _ = model.(calendarModel).Update(answer)
	return model.(calendarModel)
}

// TestCalendarGitPanelShowsTheSelectedDay is what the panel exists for: the
// commits of the day the calendar is sitting on, and of no other day.
func TestCalendarGitPanelShowsTheSelectedDay(t *testing.T) {
	m, source, today := newCalendarFixture(t)
	source.dayReports = map[string]standup.Report{today.Format("2006-01-02"): sampleDayReport()}

	if strings.Contains(m.View(), "GIT") {
		t.Error("the panel should stay down until it is asked for")
	}

	m = openGitPanel(t, m)
	if source.dayStandupCalls != 1 {
		t.Errorf("%d runs, want the one for the selected day", source.dayStandupCalls)
	}
	if got := source.dayStandupDays[0]; got != "2026-07-28" {
		t.Errorf("read the activity of %s, want the selected day", got)
	}

	// The subjects are cut to the column's width, so only their start shows.
	view := m.View()
	for _, want := range []string{"GIT", "3 commits", "stats-service", "676ee18",
		"chore(", "09:41", "mailer-service"} {
		if !strings.Contains(view, want) {
			t.Errorf("the panel should show %q:\n%s", want, view)
		}
	}
	// It shares the middle column with the notes; the stats keep theirs.
	if !m.showInsight() {
		t.Error("the stats panel should keep its column")
	}
	if !strings.Contains(view, "Tuesday, 28 July 2026") {
		t.Error("the markdown should keep its place")
	}
}

// TestCalendarGitPanelFollowsTheSelection: the panel is about the selected day, so
// moving the selection re-reads it and never shows another day's commits.
func TestCalendarGitPanelFollowsTheSelection(t *testing.T) {
	m, source, today := newCalendarFixture(t)
	source.dayReports = map[string]standup.Report{
		today.Format("2006-01-02"): sampleDayReport(),
		"2026-07-29":               {Repos: []standup.Repo{{Path: "/p/other-service", Commits: []standup.Commit{{Hash: "abc1234", Subject: "docs: the README", When: "11:03"}}}}},
	}
	m = openGitPanel(t, m)

	model, cmd := m.Update(keyMsg("right"))
	m = model.(calendarModel)
	// Until the new day's answer lands, the panel says it is reading rather than
	// leaving the previous day's commits under another date.
	if got := m.View(); strings.Contains(got, "stats-service") {
		t.Errorf("the previous day's commits are still up:\n%s", got)
	}

	answer, ok := standupAnswer(t, cmd)
	if !ok {
		t.Fatal("no run for the new day")
	}
	if source.dayStandupCalls != 2 || source.dayStandupDays[1] != "2026-07-29" {
		t.Fatalf("moving a day should re-read it, days = %v", source.dayStandupDays)
	}
	model, _ = m.Update(answer)
	view := model.(calendarModel).View()
	if !strings.Contains(view, "other-service") || !strings.Contains(view, "1 commit") {
		t.Errorf("the new day's commits should be up:\n%s", view)
	}
}

// TestCalendarGitPanelDropsAStaleAnswer: the helper takes seconds, so the day can
// have moved by the time it answers. That answer belongs to another day.
func TestCalendarGitPanelDropsAStaleAnswer(t *testing.T) {
	m, source, today := newCalendarFixture(t)
	source.dayReports = map[string]standup.Report{today.Format("2006-01-02"): sampleDayReport()}

	model, cmd := m.Update(keyMsg("s"))
	m = model.(calendarModel)
	stale, ok := standupAnswer(t, cmd)
	if !ok {
		t.Fatal("opening the panel should start a run")
	}

	// Two days on while the helper reads. Neither starts a second walk of the
	// repositories: the one in flight is followed by a catch-up instead.
	for _, pressed := range []string{"right", "right"} {
		model, cmd = m.Update(keyMsg(pressed))
		m = model.(calendarModel)
		if cmd != nil {
			t.Errorf("%q started a run while one was already in flight", pressed)
		}
	}
	if source.dayStandupCalls != 1 {
		t.Errorf("%d runs, want one in flight at a time", source.dayStandupCalls)
	}

	// The answer for the day the selection left is dropped, and the day now on
	// screen is asked for instead.
	model, cmd = m.Update(stale)
	m = model.(calendarModel)
	if strings.Contains(m.View(), "stats-service") {
		t.Error("the stale report should not be shown against another day")
	}

	answer, ok := standupAnswer(t, cmd)
	if !ok {
		t.Fatal("the stale answer should be followed by a catch-up run")
	}
	if !answer.day.Equal(m.selected) {
		t.Errorf("the catch-up run read %s, want the day on screen %s", answer.day, m.selected)
	}
	if source.dayStandupCalls != 2 || source.dayStandupDays[1] != "2026-07-30" {
		t.Fatalf("days read = %v, want the day on screen second", source.dayStandupDays)
	}
}

// TestCalendarGitPanelClosesAgain checks the toggle, and that a closed panel stops
// following the day: nothing should keep walking a directory of repositories.
func TestCalendarGitPanelClosesAgain(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	m = openGitPanel(t, m)

	m = send(t, m, "s")
	if m.gitOpen {
		t.Fatal("s should take the panel down")
	}
	view := m.View()
	if strings.Contains(view, "GIT") {
		t.Errorf("the panel should be gone:\n%s", view)
	}
	if !m.showInsight() {
		t.Error("the insight panel should have its column back")
	}

	for _, pressed := range []string{"right", "right"} {
		model, cmd := m.Update(keyMsg(pressed))
		m = model.(calendarModel)
		if cmd != nil {
			t.Errorf("%q asked for git activity with the panel down", pressed)
		}
	}
	if source.dayStandupCalls != 1 {
		t.Errorf("%d runs, a closed panel should not read anything", source.dayStandupCalls)
	}
}

// TestCalendarGitPanelReportsAMissingHelper: an optional tool that is not there is
// worth a line in the panel, not a failure that takes the calendar down.
func TestCalendarGitPanelReportsAMissingHelper(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	source.dayStandupErr = standup.ErrNotInstalled

	m = openGitPanel(t, m)
	view := m.View()
	if !strings.Contains(view, standup.Binary+" missing") {
		t.Errorf("the heading should say the helper is missing:\n%s", view)
	}
	// The hint is wrapped to a 24-column panel, so only its words survive whole.
	if !strings.Contains(view, "brew") || !strings.Contains(view, "installed") {
		t.Errorf("the panel should say how to get it:\n%s", view)
	}
	if !errors.Is(m.gitErr, standup.ErrNotInstalled) {
		t.Errorf("gitErr = %v, want the missing helper", m.gitErr)
	}
	// And the day is still readable beside it.
	if !strings.Contains(view, "Tuesday, 28 July 2026") {
		t.Error("the markdown should be untouched")
	}
}

// TestCalendarGitPanelSitsBesideOrUnderTheNotes: the panel shares the middle
// column with the day's notes, and where it lands depends on the room there is.
// Whatever happens, the stats keep their column.
func TestCalendarGitPanelSitsBesideOrUnderTheNotes(t *testing.T) {
	base, source, today := newCalendarFixture(t)
	source.dayReports = map[string]standup.Report{today.Format("2006-01-02"): sampleDayReport()}
	base = openGitPanel(t, base)

	cases := []struct {
		width   int
		beside  bool
		insight bool
	}{
		{200, true, true},
		{140, true, true},
		{120, false, true},
		{80, false, false},
	}
	for _, c := range cases {
		model, _ := base.Update(tea.WindowSizeMsg{Width: c.width, Height: 30})
		m := model.(calendarModel)

		if m.gitBeside() != c.beside {
			t.Errorf("width %d: beside the notes = %v, want %v", c.width, m.gitBeside(), c.beside)
		}
		if m.showInsight() != c.insight {
			t.Errorf("width %d: the stats panel should not have moved, showing = %v",
				c.width, m.showInsight())
		}
		// However they are arranged, both are up and readable.
		view := m.View()
		for _, want := range []string{"stats-service", "Tuesday, 28 July 2026"} {
			if !strings.Contains(view, want) {
				t.Errorf("width %d lost %q:\n%s", c.width, want, view)
			}
		}
		if m.entry.Width < minEntryPaneWidth {
			t.Errorf("width %d: the notes shrank to %d columns", c.width, m.entry.Width)
		}
		if m.entry.Height < 3 || m.git.Height < 3 {
			t.Errorf("width %d: entry %d rows, git %d rows", c.width, m.entry.Height, m.git.Height)
		}

		// Stacked or not, the middle column comes out as tall as the panels beside
		// it, so the view is the height it would be with the panel down.
		closed := send(t, m, "s")
		if got, want := viewRows(view), viewRows(closed.View()); got != want {
			t.Errorf("width %d: the view is %d rows with the panel up, %d without",
				c.width, got, want)
		}
	}
}

// viewRows counts the rows a rendered view takes.
func viewRows(view string) int { return len(strings.Split(view, "\n")) }

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
