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

// ReviewSource supplies the sprint review with its data and its editor, so the
// view touches neither the filesystem nor the editor itself.
type ReviewSource interface {
	// Review collects the tagged notes of a period.
	Review(opts journal.ReviewOptions) (journal.Review, error)
	// EditCommand prepares the editor for a day, creating the entry if needed.
	// The view runs it in the foreground and reloads afterwards.
	EditCommand(day time.Time) (*exec.Cmd, error)
}

const (
	// minReviewColumnWidth is the narrowest a mood column may get. Below three
	// of them the view shows one column at a time instead of squeezing.
	minReviewColumnWidth = 20
	// reviewPaneChrome is what a pane's border and padding cost in columns.
	reviewPaneChrome = 2 + 2*panePadding
	// reviewPageJump is how many notes ctrl+d / ctrl+u move over.
	reviewPageJump = 5
	// minReviewWidth and minReviewHeight are the smallest size a layout can be
	// built on. A terminal that has not reported its size yet says 0 by 0.
	minReviewWidth  = minReviewColumnWidth + reviewPaneChrome + 2
	minReviewHeight = 12
)

// reviewModel is the Bubble Tea model behind ShowReview.
type reviewModel struct {
	theme  *Theme
	source ReviewSource

	// initial is the period the view opened on, restored by `t`.
	initial journal.ReviewOptions
	review  journal.Review
	columns [3]reviewColumn
	focus   int

	width, height int

	// nested marks a review opened from the calendar, which quitting returns to.
	nested bool
	// status is a transient message shown in place of the key hints.
	status string
	err    error
	quit   bool
}

// reviewColumn is one mood's notes, laid out as scrollable cards.
type reviewColumn struct {
	mood     journal.Mood
	items    []journal.ReviewItem
	view     viewport.Model
	selected int
	// offsets and heights locate each card inside the rendered content, which is
	// what lets the selection be scrolled into view.
	offsets []int
	heights []int
}

