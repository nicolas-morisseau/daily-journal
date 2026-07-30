package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
)

// runCmd runs a command the way the runtime would, following a batch into the
// commands it holds.
func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, inner := range batch {
			runCmd(inner)
		}
	}
}

// sampleReport is the git half of the fixture: two repositories, three commits.
func sampleReport() standup.Report {
	return standup.Report{
		Dir:  "/Users/dev/projects",
		Days: 1,
		Repos: []standup.Repo{
			{Path: "/Users/dev/projects/stats-service", Commits: []standup.Commit{
				{Hash: "676ee18", Subject: "chore(log): bump the library", When: "3 hours ago", Author: "dev"},
				{Hash: "468a5d8", Subject: "fix: the classpath", When: "5 hours ago", Author: "dev"},
			}},
			{Path: "/Users/dev/projects/mailer-service", Commits: []standup.Commit{
				{Hash: "9329a3a", Subject: "feat: migrate the registry", When: "yesterday", Author: "dev"},
			}},
		},
	}
}

// newDailyFixture builds the stand-up view over a journal holding a note
// yesterday and one today, with git-standup already answered.
func newDailyFixture(t *testing.T) (dailyModel, *fakeDailySource) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/entries", 0o755); err != nil {
		t.Fatal(err)
	}
	for slug, body := range map[string]string{
		"2026-07-30": "# 2026-07-30\n\n**09:12** - opened the parser\n",
		"2026-07-29": "# 2026-07-29\n\n**17:05** - shipped the migration\n",
	} {
		if err := os.WriteFile(dir+"/entries/"+slug+".md", []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	source := &fakeDailySource{
		journal: journal.New(dir, func() time.Time { return now }),
		report:  sampleReport(),
	}

	m := newDailyModel(Default, source, journal.DefaultDailyDays, 140, 40)
	// Answer the run Init would have started, so the fixture is a loaded view.
	model, _ := m.Update(standupLoadedMsg{report: source.report})
	return model.(dailyModel), source
}

// TestDailyShowsBothHalves is the behaviour the view exists for: yesterday and
// today's notes next to the commits of the same window.
func TestDailyShowsBothHalves(t *testing.T) {
	m, _ := newDailyFixture(t)
	view := m.View()

	for _, want := range []string{
		"DAILY", "Thursday 30 July 2026",
		// The journal half, with the days named rather than only dated.
		"Today", "Yesterday", "09:12", "opened the parser", "17:05", "shipped the migration",
		// The git half, by repository.
		"GIT", "stats-service", "mailer-service", "676ee18", "chore(log): bump the library",
		// And the totals of both.
		"2 notes", "3 commits",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the view should show %q:\n%s", want, view)
		}
	}
}

// TestDailyPutsTheTimeInAGutter: the time is what a day is scanned by, so it sits
// beside its note rather than buried above it — and gives the column up only when
// the pane is too narrow to afford one.
func TestDailyPutsTheTimeInAGutter(t *testing.T) {
	m, _ := newDailyFixture(t)

	line := ""
	for _, candidate := range strings.Split(m.renderNotes(), "\n") {
		if strings.Contains(candidate, "09:12") {
			line = candidate
		}
	}
	if line == "" {
		t.Fatal("the note's time should be rendered")
	}
	if !strings.Contains(line, "opened the parser") {
		t.Errorf("the text should start on the time's line, got %q", line)
	}

	// Two notes of the same day are told apart by a blank line between them.
	notes := m.renderNotes()
	if !strings.Contains(notes, "\n\n") {
		t.Errorf("notes should be spaced out:\n%s", notes)
	}

	// Narrow, the time goes back to a line of its own rather than squeezing the
	// text into nothing.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 30})
	narrow := model.(dailyModel)
	for _, candidate := range strings.Split(narrow.renderNotes(), "\n") {
		if strings.Contains(candidate, "09:12") && strings.Contains(candidate, "opened the parser") {
			t.Errorf("a narrow pane should not keep the gutter, got %q", candidate)
		}
	}
	if !strings.Contains(narrow.renderNotes(), "opened the parser") {
		t.Error("the text should still be there")
	}
}

func TestDailyRunsTheStandupOverTheSameWindow(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	source := &fakeDailySource{journal: journal.New(dir, func() time.Time { return now })}

	m := newDailyModel(Default, source, 3, 140, 40)
	init := m.Init()
	if init == nil {
		t.Fatal("Init should start the run")
	}
	// Init returns the commands; running them is what the runtime would do.
	runCmd(init)

	if source.standupCalls != 1 {
		t.Fatalf("standup ran %d times, want the one Init started", source.standupCalls)
	}
	if source.standupDays != 3 {
		t.Errorf("standup covered %d days, want the view's 3", source.standupDays)
	}
	if got := m.renderSubtitle(); !strings.Contains(got, "Last 4 days") {
		t.Errorf("subtitle = %q, want the window length", got)
	}
}

