package ui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/standup"
)

// The mouse hit tests are arithmetic over the layout, so what they claim is
// checked against the rendered view rather than against itself: a rectangle that
// drifts from what is on screen fails here.

// click is the message Bubble Tea delivers for a left click.
func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

// scroll is the message for one notch of the wheel, up when delta is negative.
func scroll(x, y, delta int) tea.MouseMsg {
	button := tea.MouseButtonWheelDown
	if delta < 0 {
		button = tea.MouseButtonWheelUp
	}
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: button}
}

// viewRow returns one row of a rendered view, padded to width so any column of
// the screen can be read. Nothing is styled here — lipgloss only colours a real
// terminal — so a column of the string is a column of the screen.
func viewRow(view string, y, width int) string {
	lines := strings.Split(view, "\n")
	row := ""
	if y >= 0 && y < len(lines) {
		row = lines[y]
	}
	if pad := width - lipgloss.Width(row); pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	return row
}

// viewCell returns width columns of a rendered view at a screen position.
func viewCell(view string, x, y, width int) string {
	return string([]rune(viewRow(view, y, x+width))[x : x+width])
}

// --- calendar ----------------------------------------------------------------

// dayCell is where the cell of a day of the month sits on screen.
func dayCell(m calendarModel, day int) (x, y int) {
	grid := m.gridRect()
	for week, days := range m.calendar.Weeks {
		for i, cell := range days {
			if !cell.Padding() && cell.Date.Day() == day {
				return grid.x + i*dayCellWidth, grid.y + week
			}
		}
	}
	return -1, -1
}

// TestCalendarGridRectMatchesTheRenderedGrid pins the hit test to the render: the
// cell the model says holds a day is the cell the day is drawn in.
func TestCalendarGridRectMatchesTheRenderedGrid(t *testing.T) {
	m, _, _ := newCalendarFixture(t)
	view, grid := m.View(), m.gridRect()

	if got := len(m.calendar.Weeks); grid.h != got {
		t.Fatalf("grid covers %d rows, want the %d weeks of the month", grid.h, got)
	}
	for week, days := range m.calendar.Weeks {
		for i, day := range days {
			want := ""
			if !day.Padding() {
				want = strconv.Itoa(day.Date.Day())
			}
			got := strings.TrimSpace(viewCell(view, grid.x+i*dayCellWidth, grid.y+week, dayCellWidth))
			if got != want {
				t.Errorf("week %d, column %d shows %q, want %q", week, i, got, want)
			}
		}
	}
}

func TestCalendarClickPicksADay(t *testing.T) {
	m, _, _ := newCalendarFixture(t)

	x, y := dayCell(m, 1)
	model, _ := m.Update(click(x+1, y))
	m = model.(calendarModel)
	if got := m.selected.Format("2006-01-02"); got != "2026-07-01" {
		t.Errorf("clicking the 1st selected %s", got)
	}
	// The whole four-column cell answers, not only the digits.
	x, y = dayCell(m, 15)
	model, _ = m.Update(click(x, y))
	if got := model.(calendarModel).selected.Format("2006-01-02"); got != "2026-07-15" {
		t.Errorf("the left edge of a cell should pick its day, got %s", got)
	}

	// The padding before the 1st holds no day, and neither does the entry pane.
	grid := m.gridRect()
	for _, at := range [][2]int{{grid.x, grid.y}, {m.entryPaneX() + 4, grid.y}} {
		model, _ = m.Update(click(at[0], at[1]))
		if got := model.(calendarModel).selected; !got.Equal(m.selected) {
			t.Errorf("clicking %v moved the selection to %s", at, got)
		}
	}
}