// ShowReview runs the full-screen sprint review over the given period.
func ShowReview(theme *Theme, source ReviewSource, opts journal.ReviewOptions) error {
	m := newReviewModel(theme, source, opts, TerminalWidth(), TerminalHeight())
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newReviewModel(theme *Theme, source ReviewSource, opts journal.ReviewOptions, width, height int) reviewModel {
	m := reviewModel{theme: theme, source: source, initial: opts}
	m.resize(width, height)
	for i, mood := range journal.Moods {
		m.columns[i].mood = mood
		m.columns[i].view = viewport.New(minReviewColumnWidth, 10)
	}
	m.load(opts, true)
	return m
}

func (m reviewModel) Init() tea.Cmd { return nil }

// load fetches a period and lays it out. reset returns every column to its
// first card, which is what changing period should do; keeping the selection is
// what coming back from the editor should do.
func (m *reviewModel) load(opts journal.ReviewOptions, reset bool) {
	review, err := m.source.Review(opts)
	m.err = err
	m.review = review

	for i := range m.columns {
		m.columns[i].items = review.Columns[i].Items
		if reset {
			m.columns[i].selected = 0
			m.columns[i].view.GotoTop()
		}
		m.columns[i].clampSelection()
	}
	m.refresh(true)
}

// period returns the options that reproduce the period currently shown.
func (m reviewModel) period() journal.ReviewOptions {
	return journal.ReviewOptions{From: m.review.From, To: m.review.To}
}

// shiftPeriod moves the window by its own length, so paging back twice on a
// fortnightly review lands a month ago.
func (m *reviewModel) shiftPeriod(direction int) {
	days := m.review.Days * direction
	m.load(journal.ReviewOptions{
		From: m.review.From.AddDate(0, 0, days),
		To:   m.review.To.AddDate(0, 0, days),
	}, true)
}

// refresh re-renders every column at the current size. follow scrolls the
// focused column so its selected card is visible.
func (m *reviewModel) refresh(follow bool) {
	width, height := m.columnWidth(), m.viewHeight()
	for i := range m.columns {
		c := &m.columns[i]
		c.view.Width, c.view.Height = width, height
		c.view.SetContent(c.render(m.theme, i == m.focus))
	}
	if follow {
		m.columns[m.focus].scrollToSelection()
	}
}

// resize records the terminal size, keeping it large enough to lay something out.
func (m *reviewModel) resize(width, height int) {
	if width < minReviewWidth {
		width = minReviewWidth
	}
	if height < minReviewHeight {
		height = minReviewHeight
	}
	m.width, m.height = width, height
}

// showAllColumns reports whether the terminal fits the three columns side by
// side. It does not below that, and the view falls back to one column at a time.
func (m reviewModel) showAllColumns() bool {
	return m.width >= 3*(minReviewColumnWidth+reviewPaneChrome)+2+2
}

// columnWidth is the usable width inside one mood pane.
func (m reviewModel) columnWidth() int {
	// Outer margin, then either three panes separated by a space, or one pane.
	width := m.width - 2
	if m.showAllColumns() {
		width = (width - 2 - 3*reviewPaneChrome) / 3
	} else {
		width -= reviewPaneChrome
	}
	if width < minReviewColumnWidth {
		width = minReviewColumnWidth
	}
	return width
}

// paneHeight is the height of a mood pane, border included.
func (m reviewModel) paneHeight() int {
	// Two lines of header, a blank line, the panes, a blank line and the footer,
	// inside the outer vertical padding.
	height := m.height - 7
	if height < 8 {
		height = 8
	}
	return height
}

// viewHeight is what is left for the cards once the pane's border and its own
// heading are taken out.
func (m reviewModel) viewHeight() int {
	height := m.paneHeight() - 4
	if height < 3 {
		height = 3
	}
	return height
}

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		m.refresh(true)
		return m, nil

	case editorFinishedMsg:
		day := m.editedDay()
		// The note may have been rewritten, so re-read the period. load reports
		// its own failure, which is why the editor's outcome comes after it.
		m.load(m.period(), false)
		m.status = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if m.err == nil {
			m.status = "Saved " + day.Format(dateparse.Layout) + ".md"
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m reviewModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	m.err = nil

	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit

	case "e", "enter":
		return m, m.editSelected()

	// Moving between the moods.
	case "left", "h", "shift+tab":
		m.shiftFocus(-1)
	case "right", "l", "tab":
		m.shiftFocus(1)

	// Moving through a mood's notes.
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	case "ctrl+u":
		m.moveSelection(-reviewPageJump)
	case "ctrl+d":
		m.moveSelection(reviewPageJump)
	case "g", "home":
		m.moveSelection(-len(m.columns[m.focus].items))
	case "G", "end":
		m.moveSelection(len(m.columns[m.focus].items))

	// Moving through the periods.
	case "p", "pgup", "<":
		m.shiftPeriod(-1)
	case "n", "pgdown", ">":
		m.shiftPeriod(1)
	case "t":
		m.load(m.initial, true)
	}
	return m, nil
}

func (m *reviewModel) shiftFocus(delta int) {
	m.focus = (m.focus + delta + len(m.columns)) % len(m.columns)
	m.refresh(true)
}

func (m *reviewModel) moveSelection(delta int) {
	c := &m.columns[m.focus]
	if len(c.items) == 0 {
		return
	}
	c.selected += delta
	c.clampSelection()
	m.refresh(true)
}

// editedDay is the day the editor was asked for, which is the selected note's.
func (m reviewModel) editedDay() time.Time {
	if item, ok := m.columns[m.focus].current(); ok {
		return item.Date
	}
	return m.review.To
}

