package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
)

// CalendarSource supplies the calendar view with everything it shows, so the
// view itself touches neither the filesystem nor the editor. It covers the sprint
// review the calendar can switch to as well.
type CalendarSource interface {
	ReviewSource
	// Calendar lays out one month.
	Calendar(m journal.Month) journal.Calendar
	// Today is the current date at midnight.
	Today() time.Time
	// EntryContent returns a day's markdown and whether the entry exists.
	EntryContent(day time.Time) (string, bool)
	// Insight summarises the journal around a day, for the side panel.
	Insight(day time.Time, opts journal.InsightOptions) journal.Insight
	// ReviewPeriod is the period to review around a day.
	ReviewPeriod(day time.Time) (journal.ReviewOptions, error)
	// DayStandup reads the git activity of one day, for the panel `s` puts up. Its
	// error is shown inside that panel rather than ending the view: the journal is
	// still worth reading when the helper is missing or unconfigured.
	DayStandup(day time.Time) (standup.Report, error)
}

// Panel widths, in content columns. The centre pane takes whatever is left.
const (
	calendarPaneWidth = 28
	insightPaneWidth  = 24
	// minEntryPaneWidth is the narrowest the markdown pane may get before the
	// side panels start being dropped.
	minEntryPaneWidth = 32
)

// Panes of the calendar view the keyboard can be on, in focus order. The git
// panel is not one of them: it comes and goes, and J K reach it from either.
const (
	calendarGridPane = iota
	calendarEntryPane
	calendarPaneCount
)

// editorFinishedMsg is delivered once the foreground editor exits.
type editorFinishedMsg struct{ err error }

// dayStandupLoadedMsg carries the outcome of one git-standup run for a day. The
// day it was asked for travels with it: the selection can move several times
// while the helper walks a directory of repositories, and a report that no longer
// belongs to the day on screen is worth nothing.
type dayStandupLoadedMsg struct {
	day    time.Time
	report standup.Report
	err    error
}

// calendarModel is the Bubble Tea model behind ShowCalendar.
type calendarModel struct {
	theme  *Theme
	source CalendarSource

	month    journal.Month
	calendar journal.Calendar
	selected time.Time
	insight  journal.Insight

	entry viewport.Model
	// hasEntry records whether the selected day holds an entry.
	hasEntry bool
	// entryWords is the selected entry's word count.
	entryWords int

	// focus is the pane j and k act on: a week on the grid, a line in the notes.
	focus int

	// review is the sprint review shown over the calendar, nil while the calendar
	// itself is on screen.
	review *reviewModel

	// gitOpen is whether the git activity panel is up. It stays down until asked
	// for: reading a directory of repositories takes seconds, and the calendar is
	// mostly used without it.
	gitOpen bool
	// git holds the panel's commits, scrolled on its own.
	git viewport.Model
	// gitReport is the day's activity. It always belongs to the selected day: it is
	// dropped as soon as the day moves, and a run that comes back for another day
	// is never stored.
	gitReport standup.Report
	// gitErr is why the panel has nothing to show, if anything.
	gitErr error
	// gitLoading is true while the helper runs. Only one run is in flight at a
	// time, and the day may have moved by the time it comes back.
	gitLoading bool
	gitSpin    spinner.Model

	width, height int
	// mouse is whether the view captures the mouse. Off, clicks and the wheel go
	// to the terminal, which then selects text natively.
	mouse bool

	// status is a transient message shown in place of the hints.
	status string
	// err holds the last editor failure, if any.
	err  error
	quit bool
}

// ShowCalendar runs the full-screen calendar. It returns when the user quits;
// entries are edited in place rather than handed back to the caller.
func ShowCalendar(theme *Theme, source CalendarSource, month journal.Month, mouse bool) error {
	m := newCalendarModel(theme, source, TerminalWidth(), TerminalHeight())
	m.mouse = mouse
	m.setMonth(month)

	// Land on today when it belongs to the requested month, otherwise the 1st.
	today := source.Today()
	if journal.MonthOf(today) == month {
		m.selectDay(today)
	} else {
		m.selectDay(time.Date(month.Year, month.Month, 1, 0, 0, 0, 0, today.Location()))
	}

	_, err := tea.NewProgram(m, programOptions(mouse)...).Run()
	return err
}

func newCalendarModel(theme *Theme, source CalendarSource, width, height int) calendarModel {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	spin.Style = theme.Accent

	return calendarModel{
		theme:   theme,
		source:  source,
		entry:   viewport.New(40, 10),
		git:     viewport.New(minEntryPaneWidth, 10),
		gitSpin: spin,
		width:   width,
		height:  height,
		mouse:   true,
	}
}