// TestCalendarHintsRectMatchesTheRenderedHints checks the NAVIGATE rows are where
// the model says, including when the status line above them pushes them down.
func TestCalendarHintsRectMatchesTheRenderedHints(t *testing.T) {
	base, source, today := newCalendarFixture(t)
	source.dayReports = map[string]standup.Report{today.Format("2006-01-02"): sampleDayReport()}
	withStatus, withErr := base, base
	withStatus.status = "Saved 2026-07-28.md"
	withErr.err = errTest

	for name, m := range map[string]calendarModel{
		"plain":              base,
		"with a status":      withStatus,
		"with a failure":     withErr,
		"with the git panel": openGitPanel(t, base),
	} {
		view, hints := m.View(), m.hintsRect()
		label := viewCell(view, firstPaneX+paneContentInset, hints.y-1, len("NAVIGATE"))
		if label != "NAVIGATE" {
			t.Errorf("%s: the row above the hints reads %q, want the NAVIGATE label", name, label)
		}
		for i, hint := range m.hints() {
			row := viewRow(view, hints.y+i, m.width)
			if !strings.Contains(row, hint[0]) || !strings.Contains(row, hint[1]) {
				t.Errorf("%s: row %d reads %q, want the %q hint", name, i, row, hint[0])
			}
			if got := viewCell(view, hints.x, hints.y+i, lipgloss.Width(hint[0])); got != hint[0] {
				t.Errorf("%s: the keys of row %d sit at %q, want %q", name, i, got, hint[0])
			}
		}
	}
}

// TestCalendarClickOnAHintPressesIt: the hints are the mouse's keyboard, arrows
// included, and each glyph of a pair answers for itself.
func TestCalendarClickOnAHintPressesIt(t *testing.T) {
	m, source, today := newCalendarFixture(t)
	hints := m.hintsRect()

	rowOf := func(keys string) int {
		row := hintRow(m.hints(), keys)
		if row < 0 {
			t.Fatalf("no %q hint", keys)
		}
		return hints.y + row
	}

	// The right arrow of "← →" moves a day on, the left one back.
	model, _ := m.Update(click(hints.x+2, rowOf("← →")))
	if got := model.(calendarModel).selected.Format("2006-01-02"); got != "2026-07-29" {
		t.Errorf("clicking → selected %s, want 2026-07-29", got)
	}
	model, _ = model.(calendarModel).Update(click(hints.x, rowOf("← →")))
	if got := model.(calendarModel).selected; !got.Equal(today) {
		t.Errorf("clicking ← should come back to %s, got %s", today, got)
	}

	// Each key of "n p" pages its own way.
	model, _ = m.Update(click(hints.x, rowOf("n p")))
	if got := model.(calendarModel).month.Month.String(); got != "August" {
		t.Errorf("clicking n should page on, got %s", got)
	}
	model, _ = m.Update(click(hints.x+2, rowOf("n p")))
	if got := model.(calendarModel).month.Month.String(); got != "June" {
		t.Errorf("clicking p should page back, got %s", got)
	}

	// A hint naming a single key answers for its label too.
	model, _ = model.(calendarModel).Update(click(hints.x+12, rowOf("t")))
	if got := model.(calendarModel); !got.selected.Equal(today) {
		t.Errorf("clicking the today hint selected %s", got.selected)
	}

	if _, cmd := m.Update(click(hints.x, rowOf("e"))); cmd == nil || source.editCalls != 1 {
		t.Errorf("clicking the edit hint should run the editor, %d calls", source.editCalls)
	}
	model, _ = m.Update(click(hints.x, rowOf("q")))
	if !model.(calendarModel).quit {
		t.Error("clicking the quit hint should quit")
	}
}