func TestDailyShowsTheGitPaneLoading(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	source := &fakeDailySource{journal: journal.New(dir, func() time.Time { return now })}

	m := newDailyModel(Default, source, 1, 140, 40)
	if !m.loading {
		t.Fatal("a fresh view is waiting on git-standup")
	}
	if view := m.View(); !strings.Contains(view, "Reading git activity") {
		t.Errorf("the wait should be visible:\n%s", view)
	}

	model, _ := m.Update(standupLoadedMsg{report: sampleReport()})
	loaded := model.(dailyModel)
	if loaded.loading {
		t.Error("the report arrived, so the view is no longer loading")
	}
	if view := loaded.View(); strings.Contains(view, "Reading git activity") {
		t.Errorf("the wait should be gone:\n%s", view)
	}
}

// TestDailyExplainsAMissingHelper: git-standup is optional, so its absence is a
// hint in the git pane and never the end of the view.
func TestDailyExplainsAMissingHelper(t *testing.T) {
	m, _ := newDailyFixture(t)
	model, _ := m.Update(standupLoadedMsg{err: standup.ErrNotInstalled})
	view := model.(dailyModel).View()

	if !strings.Contains(view, standup.Binary+" is not installed") {
		t.Errorf("the missing helper should be named:\n%s", view)
	}
	if !strings.Contains(view, "brew install git-standup") {
		t.Errorf("the hint should say how to get it:\n%s", view)
	}
	// The journal half is unaffected.
	if !strings.Contains(view, "opened the parser") {
		t.Errorf("the notes should still be there:\n%s", view)
	}
}

func TestDailyShowsAnUnconfiguredProjectsDir(t *testing.T) {
	m, _ := newDailyFixture(t)
	model, _ := m.Update(standupLoadedMsg{err: errTest})
	view := model.(dailyModel).View()

	if !strings.Contains(view, "boom") {
		t.Errorf("the reason should be shown:\n%s", view)
	}
	if !strings.Contains(view, "opened the parser") {
		t.Errorf("the notes should still be there:\n%s", view)
	}
}

func TestDailyShowsAnEmptyGitWindow(t *testing.T) {
	m, _ := newDailyFixture(t)
	model, _ := m.Update(standupLoadedMsg{report: standup.Report{Dir: "/Users/dev/projects", Days: 1}})
	if view := model.(dailyModel).View(); !strings.Contains(view, "No commit in this window") {
		t.Errorf("an empty report should say so:\n%s", view)
	}
}

// TestDailyCapturesWhatIsTyped is the other half of the view's purpose: a note
// goes into today's entry without leaving the stand-up.
func TestDailyCapturesWhatIsTyped(t *testing.T) {
	m, source := newDailyFixture(t)

	m = send(t, m, "i")
	if !m.typing {
		t.Fatal("i should give the keyboard to the capture box")
	}
	if !strings.Contains(m.View(), "add to today") {
		t.Error("the footer should explain how to submit")
	}

	m = send(t, m, "d", "o", "n", "e")
	if got := m.input.Value(); got != "done" {
		t.Fatalf("the box holds %q, want the typed text", got)
	}

	m = send(t, m, "enter")
	if len(source.captured) != 1 || source.captured[0] != "done" {
		t.Fatalf("captured = %v, want one line of \"done\"", source.captured)
	}
	if m.input.Value() != "" {
		t.Errorf("the box should be cleared for the next line, holds %q", m.input.Value())
	}
	if !m.typing {
		t.Error("capturing a line should leave the box ready for another")
	}
	if !strings.Contains(m.View(), "Added to 2026-07-30.md") {
		t.Errorf("the capture should be reported:\n%s", m.View())
	}
	// The new note is on screen, which is what makes the capture believable.
	if !strings.Contains(m.View(), "done") {
		t.Errorf("the note should show up in the journal pane:\n%s", m.View())
	}
	if got := m.daily.Notes(); got != 3 {
		t.Errorf("Notes = %d, want the new one counted", got)
	}
}

func TestDailyIgnoresAnEmptyCapture(t *testing.T) {
	m, source := newDailyFixture(t)
	m = send(t, m, "i", " ", "enter")

	if len(source.captured) != 0 {
		t.Errorf("captured = %v, want nothing for blank input", source.captured)
	}
	if !m.typing {
		t.Error("an empty submit should leave the box open")
	}
}