func (m calendarModel) Init() tea.Cmd { return nil }

func (m *calendarModel) setMonth(month journal.Month) {
	m.month = month
	m.calendar = m.source.Calendar(month)
}

// selectDay moves the selection, following it into a neighbouring month and
// reloading everything that depends on the day.
func (m *calendarModel) selectDay(day time.Time) {
	m.selected = dateparse.Truncate(day)
	if month := journal.MonthOf(m.selected); month != m.month {
		m.setMonth(month)
	}
	m.insight = m.source.Insight(m.selected, journal.InsightOptions{})
	m.loadEntry()
}

// reload refreshes the month and the day after the entry may have changed on
// disk, which happens when the editor comes back.
func (m *calendarModel) reload() {
	m.setMonth(m.month)
	m.insight = m.source.Insight(m.selected, journal.InsightOptions{})
	m.loadEntry()
}

// loadEntry renders the selected day's markdown into the centre pane.
func (m *calendarModel) loadEntry() {
	m.layout()
	content, ok := m.source.EntryContent(m.selected)
	m.hasEntry = ok
	m.entryWords = 0

	if !ok {
		m.entry.SetContent(m.renderEmptyState())
		m.entry.GotoTop()
		return
	}
	m.entryWords = journal.WordCount(content)
	rendered := RenderMarkdown(m.theme, content, m.entry.Width)
	// Wrap to the pane so a long paragraph cannot push the layout apart.
	m.entry.SetContent(lipgloss.NewStyle().Width(m.entry.Width).Render(rendered))
	m.entry.GotoTop()
}

func (m calendarModel) renderEmptyState() string {
	t := m.theme
	return strings.Join([]string{
		"",
		t.Muted.Render("Nothing written for this day."),
		"",
		t.Muted.Render("Press ") + t.HelpKey.Render("e") + t.Muted.Render(" to start the entry."),
	}, "\n")
}

// --- git panel ---------------------------------------------------------------

// toggleGit puts the git activity panel up or takes it down. Opening it reads the
// selected day, so what it shows is never left over from an earlier look.
func (m *calendarModel) toggleGit() tea.Cmd {
	m.gitOpen = !m.gitOpen
	// The markdown has to be rewrapped to the column it just lost or got, and the
	// reader keeps their place through it.
	offset := m.entry.YOffset
	m.loadEntry()
	m.entry.SetYOffset(offset)
	return m.gitFollow()
}

// gitFollow points the panel at the selected day: what is on screen belongs to
// another day now, so it goes and the day's own activity is asked for.
func (m *calendarModel) gitFollow() tea.Cmd {
	if !m.gitOpen {
		return nil
	}
	m.gitReport, m.gitErr = standup.Report{}, nil
	m.git.GotoTop()
	return m.requestGit()
}

// requestGit asks the source for the selected day's activity, unless a run is
// already in flight: the helper walks every repository it is given, which takes
// seconds, and holding the arrow key down would otherwise start a run per day.
// gitLoaded starts the next one when the day has moved on meanwhile.
func (m *calendarModel) requestGit() tea.Cmd {
	if !m.gitOpen {
		return nil
	}
	if m.gitLoading {
		m.refreshGit()
		return nil
	}
	m.gitLoading = true
	m.refreshGit()

	day, source := m.selected, m.source
	return tea.Batch(m.gitSpin.Tick, func() tea.Msg {
		report, err := source.DayStandup(day)
		return dayStandupLoadedMsg{day: day, report: report, err: err}
	})
}

// gitLoaded takes in a finished run. A report for a day the selection has left is
// dropped rather than shown against the wrong day, and the day now on screen is
// asked for instead.
func (m calendarModel) gitLoaded(msg dayStandupLoadedMsg) (tea.Model, tea.Cmd) {
	m.gitLoading = false
	if !msg.day.Equal(m.selected) {
		return m, m.requestGit()
	}
	m.gitReport, m.gitErr = msg.report, msg.err
	m.refreshGit()
	return m, nil
}

// refreshGit re-renders the panel at the current size, keeping the scroll where
// the reader left it.
func (m *calendarModel) refreshGit() {
	if !m.gitOpen {
		return
	}
	m.git.SetContent(m.renderGitContent())
}

// showInsight reports whether the terminal is wide enough for the side panel.
func (m calendarModel) showInsight() bool {
	return m.width >= calendarPaneWidth+insightPaneWidth+minEntryPaneWidth+14
}