// editSelected opens the entry the selected note lives in.
func (m reviewModel) editSelected() tea.Cmd {
	item, ok := m.columns[m.focus].current()
	if !ok {
		return nil
	}
	cmd, err := m.source.EditCommand(item.Date)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// --- column ------------------------------------------------------------------

func (c reviewColumn) current() (journal.ReviewItem, bool) {
	if c.selected < 0 || c.selected >= len(c.items) {
		return journal.ReviewItem{}, false
	}
	return c.items[c.selected], true
}

func (c *reviewColumn) clampSelection() {
	if c.selected >= len(c.items) {
		c.selected = len(c.items) - 1
	}
	if c.selected < 0 {
		c.selected = 0
	}
}

// render lays the cards out, recording where each one landed so the selection
// can be followed, and returns the column's content.
func (c *reviewColumn) render(t *Theme, focused bool) string {
	if len(c.items) == 0 {
		c.offsets, c.heights = nil, nil
		return t.Muted.Render("No #"+c.mood.Tag()+" note") + "\n" + t.Muted.Render("in this period.")
	}

	c.offsets = make([]int, len(c.items))
	c.heights = make([]int, len(c.items))

	var lines []string
	for i, item := range c.items {
		if i > 0 {
			lines = append(lines, "")
		}
		card := renderReviewCard(t, item, c.mood, c.view.Width, focused && i == c.selected)
		c.offsets[i], c.heights[i] = len(lines), len(card)
		lines = append(lines, card...)
	}
	return strings.Join(lines, "\n")
}

// scrollToSelection brings the selected card into view, moving as little as
// possible so the surrounding notes stay where the eye left them.
func (c *reviewColumn) scrollToSelection() {
	if c.selected >= len(c.offsets) {
		return
	}
	start := c.offsets[c.selected]
	end := start + c.heights[c.selected]

	switch {
	case start < c.view.YOffset:
		c.view.SetYOffset(start)
	case end > c.view.YOffset+c.view.Height:
		c.view.SetYOffset(end - c.view.Height)
	}
}

// --- rendering ---------------------------------------------------------------

func (m reviewModel) View() string {
	if m.quit {
		return ""
	}

	panes := make([]string, 0, 5)
	if m.showAllColumns() {
		for i := range m.columns {
			if i > 0 {
				panes = append(panes, " ")
			}
			panes = append(panes, m.renderColumnPane(i))
		}
	} else {
		panes = append(panes, m.renderColumnPane(m.focus))
	}

	body := strings.Join([]string{
		m.renderTitle(),
		m.renderSubtitle(),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, panes...),
		"",
		m.renderFooter(),
	}, "\n")
	return lipgloss.NewStyle().Padding(1, 1).Render(body)
}

// renderTitle gives the label up before the period, and the stats before either.
func (m reviewModel) renderTitle() string {
	t := m.theme
	width := m.width - 2

	period := t.Value.Render(truncate(m.review.Period(), width))
	left := t.Title.Render("SPRINT REVIEW") + "  " + period
	if lipgloss.Width(left) > width {
		left = period
	}

	right := t.Muted.Render(reviewStats(m.review))
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return left
	}
	return spread(left, right, width)
}

// renderSubtitle carries the hint that explains an empty review, and keeps the
// layout the same height when there is nothing to explain.
func (m reviewModel) renderSubtitle() string {
	if m.review.Total() > 0 {
		return ""
	}
	t := m.theme
	width := m.width - 2
	if m.review.Entries == 0 {
		return t.Muted.Render(truncate("No entry in this period.", width))
	}

	long := t.Muted.Render("Nothing tagged yet. Write ") +
		t.Success.Render("[#good]") + t.Muted.Render(", ") +
		t.Warning.Render("[#bad]") + t.Muted.Render(" or ") +
		t.Danger.Render("[#mad]") + t.Muted.Render(" in a note and it lands here.")
	if lipgloss.Width(long) <= width {
		return long
	}
	return t.Muted.Render(truncate("Nothing tagged in this period.", width))
}

func reviewStats(review journal.Review) string {
	return fmt.Sprintf("%s  ·  %s  ·  %s",
		plural(review.Days, "day", "days"),
		plural(review.Entries, "entry", "entries"),
		plural(review.Total(), "note", "notes"))
}

// renderFooter shows the key hints, or the outcome of the last edit.
func (m reviewModel) renderFooter() string {
	t := m.theme
	switch {
	case m.err != nil:
		return t.Danger.Render("! " + truncate(m.err.Error(), m.width-4))
	case m.status != "":
		return t.Success.Render("✓ " + m.status)
	}

	leave := "quit"
	if m.nested {
		leave = "back to the calendar"
	}
	hints := [][2]string{
		{"← →", "mood"},
		{"↑ ↓", "note"},
		{"e", "edit"},
		{"p n", "period"},
		{"t", "now"},
		{"q", leave},
	}
	// Drop the least useful hints rather than let the footer wrap.
	for len(hints) > 1 && lipgloss.Width(t.Keys(hints...)) > m.width-2 {
		hints = hints[:len(hints)-1]
	}
	return t.Keys(hints...)
}