func TestDailyReportsACaptureFailure(t *testing.T) {
	m, source := newDailyFixture(t)
	source.captureErr = errTest

	m = send(t, m, "i", "x", "enter")
	if !strings.Contains(m.View(), "boom") {
		t.Errorf("the failure should be shown:\n%s", m.View())
	}
	// What was typed is kept, so it is not lost with the error.
	if got := m.input.Value(); got != "x" {
		t.Errorf("the box holds %q, want the text kept after a failure", got)
	}
}

// TestDailyTypingSwallowsShortcuts: while the box has the keyboard, q and e are
// text, and esc gives the keyboard back.
func TestDailyTypingSwallowsShortcuts(t *testing.T) {
	m, source := newDailyFixture(t)
	m = send(t, m, "i", "q", "e")

	if m.quit {
		t.Fatal("q is a letter while typing")
	}
	if source.editCalls != 0 {
		t.Fatal("e is a letter while typing")
	}
	if got := m.input.Value(); got != "qe" {
		t.Errorf("the box holds %q, want \"qe\"", got)
	}

	m = send(t, m, "esc")
	if m.typing {
		t.Error("esc should stop typing")
	}
	// And now the shortcuts mean what they say.
	if m = send(t, m, "q"); !m.quit {
		t.Error("q should quit once the box is closed")
	}
}

func TestDailyQuitsWithCtrlCWhileTyping(t *testing.T) {
	m, _ := newDailyFixture(t)
	m = send(t, m, "i", "ctrl+c")
	if !m.quit {
		t.Error("ctrl+c should quit even while typing")
	}
	if m.View() != "" {
		t.Error("a quitting model should render nothing")
	}
}

// TestDailyEditsTodaysEntry covers the shortcut for the times a line in a box is
// not enough.
func TestDailyEditsTodaysEntry(t *testing.T) {
	for _, pressed := range []string{"e", "enter"} {
		m, source := newDailyFixture(t)
		_, cmd := m.Update(keyMsg(pressed))
		if cmd == nil {
			t.Fatalf("%q should return a command that runs the editor", pressed)
		}
		if source.editCalls != 1 {
			t.Errorf("%q asked for %d edit commands, want 1", pressed, source.editCalls)
		}
		if got := source.lastEdited.Format("2006-01-02"); got != "2026-07-30" {
			t.Errorf("%q edited %s, want today", pressed, got)
		}
	}
}

func TestDailyReloadsAfterEditing(t *testing.T) {
	m, source := newDailyFixture(t)
	appendTo(t, source.journal.EntriesDir()+"/2026-07-30.md", "\n**11:30** - added by the editor\n")

	model, _ := m.Update(editorFinishedMsg{})
	after := model.(dailyModel)

	if got := after.daily.Notes(); got != 3 {
		t.Errorf("Notes = %d, want the editor's note picked up", got)
	}
	view := after.View()
	if !strings.Contains(view, "Saved 2026-07-30.md") {
		t.Errorf("a successful edit should be reported:\n%s", view)
	}
	if !strings.Contains(view, "added by the editor") {
		t.Errorf("the new note should be on screen:\n%s", view)
	}
}

func TestDailyReportsEditorFailure(t *testing.T) {
	m, _ := newDailyFixture(t)
	model, _ := m.Update(editorFinishedMsg{err: errTest})
	if !strings.Contains(model.(dailyModel).View(), "boom") {
		t.Error("an editor failure should be shown")
	}
}

func TestDailyRefreshRunsTheStandupAgain(t *testing.T) {
	m, source := newDailyFixture(t)
	before := source.standupCalls

	model, cmd := m.Update(keyMsg("r"))
	refreshed := model.(dailyModel)
	if !refreshed.loading {
		t.Error("a refresh puts the git pane back to loading")
	}
	if cmd == nil {
		t.Fatal("r should return the commands that reload")
	}
	// The commands are what re-run the helper; the runtime would deliver their
	// messages back.
	runCmd(cmd)
	if source.standupCalls <= before {
		t.Errorf("standup ran %d times, want another run", source.standupCalls)
	}

	// Pressing r again while it runs must not pile up runs.
	calls := source.standupCalls
	model, cmd = refreshed.Update(keyMsg("r"))
	if cmd != nil {
		t.Error("a refresh while loading should do nothing")
	}
	_ = model
	if source.standupCalls != calls {
		t.Errorf("standup ran %d times, want it left alone", source.standupCalls)
	}
}