// showCalendarPane reports whether the terminal is wide enough for the grid.
func (m calendarModel) showCalendarPane() bool {
	return m.width >= calendarPaneWidth+minEntryPaneWidth+10
}

// centreWidth is the content width of the middle column, which the notes have to
// themselves until the git panel is put up.
func (m calendarModel) centreWidth() int {
	// The panels are separated by a single space, and each costs paneChrome columns
	// of border and padding.
	width := m.width - 2 // outer margin
	if m.showCalendarPane() {
		width -= calendarPaneWidth + paneChrome + 1
	}
	if m.showInsight() {
		width -= insightPaneWidth + paneChrome + 1
	}
	return max(width-paneChrome, minEntryPaneWidth)
}

// gitBeside reports whether the middle column is wide enough to hold the panel
// next to the notes. The two share the column evenly, so it takes room for two
// panes of the width the markdown asks for; under that the panel goes below the
// notes, which costs height rather than prose width.
func (m calendarModel) gitBeside() bool {
	return m.gitOpen && m.centreWidth() >= 2*minEntryPaneWidth+paneChrome+1
}

// minStackedRows is the shortest a stacked pane may be: its heading, the rule
// under it and three lines of what it holds.
const minStackedRows = 5

// layout recomputes the middle column from the terminal size: the notes alone, the
// notes beside the git panel, or the notes above it.
func (m *calendarModel) layout() {
	width := m.centreWidth()
	rows := m.paneContentHeight()

	m.entry.Width, m.git.Width = width, width
	m.entry.Height, m.git.Height = max(rows-paneHeadings, 3), max(rows-paneHeadings, 3)

	switch {
	case m.gitBeside():
		// Half the column each, the way the stand-up view splits its two panes. An
		// odd column leaves the spare one to the prose.
		m.git.Width = (width - paneChrome - 1) / 2
		m.entry.Width = width - paneChrome - 1 - m.git.Width

	case m.gitOpen:
		// Stacked, the two panes share the column: their content, plus two border
		// rows apiece, has to come out as tall as the panels either side. The panel
		// takes a third of it, and the notes keep the rest — a day is read more often
		// than its commits are counted.
		shared := rows - 2*paneBorder
		git := max(shared/3, minStackedRows)
		if shared-git < minStackedRows {
			git = max(shared-minStackedRows, minStackedRows)
		}
		m.git.Height = max(git-paneHeadings, 1)
		m.entry.Height = max(shared-git-paneHeadings, 1)
	}
}

// paneContentHeight is the height available inside a panel's border.
func (m calendarModel) paneContentHeight() int {
	height := m.height - 4 // top and bottom margin, plus the border
	if height < 6 {
		height = 6
	}
	return height
}