// renderColumnPane draws one mood: its heading, its count and its cards.
func (m reviewModel) renderColumnPane(i int) string {
	t := m.theme
	c := m.columns[i]
	focused := i == m.focus
	width := m.columnWidth()

	title := moodStyle(t, c.mood).Bold(focused).Render(moodGlyph(c.mood) + " " + c.mood.Title())
	count := t.Muted.Render(fmt.Sprint(len(c.items)))
	if focused && len(c.items) > 0 {
		count = t.Value.Render(fmt.Sprintf("%d/%d", c.selected+1, len(c.items)))
	}

	content := spread(title, count, width) + "\n" + t.HRule(width) + "\n" + c.view.View()

	border := t.Palette.Border
	if focused {
		border = moodColor(t, c.mood)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, panePadding).
		Width(width + 2*panePadding).
		Height(m.paneHeight() - 2).
		Render(content)
}

// renderReviewCard renders one note as a dated header and its text, spined by a
// bar that thickens into the mood's colour when the note is selected.
func renderReviewCard(t *Theme, item journal.ReviewItem, mood journal.Mood, width int, selected bool) []string {
	bar, barStyle := "│", t.Rule
	label := t.Muted
	if selected {
		bar, barStyle = "▌", moodStyle(t, mood)
		label = moodStyle(t, mood).Bold(true)
	}

	textWidth := width - 2
	if textWidth < 8 {
		textWidth = 8
	}

	head := item.Date.Format("Mon 2 Jan")
	if item.Time != "" {
		head += "  " + item.Time
	}

	spine := barStyle.Render(bar) + " "
	lines := []string{spine + label.Render(truncate(head, textWidth))}

	body := lipgloss.NewStyle().Width(textWidth).Render(RenderMarkdown(t, item.Text, textWidth))
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, spine+line)
	}
	return lines
}

func moodColor(t *Theme, mood journal.Mood) lipgloss.TerminalColor {
	switch mood {
	case journal.Good:
		return t.Palette.Success
	case journal.Bad:
		return t.Palette.Warning
	case journal.Mad:
		return t.Palette.Danger
	}
	return t.Palette.Muted
}

func moodStyle(t *Theme, mood journal.Mood) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(moodColor(t, mood))
}

// moodGlyph marks a mood for the terminals and the readers that do not tell the
// three colours apart.
func moodGlyph(mood journal.Mood) string {
	switch mood {
	case journal.Good:
		return "✓"
	case journal.Bad:
		return "▲"
	case journal.Mad:
		return "✖"
	}
	return "·"
}

// spread pushes left and right apart to fill width, keeping at least one space
// between them.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// --- static output -----------------------------------------------------------

// RenderReviewStatic renders a review without the interactive chrome, for piped
// output. It keeps the three columns side by side while they fit, and stacks
// them when they do not.
func RenderReviewStatic(theme *Theme, review journal.Review, width int) string {
	if width < minReviewColumnWidth+4 {
		width = minReviewColumnWidth + 4
	}

	var b strings.Builder
	b.WriteString("\n  " + theme.Title.Render("Sprint review") + "  " + theme.Value.Render(review.Period()) + "\n")
	b.WriteString("  " + theme.Muted.Render(reviewStats(review)) + "\n\n")

	columnWidth := (width - 2 - 4) / 3
	if columnWidth >= minReviewColumnWidth {
		blocks := make([]string, 0, 5)
		for i := range review.Columns {
			if i > 0 {
				blocks = append(blocks, "  ")
			}
			blocks = append(blocks, staticColumn(theme, review.Columns[i], columnWidth))
		}
		b.WriteString(Indent(lipgloss.JoinHorizontal(lipgloss.Top, blocks...)) + "\n\n")
		return b.String()
	}

	for _, column := range review.Columns {
		b.WriteString(Indent(staticColumn(theme, column, width-2)) + "\n\n")
	}
	return b.String()
}

// staticColumn renders one mood as a fixed-width block of cards.
func staticColumn(theme *Theme, column journal.ReviewColumn, width int) string {
	head := moodStyle(theme, column.Mood).Bold(true).
		Render(moodGlyph(column.Mood) + " " + column.Mood.Title())
	lines := []string{
		spread(head, theme.Muted.Render(fmt.Sprint(len(column.Items))), width),
		theme.HRule(width),
	}

	if len(column.Items) == 0 {
		lines = append(lines, theme.Muted.Render("No #"+column.Mood.Tag()+" note"))
	}
	for i, item := range column.Items {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderReviewCard(theme, item, column.Mood, width, false)...)
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}