func TestDailyRefreshPicksUpAnEditedEntry(t *testing.T) {
	m, source := newDailyFixture(t)
	appendTo(t, source.journal.EntriesDir()+"/2026-07-30.md", "\n**11:45** - written elsewhere\n")

	model, _ := m.Update(keyMsg("r"))
	if got := model.(dailyModel).daily.Notes(); got != 3 {
		t.Errorf("Notes = %d, want the outside change picked up", got)
	}
}

func TestDailyScrollsTheFocusedPane(t *testing.T) {
	m, _ := newDailyFixture(t)
	// A short terminal is what makes the panes scrollable at all.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 20})
	m = model.(dailyModel)

	if m.focus != dailyNotesPane {
		t.Fatalf("focus = %d, want the journal pane first", m.focus)
	}
	m = send(t, m, "tab")
	if m.focus != dailyGitPane {
		t.Errorf("tab should move to the git pane, focus = %d", m.focus)
	}

	// Scrolling one pane leaves the other where it was.
	m = send(t, m, "G")
	if m.git.YOffset == 0 && m.git.TotalLineCount() > m.git.Height {
		t.Error("G should scroll the git pane to the bottom")
	}
	if m.notes.YOffset != 0 {
		t.Errorf("the journal pane moved to %d while the git pane had focus", m.notes.YOffset)
	}

	m = send(t, m, "tab")
	if m.focus != dailyNotesPane {
		t.Error("tab should come back to the journal pane")
	}
}

// TestDailyDropsAPaneWhenNarrow checks the responsive fallback: below two
// columns the view shows the focused pane alone rather than squeezing both.
func TestDailyDropsAPaneWhenNarrow(t *testing.T) {
	m, _ := newDailyFixture(t)

	for _, tc := range []struct {
		width int
		both  bool
	}{{140, true}, {80, true}, {60, false}, {40, false}} {
		model, _ := m.Update(tea.WindowSizeMsg{Width: tc.width, Height: 30})
		sized := model.(dailyModel)
		if sized.showBothPanes() != tc.both {
			t.Errorf("width %d: both panes = %v, want %v", tc.width, sized.showBothPanes(), tc.both)
		}
		if sized.View() == "" {
			t.Errorf("width %d rendered nothing", tc.width)
		}
		if sized.notes.Width < minDailyColumnWidth {
			t.Errorf("width %d: journal pane shrank to %d", tc.width, sized.notes.Width)
		}
	}

	// Narrow, tab is what reaches the git pane, and it is the one on screen.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	narrow := send(t, model.(dailyModel), "tab")
	view := narrow.View()
	if !strings.Contains(view, "GIT") {
		t.Errorf("the git pane should be the one shown:\n%s", view)
	}
	if strings.Contains(view, "JOURNAL") {
		t.Errorf("only one pane fits, so the journal one should be gone:\n%s", view)
	}
}

func TestDailyEmptyJournalHint(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	source := &fakeDailySource{journal: journal.New(dir, func() time.Time { return now })}

	m := newDailyModel(Default, source, 1, 140, 40)
	model, _ := m.Update(standupLoadedMsg{report: sampleReport()})
	view := model.(dailyModel).View()

	if !strings.Contains(view, "nothing written") {
		t.Errorf("a day without an entry should say so:\n%s", view)
	}
	if !strings.Contains(view, "to add a line") {
		t.Errorf("an empty window should point at the capture box:\n%s", view)
	}
}

func TestRenderDailyStatic(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/entries", 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# 2026-07-30\n\n**09:12** - opened the parser\n"
	if err := os.WriteFile(dir+"/entries/2026-07-30.md", []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	j := journal.New(dir, func() time.Time { return now })

	out := RenderDailyStatic(Default, j.Daily(journal.DailyOptions{}), sampleReport(), nil, 80)
	for _, want := range []string{
		"Daily", "Thursday 30 July 2026", "JOURNAL", "09:12", "opened the parser",
		"Wednesday 29 July", "nothing written",
		"GIT", "stats-service", "676ee18", "chore(log): bump the library",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("static output should hold %q:\n%s", want, out)
		}
	}

	// A missing helper is a hint there too, not an empty section.
	out = RenderDailyStatic(Default, j.Daily(journal.DailyOptions{}), standup.Report{}, standup.ErrNotInstalled, 80)
	if !strings.Contains(out, standup.Binary+" is not installed") {
		t.Errorf("the missing helper should be named:\n%s", out)
	}
	if !strings.Contains(out, "opened the parser") {
		t.Errorf("the journal half should still be printed:\n%s", out)
	}

	// So is any other reason the git half is missing.
	out = RenderDailyStatic(Default, j.Daily(journal.DailyOptions{}), standup.Report{}, errTest, 80)
	if !strings.Contains(out, "boom") {
		t.Errorf("the reason should be printed:\n%s", out)
	}
}