func (m calendarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// The calendar keeps its own layout up to date even while the review is on
	// screen, so coming back from it never shows a stale month.
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		m.loadEntry()
		m.refreshGit()
	}
	// The git panel's own messages are answered whatever is on screen: a run
	// started before the review was opened still lands, and dropping its answer
	// would leave the panel reading for ever.
	switch msg := msg.(type) {
	case dayStandupLoadedMsg:
		return m.gitLoaded(msg)

	case spinner.TickMsg:
		if !m.gitLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.gitSpin, cmd = m.gitSpin.Update(msg)
		m.refreshGit()
		return m, cmd
	}
	if m.review != nil {
		return m.updateReview(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case editorFinishedMsg:
		m.err = msg.err
		m.status = ""
		if msg.err == nil {
			m.status = "Saved " + m.selected.Format(dateparse.Layout) + ".md"
		}
		m.reload()
		return m, mouseAgain(m.mouse)

	case tea.KeyMsg:
		return m.apply(msg.String())

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

// apply performs the move a key stands for. It is named rather than driven by the
// message so a click on a hint can reach exactly the same code.
func (m calendarModel) apply(key string) (tea.Model, tea.Cmd) {
	m.status = ""
	m.err = nil
	// Whatever the key does, the git panel follows the day it lands on.
	before := m.selected

	switch key {
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit

	case "e", "enter":
		return m, m.editSelected()

	case "r":
		m.openReview()

	case "s":
		return m, m.toggleGit()

	case "tab", "shift+tab":
		m.focus = (m.focus + 1) % calendarPaneCount

	// Moving through the calendar.
	case "left", "h":
		m.shiftDay(-1)
	case "right", "l":
		m.shiftDay(1)
	case "up":
		m.shiftDay(-7)
	case "down":
		m.shiftDay(7)
	case "n", "pgdown", ">":
		m.shiftMonth(1)
	case "p", "pgup", "<":
		m.shiftMonth(-1)
	case "t":
		m.selectDay(m.source.Today())

	// j and k are the arrows of the grid, or scroll the notes once they have the
	// focus.
	case "j":
		if m.gridFocused() {
			m.shiftDay(7)
		} else {
			m.entry.ScrollDown(1)
		}
	case "k":
		if m.gridFocused() {
			m.shiftDay(-7)
		} else {
			m.entry.ScrollUp(1)
		}

	// The grid has no use for these, so they scroll the notes whatever the focus.
	case "ctrl+d":
		m.entry.HalfPageDown()
	case "ctrl+u":
		m.entry.HalfPageUp()
	case "g", "home":
		m.entry.GotoTop()
	case "G", "end":
		m.entry.GotoBottom()

	// The git panel has its own pane, so it has its own two keys.
	case "J":
		m.git.ScrollDown(1)
	case "K":
		m.git.ScrollUp(1)
	}
	return m, m.followSelection(before)
}

// gridFocused reports whether j and k move on the grid. On a terminal too narrow
// for the grid, the notes are the only pane left and have the keyboard.
func (m calendarModel) gridFocused() bool {
	return m.focus == calendarGridPane && m.showCalendarPane()
}

// followSelection reloads what the selected day feeds when the day has moved.
func (m *calendarModel) followSelection(before time.Time) tea.Cmd {
	if m.selected.Equal(before) {
		return nil
	}
	return m.gitFollow()
}

// openReview switches to the sprint review of the period the selected day falls
// in. Nothing is lost: leaving the review comes back to this month and day.
func (m *calendarModel) openReview() {
	opts, err := m.source.ReviewPeriod(m.selected)
	if err != nil {
		m.err = err
		return
	}
	review := newReviewModel(m.theme, m.source, opts, m.width, m.height)
	review.nested = true
	review.mouse = m.mouse
	m.review = &review
}

// updateReview drives the review while it is on screen and takes the calendar
// back when it is left. Leaving it must not end the program, so the Quit command
// it returns is dropped.
func (m calendarModel) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.review.Update(msg)
	next, ok := updated.(reviewModel)
	if !ok {
		return m, cmd
	}
	if next.quit {
		m.review = nil
		m.reload()
		return m, nil
	}
	*m.review = next
	return m, cmd
}

// editSelected opens the selected day in the editor, in the foreground.
func (m calendarModel) editSelected() tea.Cmd {
	cmd, err := m.source.EditCommand(m.selected)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m *calendarModel) shiftDay(delta int) {
	m.selectDay(m.selected.AddDate(0, 0, delta))
}

// shiftMonth changes month, clamping the day to the new month's length.
func (m *calendarModel) shiftMonth(delta int) {
	next := m.month.Add(delta)
	loc := m.selected.Location()
	day := m.selected.Day()
	if maxDay := next.Days(loc); day > maxDay {
		day = maxDay
	}
	m.selectDay(time.Date(next.Year, next.Month, day, 0, 0, 0, 0, loc))
}

// --- mouse -------------------------------------------------------------------

const (
	// dayCellWidth is what one day cell costs, the four columns renderDayCell
	// writes.
	dayCellWidth = 4
	// calendarPaneY is the row the panes' top border sits on: the calendar view
	// renders them straight under its outer padding.
	calendarPaneY = 1
	// calendarGridRow is the pane row the first week lands on: the month title, a
	// blank line and the weekday headers come before it.
	calendarGridRow = 3
)

// handleMouse turns a mouse event into the moves the keyboard already makes: a
// click picks the day or presses the hint it lands on, a click elsewhere in a pane
// gives it the focus, and the wheel pages the month or scrolls the entry,
// depending on the pane it is over.
func (m calendarModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	before := m.selected

	if delta := wheel(msg); delta != 0 {
		switch {
		case m.calendarBand().contains(msg.X, msg.Y):
			m.shiftMonth(delta)
		case m.gitBand().contains(msg.X, msg.Y):
			scrollBy(&m.git, delta*wheelLines)
		case m.entryBand().contains(msg.X, msg.Y):
			scrollBy(&m.entry, delta*wheelLines)
		}
		return m, m.followSelection(before)
	}
	if !leftClick(msg) {
		return m, nil
	}

	// The hit tests read the pane as it stands on screen, so they run before
	// anything clears the status line the hints sit under.
	if day, ok := m.dayAt(msg.X, msg.Y); ok {
		m.status, m.err = "", nil
		m.focus = calendarGridPane
		m.selectDay(day)
		return m, m.followSelection(before)
	}
	// A hint is a key press, and the focus is what that key decides.
	if key, ok := m.hintAt(msg.X, msg.Y); ok {
		return m.apply(key)
	}
	switch {
	case m.calendarBand().contains(msg.X, msg.Y):
		m.focus = calendarGridPane
	case m.entryBand().contains(msg.X, msg.Y):
		m.focus = calendarEntryPane
	}
	return m, nil
}

// dayAt returns the day the cell under the given screen position holds. The
// padding cells before the 1st and after the last hold none.
func (m calendarModel) dayAt(x, y int) (time.Time, bool) {
	grid := m.gridRect()
	if !grid.contains(x, y) {
		return time.Time{}, false
	}
	day := m.calendar.Weeks[y-grid.y][(x-grid.x)/dayCellWidth]
	if day.Padding() {
		return time.Time{}, false
	}
	return day.Date, true
}

// hintAt returns the key named by the row of the NAVIGATE table under the given
// screen position.
func (m calendarModel) hintAt(x, y int) (string, bool) {
	hints := m.hintsRect()
	if !hints.contains(x, y) {
		return "", false
	}
	return hintKeyAt(m.hints()[y-hints.y][0], x-hints.x, true)
}

// entryPaneX is the column the centre pane's border sits on.
func (m calendarModel) entryPaneX() int {
	if m.showCalendarPane() {
		return firstPaneX + calendarPaneWidth + paneChrome + 1
	}
	return firstPaneX
}

// calendarBand, entryBand and gitBand are the column bands the panes occupy, over
// the whole height of the screen: what the wheel does depends on which pane the
// pointer is over, not on how far down it sits, and the panes are not the same
// height. A pane that is not on screen has no band.
func (m calendarModel) calendarBand() rect {
	if !m.showCalendarPane() {
		return rect{}
	}
	return rect{x: firstPaneX, y: 0, w: calendarPaneWidth + paneChrome, h: m.height}
}

func (m calendarModel) entryBand() rect {
	band := rect{x: m.entryPaneX(), y: 0, w: m.entry.Width + paneChrome, h: m.height}
	if m.gitOpen && !m.gitBeside() {
		// Stacked, the notes stop where the panel starts.
		band.y, band.h = calendarPaneY, m.entry.Height+paneRows
	}
	return band
}

// gitBand is where the panel sits: to the right of the notes, or under them.
func (m calendarModel) gitBand() rect {
	switch {
	case !m.gitOpen:
		return rect{}
	case m.gitBeside():
		return rect{
			x: m.entryPaneX() + m.entry.Width + paneChrome + 1,
			y: 0,
			w: m.git.Width + paneChrome,
			h: m.height,
		}
	}
	return rect{
		x: m.entryPaneX(),
		y: calendarPaneY + m.entry.Height + paneRows,
		w: m.git.Width + paneChrome,
		h: m.git.Height + paneRows,
	}
}

// gridRect is where the day cells sit, the weekday headers excluded.
func (m calendarModel) gridRect() rect {
	if !m.showCalendarPane() {
		return rect{}
	}
	return rect{
		x: firstPaneX + paneContentInset,
		y: calendarPaneY + paneBorder + calendarGridRow,
		w: len(journal.WeekdayHeaders) * dayCellWidth,
		h: len(m.calendar.Weeks),
	}
}

// hintsRect is where the NAVIGATE rows sit. renderKeys writes each of them as a
// leading space, the keys, then the label, which is why the keys start one column
// into the pane's content.
func (m calendarModel) hintsRect() rect {
	if !m.showCalendarPane() {
		return rect{}
	}
	return rect{
		x: firstPaneX + paneContentInset + 1,
		y: calendarPaneY + paneBorder + m.hintsRow(),
		w: calendarPaneWidth - 1,
		h: len(m.hints()),
	}
}

// hintsRow is the pane row the first hint lands on. It walks the lines
// renderCalendarPane writes, so the two must change together.
func (m calendarModel) hintsRow() int {
	// The grid, a blank line, the legend, another blank line.
	row := calendarGridRow + len(m.calendar.Weeks) + 1 + len(legendItems(m.theme)) + 1
	// Then whichever of the status and the failure is up, each with the blank line
	// under it.
	if m.status != "" {
		row += 2
	}
	if m.err != nil {
		row += 2
	}
	// And the NAVIGATE label itself.
	return row + 1
}

func (m calendarModel) View() string {
	if m.quit {
		return ""
	}
	if m.review != nil {
		return m.review.View()
	}

	panes := make([]string, 0, 5)
	if m.showCalendarPane() {
		panes = append(panes, m.pane(m.renderCalendarPane(), calendarPaneWidth, 0, m.gridFocused()), " ")
	}
	panes = append(panes, m.renderCentreColumn())
	if m.showInsight() {
		panes = append(panes, " ", m.pane(m.renderInsightPane(), insightPaneWidth, m.paneContentHeight(), false))
	}

	return lipgloss.NewStyle().Padding(1, 1).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, panes...))
}