func TestCalendarWheelPagesMonthsOverTheGridAndScrollsTheEntry(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	// A long entry, so the centre pane has somewhere to scroll to.
	appendTo(t, source.journal.EntriesDir()+"/2026-07-28.md",
		strings.Repeat("\nanother line of the day\n", 60))
	m.reload()

	grid := m.gridRect()
	model, _ := m.Update(scroll(grid.x, grid.y, 1))
	paged := model.(calendarModel)
	if got := paged.month.Month.String(); got != "August" {
		t.Errorf("the wheel over the grid should page to August, got %s", got)
	}
	if paged.entry.YOffset != 0 {
		t.Errorf("paging a month should not scroll the entry, offset %d", paged.entry.YOffset)
	}

	entry := m.entryBand()
	model, _ = m.Update(scroll(entry.x+2, grid.y, 1))
	scrolled := model.(calendarModel)
	if scrolled.entry.YOffset != wheelLines {
		t.Errorf("the wheel over the entry scrolled %d lines, want %d", scrolled.entry.YOffset, wheelLines)
	}
	if scrolled.month != m.month {
		t.Errorf("scrolling the entry should not change month, got %v", scrolled.month)
	}
	model, _ = scrolled.Update(scroll(entry.x+2, grid.y, -1))
	if got := model.(calendarModel).entry.YOffset; got != 0 {
		t.Errorf("the wheel back up left the entry at %d, want the top", got)
	}
}

// hintRow is the index of a hint in a table, or -1 when it is not up.
func hintRow(hints [][2]string, keys string) int {
	for i, hint := range hints {
		if hint[0] == keys {
			return i
		}
	}
	return -1
}

// busyDayReport is a day with more commits than a panel can hold.
func busyDayReport(commits int) standup.Report {
	repo := standup.Repo{Path: "/p/busy-service"}
	for i := range commits {
		repo.Commits = append(repo.Commits, standup.Commit{
			Hash:    fmt.Sprintf("c%06d", i),
			Subject: "chore: another one",
			When:    "09:00",
		})
	}
	return standup.Report{Repos: []standup.Repo{repo}}
}

// TestCalendarGitPanelMouse: the panel is scrolled with the wheel over it, and its
// own hint row answers to a click like every other.
func TestCalendarGitPanelMouse(t *testing.T) {
	m, source, today := newCalendarFixture(t)
	source.dayReports = map[string]standup.Report{today.Format("2006-01-02"): busyDayReport(40)}
	m = openGitPanel(t, m)

	// The panel is under the notes at this width, so its rows are its own.
	git := m.gitBand()
	if got := viewCell(m.View(), git.x+paneContentInset, git.y+paneBorder, 3); got != "GIT" {
		t.Fatalf("the panel's heading reads %q, want it at the top of its band", got)
	}
	model, _ := m.Update(scroll(git.x+paneContentInset, git.y+paneBorder, 1))
	scrolled := model.(calendarModel)
	if scrolled.git.YOffset != wheelLines {
		t.Errorf("the wheel over the panel scrolled %d lines, want %d", scrolled.git.YOffset, wheelLines)
	}
	if scrolled.entry.YOffset != 0 {
		t.Error("the markdown should not have moved")
	}
	if scrolled.month != m.month {
		t.Errorf("the wheel over the panel paged the month to %v", scrolled.month)
	}

	// K scrolls it back, and leaves the notes where they were.
	back := send(t, scrolled, "K")
	if got := back.git.YOffset; got != wheelLines-1 {
		t.Errorf("K left the panel at %d, want %d", got, wheelLines-1)
	}
	if back.entry.YOffset != 0 {
		t.Error("K should not have moved the notes")
	}

	// Wide enough for the panel to sit beside the notes, its band moves with it.
	model, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 32})
	wide := model.(calendarModel)
	if !wide.gitBeside() {
		t.Fatal("160 columns should hold the panel beside the notes")
	}
	beside := wide.gitBand()
	if got := viewCell(wide.View(), beside.x+paneContentInset, calendarPaneY+paneBorder, 3); got != "GIT" {
		t.Fatalf("the panel's heading reads %q, want it at the left of its band", got)
	}
	model, _ = wide.Update(scroll(beside.x+paneContentInset, calendarPaneY+paneBorder+2, 1))
	if got := model.(calendarModel).git.YOffset; got != wheelLines {
		t.Errorf("the wheel beside the notes scrolled %d lines, want %d", got, wheelLines)
	}
	if got := model.(calendarModel).entry.YOffset; got != 0 {
		t.Errorf("the notes moved to %d instead", got)
	}
}

