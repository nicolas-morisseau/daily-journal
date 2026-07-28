package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
)

// CalendarSource supplies the calendar view with everything it shows, so the
// view itself touches neither the filesystem nor the editor.
type CalendarSource interface {
	// Calendar lays out one month.
	Calendar(m journal.Month) journal.Calendar
	// Today is the current date at midnight.
	Today() time.Time
	// EntryContent returns a day's markdown and whether the entry exists.
	EntryContent(day time.Time) (string, bool)
	// Insight summarises the journal around a day, for the side panel.
	Insight(day time.Time, opts journal.InsightOptions) journal.Insight
	// EditCommand prepares the editor for a day, creating the entry if needed.
	// The view runs it in the foreground and reloads afterwards.
	EditCommand(day time.Time) (*exec.Cmd, error)
}

// Panel widths, in content columns. The centre pane takes whatever is left.
const (
	calendarPaneWidth = 28
	insightPaneWidth  = 24
	// minEntryPaneWidth is the narrowest the markdown pane may get before the
	// side panels start being dropped.
	minEntryPaneWidth = 32
)

// editorFinishedMsg is delivered once the foreground editor exits.
type editorFinishedMsg struct{ err error }

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

	width, height int

	// status is a transient message shown in place of the hints.
	status string
	// err holds the last editor failure, if any.
	err  error
	quit bool
}

// ShowCalendar runs the full-screen calendar. It returns when the user quits;
// entries are edited in place rather than handed back to the caller.
func ShowCalendar(theme *Theme, source CalendarSource, month journal.Month) error {
	m := calendarModel{
		theme:  theme,
		source: source,
		entry:  viewport.New(40, 10),
		width:  TerminalWidth(),
		height: TerminalHeight(),
	}
	m.setMonth(month)

	// Land on today when it belongs to the requested month, otherwise the 1st.
	today := source.Today()
	if journal.MonthOf(today) == month {
		m.selectDay(today)
	} else {
		m.selectDay(time.Date(month.Year, month.Month, 1, 0, 0, 0, 0, today.Location()))
	}

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
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

// showInsight reports whether the terminal is wide enough for the side panel.
func (m calendarModel) showInsight() bool {
	return m.width >= calendarPaneWidth+insightPaneWidth+minEntryPaneWidth+14
}

// showCalendarPane reports whether the terminal is wide enough for the grid.
func (m calendarModel) showCalendarPane() bool {
	return m.width >= calendarPaneWidth+minEntryPaneWidth+10
}

// layout recomputes the centre pane from the terminal size.
func (m *calendarModel) layout() {
	// Each panel costs 2 columns of border plus 2 of padding, and the panels are
	// separated by a single space.
	const chrome = 4

	width := m.width - 2 // outer margin
	if m.showCalendarPane() {
		width -= calendarPaneWidth + chrome + 1
	}
	if m.showInsight() {
		width -= insightPaneWidth + chrome + 1
	}
	width -= chrome
	if width < minEntryPaneWidth {
		width = minEntryPaneWidth
	}

	m.entry.Width = width
	m.entry.Height = m.paneContentHeight() - 2 // the pane header and its rule
	if m.entry.Height < 3 {
		m.entry.Height = 3
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.loadEntry()
		return m, nil

	case editorFinishedMsg:
		m.err = msg.err
		m.status = ""
		if msg.err == nil {
			m.status = "Saved " + m.selected.Format(dateparse.Layout) + ".md"
		}
		m.reload()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m calendarModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	m.err = nil

	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit

	case "e", "enter":
		return m, m.editSelected()

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

	// Scrolling the entry, as tock scrolls its detail pane.
	case "j":
		m.entry.ScrollDown(1)
	case "k":
		m.entry.ScrollUp(1)
	case "ctrl+d":
		m.entry.HalfPageDown()
	case "ctrl+u":
		m.entry.HalfPageUp()
	case "g", "home":
		m.entry.GotoTop()
	case "G", "end":
		m.entry.GotoBottom()
	}
	return m, nil
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

func (m calendarModel) View() string {
	if m.quit {
		return ""
	}

	panes := make([]string, 0, 5)
	if m.showCalendarPane() {
		panes = append(panes, m.pane(m.renderCalendarPane(), calendarPaneWidth, false), " ")
	}
	panes = append(panes, m.pane(m.renderEntryPane(), m.entry.Width, true))
	if m.showInsight() {
		panes = append(panes, " ", m.pane(m.renderInsightPane(), insightPaneWidth, true))
	}

	return lipgloss.NewStyle().Padding(1, 1).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, panes...))
}

// panePadding is the horizontal padding inside every panel, on each side.
const panePadding = 1

// pane wraps content in a rounded border.
//
// width is the content width: lipgloss counts padding inside Width, so it is
// added back here, and every caller can think in terms of usable columns.
func (m calendarModel) pane(content string, width int, fullHeight bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Palette.Border).
		Padding(0, panePadding).
		Width(width + 2*panePadding)
	if fullHeight {
		style = style.Height(m.paneContentHeight())
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
// keeps its hints next to the grid.
var keyHints = [][2]string{
	{"← →", "day"},
	{"↑ ↓", "week"},
	{"n p", "month"},
	{"t", "today"},
	{"j k", "scroll entry"},
	{"e", "edit entry"},
	{"q", "quit"},
}

func (m calendarModel) renderKeys() string {
	t := m.theme
	width := 0
	for _, hint := range keyHints {
		if n := lipgloss.Width(hint[0]); n > width {
			width = n
		}
	}

	lines := []string{t.Muted.Render("NAVIGATE")}
	for _, hint := range keyHints {
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

// truncate shortens text to width columns.
func truncate(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width <= 1 {
		return string(runes[:width])
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