// renderCentreColumn is the day itself: the notes, with the git panel beside them
// when the column is wide enough for two and under them when it is not. However
// they are arranged, the column comes out the same height as the panels either
// side of it.
func (m calendarModel) renderCentreColumn() string {
	entry := m.pane(m.renderEntryPane(), m.entry.Width, m.entry.Height+paneHeadings, !m.gridFocused())
	if !m.gitOpen {
		return entry
	}

	git := m.pane(m.renderGitPane(), m.git.Width, m.git.Height+paneHeadings, false)
	if m.gitBeside() {
		return lipgloss.JoinHorizontal(lipgloss.Top, entry, " ", git)
	}
	return lipgloss.JoinVertical(lipgloss.Left, entry, git)
}

// Every view lays its panels out on the same grid, and the mouse hit tests read
// it back.
const (
	// panePadding is the horizontal padding inside every panel, on each side.
	panePadding = 1
	// paneChrome is what a pane's border and padding cost in columns.
	paneChrome = 2 + 2*panePadding
	// firstPaneX is the column the leftmost pane's border sits on, one in from
	// the outer padding every view renders.
	firstPaneX = 1
	// paneContentInset is how far a pane's content sits from its own left edge:
	// its border, then its padding.
	paneContentInset = 1 + panePadding
	// paneBorder is what a pane's top border costs above its first content row.
	paneBorder = 1
	// paneHeadings is what a pane with a scrolling body costs above it: its heading
	// and the rule under it.
	paneHeadings = 2
	// paneRows is what such a pane takes on screen beyond its viewport: the two
	// lines above it and its two border rows.
	paneRows = paneHeadings + 2*paneBorder
)