// TestCalendarClickOpensTheGitPanel: the panel's own hint puts it up.
func TestCalendarClickOpensTheGitPanel(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	hints := m.hintsRect()

	model, cmd := m.Update(click(hints.x+4, hints.y+hintRow(m.hints(), "s")))
	opened := model.(calendarModel)
	if !opened.gitOpen {
		t.Fatal("clicking the git hint should put the panel up")
	}
	if _, ok := standupAnswer(t, cmd); !ok {
		t.Error("opening the panel should ask for the day's git activity")
	}
	if source.dayStandupCalls != 1 {
		t.Errorf("%d runs, want the one for the selected day", source.dayStandupCalls)
	}
}

// --- review ------------------------------------------------------------------

// cardRow is the screen row a column's card starts on.
func cardRow(m reviewModel, column, note int) int {
	c := m.columns[column]
	return reviewPanesY + paneBorder + reviewCardsRow + c.offsets[note] - c.view.YOffset
}

// TestReviewColumnRectsMatchTheRenderedPanes pins the column geometry to the
// render: each mood's heading sits at the top of the rectangle claimed for it.
func TestReviewColumnRectsMatchTheRenderedPanes(t *testing.T) {
	m, _ := reviewFixture(t)
	view := m.View()

	if !m.showAllColumns() {
		t.Fatal("the fixture is wide enough for the three columns")
	}
	for i := range m.columns {
		pane := m.columnPaneRect(i)
		heading := moodGlyph(m.columns[i].mood) + " " + m.columns[i].mood.Title()
		got := viewCell(view, pane.x+paneContentInset, pane.y+paneBorder, lipgloss.Width(heading))
		if got != heading {
			t.Errorf("column %d starts with %q, want the %q heading", i, got, heading)
		}
	}
	// And the first card of the focused column is where cardRow says it is.
	if got := viewRow(view, cardRow(m, 0, 0), m.width); !strings.Contains(got, "20 Jul") {
		t.Errorf("the first #good card reads %q, want its date", got)
	}
}

func TestReviewClickPicksAColumnAndANote(t *testing.T) {
	m, _ := reviewFixture(t)
	bad := m.columnPaneRect(1)

	// The second #bad note, in a column the keyboard is not on.
	model, _ := m.Update(click(bad.x+paneContentInset, cardRow(m, 1, 1)))
	clicked := model.(reviewModel)
	if clicked.focus != 1 {
		t.Errorf("focus = %d, want the column that was clicked", clicked.focus)
	}
	if got := clicked.columns[1].selected; got != 1 {
		t.Errorf("selected = %d, want the note that was clicked", got)
	}
	if !strings.Contains(clicked.View(), "2/2") {
		t.Error("the heading should count the clicked note")
	}

	// Clicking a column beside its cards takes the focus without moving anything.
	mad := m.columnPaneRect(2)
	model, _ = clicked.Update(click(mad.x+paneContentInset, mad.y+mad.h-2))
	moved := model.(reviewModel)
	if moved.focus != 2 {
		t.Errorf("focus = %d, want the third column", moved.focus)
	}
	if got := moved.columns[1].selected; got != 1 {
		t.Errorf("the second column's selection moved to %d", got)
	}

	// Outside the panes nothing happens.
	model, _ = moved.Update(click(0, 0))
	if got := model.(reviewModel).focus; got != 2 {
		t.Errorf("a click on the title moved the focus to %d", got)
	}
}