// pane wraps content in a rounded border, in the accent when it has the focus.
//
// width is the content width: lipgloss counts padding inside Width, so it is
// added back here, and every caller can think in terms of usable columns. A
// height of zero leaves the pane as tall as what it holds.
func (m calendarModel) pane(content string, width, height int, focused bool) string {
	border := m.theme.Palette.Border
	if focused {
		border = m.theme.Palette.Accent
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, panePadding).
		Width(width + 2*panePadding)
	if height > 0 {
		style = style.Height(height)
	}
	return style.Render(content)
}

// --- left pane ---------------------------------------------------------------

func (m calendarModel) renderCalendarPane() string {
	t := m.theme
	lines := []string{
		centre(t.Title.Render(m.month.Title()), calendarPaneWidth),
		"",
		RenderCalendarGrid(t, m.calendar, m.selected),
		"",
		CalendarLegendLines(t),
		"",
	}

	if m.status != "" {
		lines = append(lines, t.Success.Render("✓ "+m.status), "")
	}
	if m.err != nil {
		lines = append(lines, t.Danger.Render("! "+truncate(m.err.Error(), calendarPaneWidth)), "")
	}
	lines = append(lines, m.renderKeys())
	return strings.Join(lines, "\n")
}

// keyHints is the navigation table shown inside the calendar pane, the way tock
// keeps its hints next to the grid. Its rows are clickable, which needs no line
// of its own: a hint is the key it names, however it is pressed.
var keyHints = [][2]string{
	{"← →", "day"},
	{"↑ ↓", "week"},
	{"n p", "month"},
	{"t", "today"},
	{"e", "edit entry"},
	{"s", "git activity"},
	{"r", "sprint review"},
	// Last, so a short terminal drops it first: a click on the notes does the same.
	{"tab", "pane"},
}

// quitHint sits at the bottom of the table whatever else fits above it.
var quitHint = [2]string{"q", "quit"}

// hints is the navigation table as it ends up on screen. The clicking is driven
// off the same list, so a row that is not up cannot be clicked either.
//
// Scrolling is not in it: j k on the focused pane, J K on the git panel and the
// wheel on whichever the pointer is over are the keys everyone already tries.
func (m calendarModel) hints() [][2]string {
	// Drop the least useful rows rather than let the table run past the pane, the
	// way the other views' footers give ground. Quitting keeps its place whatever
	// goes: it is the one key with nowhere else to be read.
	rows := append([][2]string{}, keyHints...)
	room := m.paneContentHeight() - m.hintsRow() - 1
	for len(rows) > 0 && len(rows) > room {
		rows = rows[:len(rows)-1]
	}
	return append(rows, quitHint)
}

func (m calendarModel) renderKeys() string {
	t := m.theme
	hints := m.hints()

	width := 0
	for _, hint := range hints {
		if n := lipgloss.Width(hint[0]); n > width {
			width = n
		}
	}

	lines := []string{t.Muted.Render("NAVIGATE")}
	for _, hint := range hints {
		pad := strings.Repeat(" ", width-lipgloss.Width(hint[0]))
		lines = append(lines, " "+t.HelpKey.Render(hint[0])+pad+"  "+t.Muted.Render(hint[1]))
	}
	return strings.Join(lines, "\n")
}

// --- centre pane -------------------------------------------------------------

func (m calendarModel) renderEntryPane() string {
	t := m.theme
	title := t.Title.Render(m.selected.Format("Monday, 2 January 2006"))

	meta := t.Muted.Render("no entry")
	if m.hasEntry {
		meta = t.Muted.Render(fmt.Sprintf("%s words", Thousands(m.entryWords)))
	}
	gap := m.entry.Width - lipgloss.Width(title) - lipgloss.Width(meta)
	if gap < 1 {
		gap = 1
	}

	header := title + strings.Repeat(" ", gap) + meta
	return header + "\n" + t.HRule(m.entry.Width) + "\n" + m.entry.View()
}

// --- git pane ----------------------------------------------------------------

// renderGitPane draws the git activity of the selected day.
func (m calendarModel) renderGitPane() string {
	t := m.theme
	width := m.git.Width

	meta := m.gitMeta()
	// The panel scrolls, so it says when there is more under the fold.
	if !m.git.AtBottom() {
		meta += " ↓"
	}
	header := spread(t.Title.Render("GIT"), t.Muted.Render(truncate(meta, width-4)), width)
	return header + "\n" + t.HRule(width) + "\n" + m.git.View()
}

// gitMeta counts the day's commits, or says why there are none to count.
func (m calendarModel) gitMeta() string {
	switch {
	case m.gitLoading:
		return standupReading
	case m.gitErr != nil:
		return standupTrouble(m.gitErr)
	}
	return plural(m.gitReport.Commits(), "commit", "commits")
}

// renderGitContent lays the day's commits out, or says what stands in their place.
func (m calendarModel) renderGitContent() string {
	t := m.theme
	switch {
	case m.gitLoading:
		return renderStandupReading(t, m.gitSpin)
	case m.gitErr != nil:
		return renderStandupProblem(t, m.gitErr, m.git.Width)
	case m.gitReport.Empty():
		return t.Muted.Render("No commit on this day.")
	}
	return strings.Join(renderStandupRepos(t, m.gitReport, m.git.Width), "\n")
}

// --- right pane --------------------------------------------------------------