func TestReviewClickOnAFooterHint(t *testing.T) {
	m, source := reviewFixture(t)
	footer := reviewPanesY + m.paneHeight() + 1

	// The footer the hit test reads is the footer on screen.
	if got := viewRow(m.View(), footer, m.width); !strings.Contains(got, "mood") {
		t.Fatalf("row %d reads %q, want the hints", footer, got)
	}

	period := m.review.Period()
	model, _ := m.Update(click(firstPaneX+lipgloss.Width("← →  mood  ·  ↑ ↓  note  ·  e edit  ·  p"), footer))
	if got := model.(reviewModel).review.Period(); got == period {
		t.Errorf("clicking p should page the period back, still %s", got)
	}

	if _, cmd := m.Update(click(firstPaneX+lipgloss.Width("← →  mood  ·  ↑ ↓  note  ·  e"), footer)); cmd == nil || source.editCalls != 1 {
		t.Errorf("clicking the edit hint should run the editor, %d calls", source.editCalls)
	}

	// A footer reporting an outcome has no hints to click.
	m.status = "Saved 2026-07-28.md"
	if key, ok := m.footerHintAt(firstPaneX, footer); ok {
		t.Errorf("the status footer answered %q", key)
	}
}

func TestReviewWheelScrollsTheColumnUnderThePointer(t *testing.T) {
	m, _ := reviewFixture(t)
	// A short pane, so a column of three notes has somewhere to scroll to.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 16})
	m = model.(reviewModel)

	good := m.columnPaneRect(0)
	model, _ = m.Update(scroll(good.x+paneContentInset, good.y+good.h-2, 1))
	scrolled := model.(reviewModel)
	if scrolled.columns[0].view.YOffset == 0 {
		t.Error("the wheel over a column should scroll it")
	}
	if scrolled.focus != 0 || scrolled.columns[0].selected != 0 {
		t.Error("scrolling should not move the focus or the selection")
	}

	// Over another column it scrolls that one, still without taking the keyboard.
	bad := m.columnPaneRect(1)
	model, _ = m.Update(scroll(bad.x+paneContentInset, bad.y+bad.h-2, 1))
	other := model.(reviewModel)
	if other.columns[0].view.YOffset != 0 {
		t.Error("the wrong column scrolled")
	}
	if other.focus != 0 {
		t.Errorf("focus = %d, the wheel should not move it", other.focus)
	}
}

// --- daily -------------------------------------------------------------------

// TestDailyPaneRectsMatchTheRenderedPanes pins the pane geometry to the render.
func TestDailyPaneRectsMatchTheRenderedPanes(t *testing.T) {
	m, _ := newDailyFixture(t)
	view := m.View()

	if !m.showBothPanes() {
		t.Fatal("the fixture is wide enough for both panes")
	}
	for index, want := range map[int]string{dailyNotesPane: "JOURNAL", dailyGitPane: "GIT"} {
		pane := m.paneRect(index)
		got := viewCell(view, pane.x+paneContentInset, pane.y+paneBorder, len(want))
		if got != want {
			t.Errorf("pane %d starts with %q, want %s", index, got, want)
		}
	}

	// The capture box and the footer under it.
	capture := m.captureRect()
	if got := viewRow(view, capture.y+1, m.width); !strings.Contains(got, captureHint) {
		t.Errorf("row %d reads %q, want the capture box", capture.y+1, got)
	}
	if got := viewRow(view, capture.y+capture.h, m.width); !strings.Contains(got, "type a note") {
		t.Errorf("row %d reads %q, want the footer hints", capture.y+capture.h, got)
	}
}

func TestDailyClickMovesTheFocusAndTheKeyboard(t *testing.T) {
	m, _ := newDailyFixture(t)

	git := m.paneRect(dailyGitPane)
	model, _ := m.Update(click(git.x+paneContentInset, git.y+2))
	focused := model.(dailyModel)
	if focused.focus != dailyGitPane {
		t.Errorf("focus = %d, want the git pane", focused.focus)
	}

	// Clicking the capture box asks for the keyboard, clicking a pane gives it back.
	capture := m.captureRect()
	model, _ = focused.Update(click(capture.x+paneContentInset, capture.y+1))
	typing := model.(dailyModel)
	if !typing.typing {
		t.Error("clicking the capture box should start typing")
	}
	if !strings.Contains(typing.View(), "add to today") {
		t.Error("the footer should switch to the capture box's keys")
	}

	notes := m.paneRect(dailyNotesPane)
	model, _ = typing.Update(click(notes.x+paneContentInset, notes.y+2))
	back := model.(dailyModel)
	if back.typing {
		t.Error("clicking away from the box should stop typing")
	}
	if back.focus != dailyNotesPane {
		t.Errorf("focus = %d, want the journal pane", back.focus)
	}
}