func (m calendarModel) renderInsightPane() string {
	t := m.theme
	in := m.insight
	w := insightPaneWidth

	sections := [][]string{
		{
			t.Muted.Render("THIS MONTH"),
			field(t, "Entries", fmt.Sprint(in.Entries), w),
			field(t, "Words", Thousands(in.Words), w),
			field(t, "Days", fmt.Sprintf("%d/%d", in.DaysWritten, in.DaysInMonth), w),
		},
		{
			t.Muted.Render("STREAKS"),
			field(t, "Current", plural(in.CurrentStreak, "day", "days"), w),
			field(t, "Longest", plural(in.LongestStreak, "day", "days"), w),
		},
		append([]string{t.Muted.Render("THIS WEEK")}, m.renderWeekBars()...),
	}
	if len(in.Tags) > 0 {
		sections = append(sections, append([]string{t.Muted.Render("TOP TAGS")}, m.renderTagBars()...))
	}

	blocks := make([]string, 0, len(sections))
	for _, section := range sections {
		blocks = append(blocks, strings.Join(section, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

// renderWeekBars draws one bar per weekday, scaled to the busiest day.
func (m calendarModel) renderWeekBars() []string {
	t := m.theme
	most := m.insight.MaxWeekWords()

	lines := make([]string, 0, 7)
	for _, day := range m.insight.Week {
		label := day.Date.Format("Mon")[:2]
		style := t.Muted
		if day.Date.Equal(m.selected) {
			style = t.SelectedItem
		}

		bar := ""
		if day.Words > 0 {
			bar = lipgloss.NewStyle().Foreground(t.Palette.Accent).
				Render(strings.Repeat("█", barLength(day.Words, most, 12)))
		}
		count := ""
		if day.HasEntry {
			count = t.Muted.Render(" " + Thousands(day.Words))
		}
		lines = append(lines, style.Render(label)+" "+bar+count)
	}
	return lines
}

// renderTagBars draws the most used tags, scaled to the most frequent one.
func (m calendarModel) renderTagBars() []string {
	t := m.theme
	most := m.insight.MaxTagCount()

	lines := make([]string, 0, len(m.insight.Tags))
	for _, tag := range m.insight.Tags {
		label := truncate("#"+tag.Tag, 10)
		pad := strings.Repeat(" ", 10-lipgloss.Width(label))
		bar := lipgloss.NewStyle().Foreground(t.Palette.Info).
			Render(strings.Repeat("▪", barLength(tag.Count, most, 6)))
		lines = append(lines,
			t.Accent.Render(label)+pad+" "+bar+t.Muted.Render(" "+fmt.Sprint(tag.Count)))
	}
	return lines
}

// barLength scales value against most, always showing at least one cell for a
// non-zero value.
func barLength(value, most, maxCells int) int {
	if value <= 0 || most <= 0 {
		return 0
	}
	cells := value * maxCells / most
	if cells < 1 {
		cells = 1
	}
	return cells
}

// field renders a label/value row filling the given width.
func field(t *Theme, label, value string, width int) string {
	gap := width - lipgloss.Width(label) - lipgloss.Width(value)
	if gap < 1 {
		gap = 1
	}
	return t.Label.Render(label) + strings.Repeat(" ", gap) + t.Value.Render(value)
}

// centre pads text so it sits in the middle of width columns.
func centre(text string, width int) string {
	pad := (width - lipgloss.Width(text)) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + text
}

// truncate shortens text to width columns. A width of zero or less leaves room
// for nothing, which is what a terminal yet to report its size asks for.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

// --- shared grid rendering ---------------------------------------------------

// RenderCalendarGrid draws a month as a Monday-first grid. When selected is
// non-zero that day is highlighted.
//
// It is shared by the interactive view and the plain `journal calendar` output.
func RenderCalendarGrid(theme *Theme, cal journal.Calendar, selected time.Time) string {
	var b strings.Builder

	// Every cell is four columns wide so the grid stays aligned whether or not
	// a day carries the selection highlight.
	headers := make([]string, 0, 7)
	for _, name := range journal.WeekdayHeaders {
		headers = append(headers, theme.Muted.Render(" "+name+" "))
	}
	b.WriteString(strings.Join(headers, ""))
	b.WriteString("\n")

	for _, week := range cal.Weeks {
		cells := make([]string, 0, 7)
		for _, day := range week {
			cells = append(cells, renderDayCell(theme, day, selected))
		}
		b.WriteString(strings.Join(cells, ""))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderDayCell renders one four-column day cell: a leading space, the
// right-aligned day number, then a trailing space.
func renderDayCell(theme *Theme, day journal.Day, selected time.Time) string {
	if day.Padding() {
		return "    "
	}
	number := fmt.Sprintf("%2d", day.Date.Day())

	var style lipgloss.Style
	switch {
	case day.IsToday && day.HasEntry:
		style = lipgloss.NewStyle().Bold(true).Foreground(theme.Palette.Success)
	case day.IsToday:
		style = lipgloss.NewStyle().Bold(true).Foreground(theme.Palette.Warning)
	case day.HasEntry:
		style = lipgloss.NewStyle().Foreground(theme.Palette.Info)
	default:
		style = theme.Muted
	}

	if !selected.IsZero() && day.Date.Equal(selected) {
		return lipgloss.NewStyle().
			Foreground(theme.Palette.Inverse).
			Background(theme.Palette.Accent).
			Bold(true).
			Render(" " + number + " ")
	}
	return " " + style.Render(number) + " "
}

// CalendarLegend explains the day colours on one line, for piped output.
func CalendarLegend(theme *Theme) string {
	return strings.Join(legendItems(theme), theme.Muted.Render("   "))
}

// CalendarLegendLines stacks the legend, for the narrow calendar pane.
func CalendarLegendLines(theme *Theme) string {
	return strings.Join(legendItems(theme), "\n")
}

func legendItems(theme *Theme) []string {
	dot := func(color lipgloss.TerminalColor, label string) string {
		return lipgloss.NewStyle().Foreground(color).Render("●") + theme.Muted.Render(" "+label)
	}
	return []string{
		dot(theme.Palette.Success, "today + entry"),
		dot(theme.Palette.Info, "entry"),
		dot(theme.Palette.Warning, "today"),
	}
}