func TestDailyClickOnAFooterHint(t *testing.T) {
	m, source := newDailyFixture(t)
	footer := m.captureRect().y + m.captureRect().h

	model, _ := m.Update(click(firstPaneX, footer))
	typing := model.(dailyModel)
	if !typing.typing {
		t.Error("clicking the i hint should start typing")
	}

	// While typing, the footer hints are the box's own: enter captures the line.
	typing.input.SetValue("a line from the mouse")
	model, _ = typing.Update(click(firstPaneX, footer))
	captured := model.(dailyModel)
	if len(source.captured) != 1 || source.captured[0] != "a line from the mouse" {
		t.Errorf("clicking the enter hint captured %v", source.captured)
	}
	if !captured.typing {
		t.Error("capturing a line should leave the box ready for the next one")
	}

	m.Update(click(firstPaneX+lipgloss.Width("i type a note  ·  e"), footer))
	if source.editCalls != 1 {
		t.Errorf("clicking the edit hint should run the editor, %d calls", source.editCalls)
	}
}

func TestDailyWheelScrollsThePaneUnderThePointer(t *testing.T) {
	m, _ := newDailyFixture(t)
	// A short view, so the panes have somewhere to scroll to.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 18})
	m = model.(dailyModel)

	git := m.paneRect(dailyGitPane)
	model, _ = m.Update(scroll(git.x+paneContentInset, git.y+3, 1))
	scrolled := model.(dailyModel)
	if scrolled.git.YOffset == 0 {
		t.Error("the wheel over the git pane should scroll it")
	}
	if scrolled.notes.YOffset != 0 {
		t.Error("the journal pane should not have moved")
	}
	if scrolled.focus != dailyNotesPane {
		t.Errorf("focus = %d, the wheel should not move it", scrolled.focus)
	}

	model, _ = scrolled.Update(scroll(git.x+paneContentInset, git.y+3, -1))
	if got := model.(dailyModel).git.YOffset; got != 0 {
		t.Errorf("the wheel back up left the pane at %d, want the top", got)
	}
}

// TestMouseIsIgnoredOutsideTheViews covers the events that mean nothing: a
// release, a drag, and a middle click.
func TestMouseIsIgnoredOutsideTheViews(t *testing.T) {
	m, _, today := newCalendarFixture(t)
	x, y := dayCell(m, 1)

	for _, msg := range []tea.MouseMsg{
		{X: x, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
		{X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft},
		{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonMiddle},
	} {
		model, cmd := m.Update(msg)
		if got := model.(calendarModel).selected; !got.Equal(today) {
			t.Errorf("%s moved the selection to %s", msg, got)
		}
		if cmd != nil {
			t.Errorf("%s returned a command", msg)
		}
	}
}

// TestMouseIsReArmedAfterTheEditor: handing the terminal over turns mouse
// reporting off, and Bubble Tea does not put it back, so every view has to.
func TestMouseIsReArmedAfterTheEditor(t *testing.T) {
	calendar, _, _ := newCalendarFixture(t)
	review, _ := reviewFixture(t)
	daily, _ := newDailyFixture(t)

	failed := review
	failed.err = errTest

	for name, m := range map[string]tea.Model{
		"calendar":      calendar,
		"review":        review,
		"failed review": failed,
		"daily":         daily,
	} {
		_, cmd := m.Update(editorFinishedMsg{})
		if cmd == nil {
			t.Errorf("%s: no command after the editor, want the mouse back", name)
			continue
		}
		if got := cmd(); got != tea.EnableMouseCellMotion() {
			t.Errorf("%s: the command sends %T, want the mouse back on", name, got)
		}
	}
}

// TestMouseStaysOffAfterTheEditorWhenNotWanted: with MOUSE=false the terminal
// owns the mouse, and coming back from the editor must not take it back, nested
// review included.
func TestMouseStaysOffAfterTheEditorWhenNotWanted(t *testing.T) {
	calendar, source, _ := newCalendarFixture(t)
	calendar.mouse = false
	review, _ := reviewFixture(t)
	review.mouse = false
	daily, _ := newDailyFixture(t)
	daily.mouse = false

	failed := review
	failed.err = errTest

	appendTo(t, source.journal.EntriesDir()+"/2026-07-28.md", "\n**09:00** - shipped it [#good]\n")
	nested := send(t, calendar, "r")
	if nested.review == nil {
		t.Fatal("r should open the review")
	}

	for name, m := range map[string]tea.Model{
		"calendar":      calendar,
		"review":        review,
		"failed review": failed,
		"daily":         daily,
		"nested review": nested,
	} {
		if _, cmd := m.Update(editorFinishedMsg{}); cmd != nil {
			t.Errorf("%s: the editor's return sends %T, want nothing", name, cmd())
		}
	}
}

// TestProgramOptionsLeaveTheMouseToTheTerminal runs a program the way the views
// start one and reads what it asks the terminal for.
func TestProgramOptionsLeaveTheMouseToTheTerminal(t *testing.T) {
	const cellMotion = "\x1b[?1002h"

	for _, mouse := range []bool{true, false} {
		var out strings.Builder
		opts := append(programOptions(mouse), tea.WithInput(nil), tea.WithOutput(&out))
		if _, err := tea.NewProgram(quitAtOnce{}, opts...).Run(); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(out.String(), cellMotion); got != mouse {
			t.Errorf("mouse %v: mouse reporting requested = %v", mouse, got)
		}
		if !strings.Contains(out.String(), "\x1b[?1049h") {
			t.Errorf("mouse %v: the alternate screen should be on either way", mouse)
		}
	}
}

// quitAtOnce is a program that stops as soon as it starts.
type quitAtOnce struct{}

func (quitAtOnce) Init() tea.Cmd                       { return tea.Quit }
func (quitAtOnce) Update(tea.Msg) (tea.Model, tea.Cmd) { return quitAtOnce{}, nil }
func (quitAtOnce) View() string                        { return "" }

// TestReviewMouseWorksNestedInTheCalendar: the review opened from the calendar
// gets the mouse events too, and clicking its footer comes back to the month.
func TestReviewMouseWorksNestedInTheCalendar(t *testing.T) {
	m, source, _ := newCalendarFixture(t)
	appendTo(t, source.journal.EntriesDir()+"/2026-07-28.md", "\n**09:00** - shipped it [#good]\n")

	m = send(t, m, "r")
	if m.review == nil {
		t.Fatal("r should open the review")
	}

	bad := m.review.columnPaneRect(1)
	model, _ := m.Update(click(bad.x+paneContentInset, bad.y+3))
	m = model.(calendarModel)
	if m.review == nil {
		t.Fatal("clicking inside the review should not leave it")
	}
	if m.review.focus != 1 {
		t.Errorf("focus = %d, want the clicked column", m.review.focus)
	}

	// The nested footer's last hint goes back to the calendar rather than quitting.
	hints := m.review.footerHints()
	footer := reviewPanesY + m.review.paneHeight() + 1
	offset := 0
	for i, hint := range hints[:len(hints)-1] {
		offset += lipgloss.Width(hint[0]) + 1 + lipgloss.Width(hint[1])
		if i >= 0 {
			offset += lipgloss.Width("  ·  ")
		}
	}
	model, _ = m.Update(click(firstPaneX+offset, footer))
	back := model.(calendarModel)
	if back.review != nil {
		t.Error("clicking the leave hint should come back to the calendar")
	}
	if back.quit {
		t.Error("leaving the nested review must not quit the program")
	}
}
