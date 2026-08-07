package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
	"daily-journal/internal/standup"
)

// DailySource supplies the stand-up view with everything it shows and everything
// it writes, so the view touches neither the filesystem, git nor the editor.
type DailySource interface {
	// Today is the current date at midnight.
	Today() time.Time
	// Daily collects the journal notes of the window.
	Daily(opts journal.DailyOptions) journal.Daily
	// Standup reads the git activity of the last days. Its error is shown in the
	// git pane rather than ending the view: the journal half is still worth
	// having when the helper is missing or unconfigured.
	Standup(days int) (standup.Report, error)
	// Capture appends a timestamped line to today's entry.
	Capture(text string) error
	// EditCommand prepares the editor for a day, creating the entry if needed.
	EditCommand(day time.Time) (*exec.Cmd, error)
}

const (
	// minDailyColumnWidth is the narrowest a pane may get before the view shows
	// one of the two at a time instead of squeezing both.
	minDailyColumnWidth = 34
	// dailyPageJump is how many lines ctrl+d / ctrl+u scroll.
	dailyPageJump = 10
	// captureHint sits in the empty input, so what the box is for is never a
	// guess.
	captureHint = "Add a line to today's entry…"
)

// Panes of the stand-up view, in focus order.
const (
	dailyNotesPane = iota
	dailyGitPane
	dailyPaneCount
)

// standupLoadedMsg carries the outcome of one git-standup run. The helper walks
// every repository it is given, which takes seconds, so it runs as a command and
// the view stays responsive while it does.
type standupLoadedMsg struct {
	report standup.Report
	err    error
}

// dailyModel is the Bubble Tea model behind ShowDaily.
type dailyModel struct {
	theme  *Theme
	source DailySource
	// days is how far back both halves reach.
	days int

	daily journal.Daily
	notes viewport.Model

	report standup.Report
	// standupErr is why the git pane has nothing to show, if anything.
	standupErr error
	// loading is true while git-standup runs.
	loading bool
	spin    spinner.Model
	git     viewport.Model

	// input captures a note without leaving the view; typing tells whether it
	// has the keyboard.
	input  textinput.Model
	typing bool
	focus  int

	width, height int

	// status is a transient message shown in place of the key hints.
	status string
	// err holds the last capture or editor failure.
	err  error
	quit bool
}

// ShowDaily runs the full-screen stand-up view. It returns when the user quits;
// notes are captured and entries edited in place rather than handed back.
func ShowDaily(theme *Theme, source DailySource, days int) error {
	m := newDailyModel(theme, source, days, TerminalWidth(), TerminalHeight())
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func newDailyModel(theme *Theme, source DailySource, days, width, height int) dailyModel {
	input := textinput.New()
	input.Placeholder = captureHint
	input.Prompt = "› "
	input.PromptStyle = theme.Accent
	input.PlaceholderStyle = theme.Muted

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	spin.Style = theme.Accent

	m := dailyModel{
		theme:   theme,
		source:  source,
		days:    days,
		notes:   viewport.New(minDailyColumnWidth, 10),
		git:     viewport.New(minDailyColumnWidth, 10),
		input:   input,
		spin:    spin,
		loading: true,
		width:   width,
		height:  height,
	}
	m.reload()
	return m
}

// Init starts the spinner and the git-standup run behind it.
func (m dailyModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.loadStandup())
}

// loadStandup runs the helper off the update loop.
func (m dailyModel) loadStandup() tea.Cmd {
	source, days := m.source, m.days
	return func() tea.Msg {
		report, err := source.Standup(days)
		return standupLoadedMsg{report: report, err: err}
	}
}

// reload re-reads the journal side, which is cheap enough to do on every change.
func (m *dailyModel) reload() {
	m.daily = m.source.Daily(journal.DailyOptions{Days: m.days})
	m.layout()
	m.renderPanes()
}

// --- layout ------------------------------------------------------------------

// showBothPanes reports whether the terminal fits the journal and the git pane
// side by side.
func (m dailyModel) showBothPanes() bool {
	return m.width >= 2*(minDailyColumnWidth+paneChrome)+1+2
}

// columnWidth is the usable width inside one pane.
func (m dailyModel) columnWidth() int {
	width := m.width - 2 // outer margin
	if m.showBothPanes() {
		width = (width - 1 - 2*paneChrome) / 2
	} else {
		width -= paneChrome
	}
	return max(width, minDailyColumnWidth)
}

// paneHeight is the height of a pane, border included.
func (m dailyModel) paneHeight() int {
	// Two lines of header, a blank line, the panes, a blank line, the three lines
	// of the capture box, a blank line and the footer, inside the vertical
	// padding.
	return max(m.height-11, 8)
}

// viewHeight is what is left inside a pane once its border and heading are out.
func (m dailyModel) viewHeight() int {
	return max(m.paneHeight()-4, 3)
}

func (m *dailyModel) layout() {
	width, height := m.columnWidth(), m.viewHeight()
	m.notes.Width, m.notes.Height = width, height
	m.git.Width, m.git.Height = width, height
	m.input.Width = max(m.width-2-paneChrome-lipgloss.Width(m.input.Prompt)-1, 12)
}

// renderPanes re-renders both panes at the current size, keeping the scroll
// where the reader left it.
func (m *dailyModel) renderPanes() {
	m.notes.SetContent(m.renderNotes())
	m.git.SetContent(m.renderGit())
}

// --- update ------------------------------------------------------------------

func (m dailyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.renderPanes()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		m.renderPanes()
		return m, cmd

	case standupLoadedMsg:
		m.loading = false
		m.report, m.standupErr = msg.report, msg.err
		m.renderPanes()
		return m, nil

	case editorFinishedMsg:
		m.status, m.err = "", msg.err
		if msg.err == nil {
			m.status = "Saved " + m.source.Today().Format(dateparse.Layout) + ".md"
		}
		m.reload()
		return m, mouseAgain

	case tea.KeyMsg:
		if m.typing {
			return m.handleTypingKey(msg)
		}
		return m.apply(msg.String())

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

// handleTypingKey drives the capture box. Everything that is not one of its own
// keys is text, which is why quitting takes ctrl+c here.
func (m dailyModel) handleTypingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model, cmd, ok := m.applyTyping(msg.String()); ok {
		return model, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// applyTyping performs the capture box's own keys and reports whether the key was
// one of them. It is named rather than driven by the message so a click on a
// footer hint can reach the same code; anything it turns down is text for the box.
func (m dailyModel) applyTyping(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "ctrl+c":
		m.quit = true
		return m, tea.Quit, true

	case "esc", "tab":
		m.stopTyping()
		return m, nil, true

	case "enter":
		m.capture()
		return m, nil, true
	}
	return m, nil, false
}

// apply performs the move a key stands for, outside the capture box.
func (m dailyModel) apply(key string) (tea.Model, tea.Cmd) {
	m.status = ""
	m.err = nil

	switch key {
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit

	case "i", "a":
		m.startTyping()

	case "e", "enter":
		return m, m.editToday()

	case "r":
		if m.loading {
			return m, nil
		}
		m.loading = true
		m.reload()
		return m, tea.Batch(m.spin.Tick, m.loadStandup())

	// Moving between the two panes.
	case "tab", "shift+tab", "left", "right", "h", "l":
		m.focus = (m.focus + 1) % dailyPaneCount

	// Scrolling the focused pane.
	case "up", "k":
		m.focused().ScrollUp(1)
	case "down", "j":
		m.focused().ScrollDown(1)
	case "ctrl+u":
		m.focused().ScrollUp(dailyPageJump)
	case "ctrl+d":
		m.focused().ScrollDown(dailyPageJump)
	case "g", "home":
		m.focused().GotoTop()
	case "G", "end":
		m.focused().GotoBottom()
	}
	return m, nil
}

// focused is the viewport the keyboard scrolls.
func (m *dailyModel) focused() *viewport.Model { return m.pane(m.focus) }

func (m *dailyModel) startTyping() {
	m.typing = true
	m.status, m.err = "", nil
	m.input.Focus()
}

func (m *dailyModel) stopTyping() {
	m.typing = false
	m.input.Blur()
}

// capture appends what was typed to today's entry and clears the box, leaving it
// ready for the next line: a stand-up is written in several.
func (m *dailyModel) capture() {
	text := strings.TrimSpace(m.input.Value())
	m.status, m.err = "", nil
	if text == "" {
		return
	}
	if err := m.source.Capture(text); err != nil {
		m.err = err
		return
	}
	m.input.SetValue("")
	m.status = "Added to " + m.source.Today().Format(dateparse.Layout) + ".md"
	m.reload()
	// The new note is the last one of today, which is at the top of the pane.
	m.notes.GotoTop()
}

// --- mouse -------------------------------------------------------------------

// dailyPanesY is the row the panes' top border sits on: the title, the subtitle
// and a blank line come above them.
const dailyPanesY = 4

// handleMouse turns a mouse event into the moves the keyboard already makes: a
// click takes the focus to the pane it lands on, or the keyboard to the capture
// box, the wheel scrolls the pane it is over, and the footer hints answer to a
// click as well.
func (m dailyModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	pane, inPane := m.paneAt(msg.X, msg.Y)

	if delta := wheel(msg); delta != 0 {
		if inPane {
			scrollBy(m.pane(pane), delta*wheelLines)
		}
		return m, nil
	}
	if !leftClick(msg) {
		return m, nil
	}

	if key, ok := m.footerHintAt(msg.X, msg.Y); ok {
		if m.typing {
			model, cmd, _ := m.applyTyping(key)
			return model, cmd
		}
		return m.apply(key)
	}

	// Clicking the box is how the mouse asks for the keyboard, and clicking away
	// from it is how it gives it back.
	if m.captureRect().contains(msg.X, msg.Y) {
		m.startTyping()
		return m, nil
	}
	if m.typing {
		m.stopTyping()
	}
	if inPane {
		m.status, m.err = "", nil
		m.focus = pane
	}
	return m, nil
}

// paneAt returns the pane the given screen position falls in.
func (m dailyModel) paneAt(x, y int) (int, bool) {
	for i := range dailyPaneCount {
		if m.paneRect(i).contains(x, y) {
			return i, true
		}
	}
	return 0, false
}

// paneRect is where a pane sits, its border included. Only the focused pane is on
// screen when the terminal is too narrow for both, and the other is nowhere.
func (m dailyModel) paneRect(i int) rect {
	width := m.columnWidth() + paneChrome
	x := firstPaneX
	switch {
	case m.showBothPanes():
		x += i * (width + 1)
	case i != m.focus:
		return rect{}
	}
	return rect{x: x, y: dailyPanesY, w: width, h: m.paneHeight()}
}

// captureRect is where the capture box sits, its border included.
func (m dailyModel) captureRect() rect {
	return rect{
		x: firstPaneX,
		y: dailyPanesY + m.paneHeight() + 1, // the blank line under the panes
		w: max(m.width-2-paneChrome, minDailyColumnWidth) + paneChrome,
		h: 3, // its border, the input, its border
	}
}

// footerHintAt returns the key named by the footer hint under the given screen
// position. A footer reporting an outcome carries no hints to click.
func (m dailyModel) footerHintAt(x, y int) (string, bool) {
	if m.err != nil || m.status != "" {
		return "", false
	}
	capture := m.captureRect()
	if y != capture.y+capture.h {
		return "", false
	}
	return footerKeyAt(m.footerHints(), x-firstPaneX)
}

// pane is the viewport of a pane, by index.
func (m *dailyModel) pane(index int) *viewport.Model {
	if index == dailyGitPane {
		return &m.git
	}
	return &m.notes
}

// editToday opens today's entry in the editor, in the foreground.
func (m dailyModel) editToday() tea.Cmd {
	cmd, err := m.source.EditCommand(m.source.Today())
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// --- rendering ---------------------------------------------------------------

func (m dailyModel) View() string {
	if m.quit {
		return ""
	}

	panes := make([]string, 0, 3)
	if m.showBothPanes() {
		panes = append(panes,
			m.renderPane(dailyNotesPane), " ", m.renderPane(dailyGitPane))
	} else {
		panes = append(panes, m.renderPane(m.focus))
	}

	body := strings.Join([]string{
		m.renderTitle(),
		m.renderSubtitle(),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, panes...),
		"",
		m.renderCaptureBox(),
		m.renderFooter(),
	}, "\n")
	return lipgloss.NewStyle().Padding(1, 1).Render(body)
}

// renderTitle names the view and the day, with the totals on the right.
func (m dailyModel) renderTitle() string {
	t := m.theme
	width := m.width - 2

	day := t.Value.Render(m.source.Today().Format("Monday 2 January 2006"))
	left := t.Title.Render("DAILY") + "  " + day
	if lipgloss.Width(left) > width {
		left = day
	}

	right := t.Muted.Render(m.stats())
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return left
	}
	return spread(left, right, width)
}

// stats sums up both halves of the window in one line.
func (m dailyModel) stats() string {
	parts := []string{
		plural(m.daily.Notes(), "note", "notes"),
		plural(m.daily.Words(), "word", "words"),
	}
	switch {
	case m.loading:
		parts = append(parts, "reading git…")
	case m.standupErr == nil:
		parts = append(parts, plural(m.report.Commits(), "commit", "commits"))
	}
	return strings.Join(parts, "  ·  ")
}

// renderSubtitle says which window is on screen.
func (m dailyModel) renderSubtitle() string {
	label := "Since yesterday"
	if m.days != journal.DefaultDailyDays {
		label = fmt.Sprintf("Last %s", plural(m.days+1, "day", "days"))
	}
	from := m.daily.From.Format("Mon 2 Jan")
	return m.theme.Muted.Render(truncate(label+"  ·  "+from+" → "+m.daily.To.Format("Mon 2 Jan"), m.width-2))
}

// renderPane draws one pane with its border, the focused one carrying the accent.
func (m dailyModel) renderPane(index int) string {
	t := m.theme
	width := m.columnWidth()
	focused := index == m.focus

	var title, count, content string
	switch index {
	case dailyGitPane:
		title = "GIT"
		count = m.gitCount()
		content = m.git.View()
	default:
		title = "JOURNAL"
		count = plural(m.daily.Notes(), "note", "notes")
		content = m.notes.View()
	}

	heading := spread(
		lipgloss.NewStyle().Bold(focused).Foreground(t.Palette.Accent).Render(title),
		t.Muted.Render(truncate(count, width/2)),
		width)

	border := t.Palette.Border
	if focused {
		border = t.Palette.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, panePadding).
		Width(width + 2*panePadding).
		Height(m.paneHeight() - 2).
		Render(heading + "\n" + t.HRule(width) + "\n" + content)
}

// gitCount is the git pane's heading count, or why there is none.
func (m dailyModel) gitCount() string {
	switch {
	case m.loading:
		return standupReading
	case m.standupErr != nil:
		return standupTrouble(m.standupErr)
	}
	return fmt.Sprintf("%s in %s",
		plural(m.report.Commits(), "commit", "commits"),
		plural(len(m.report.Repos), "repo", "repos"))
}

// standupReading is what a git pane's heading says while the helper runs.
const standupReading = "reading…"

// standupTrouble labels a missing or failed helper for a pane heading, where there
// is no room for the whole story. The pane's body carries that.
func standupTrouble(err error) string {
	if errors.Is(err, standup.ErrNotInstalled) {
		return standup.Binary + " missing"
	}
	return "unavailable"
}

// renderNotes lays the journal window out: one heading per day, then its notes.
func (m dailyModel) renderNotes() string {
	t := m.theme
	width := m.notes.Width

	var lines []string
	for i, day := range m.daily.Days {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.renderDayHeading(day, i, width))

		if len(day.Notes) == 0 {
			lines = append(lines, t.Muted.Render("  nothing written"))
			continue
		}
		// A blank line between notes, so two of them are never read as one.
		for j, note := range day.Notes {
			if j > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, renderDailyNote(t, note, width)...)
		}
	}

	if m.daily.Notes() == 0 {
		lines = append(lines, "",
			t.Muted.Render("Press ")+t.HelpKey.Render("i")+t.Muted.Render(" and type to add a line, or ")+
				t.HelpKey.Render("e")+t.Muted.Render(" to open today's entry."))
	}
	return strings.Join(lines, "\n")
}

// renderDayHeading labels a day, naming today and yesterday rather than making
// the reader work the dates out. The days come newest first, so their position
// says which is which.
func (m dailyModel) renderDayHeading(day journal.DailyDay, index, width int) string {
	t := m.theme
	label := day.Date.Format("Monday 2 January")
	switch index {
	case 0:
		label = "Today · " + label
	case 1:
		label = "Yesterday · " + label
	}

	style := t.Muted
	if day.HasEntry {
		style = t.Accent.Bold(true)
	}
	return style.Render(truncate(label, width))
}

// dailyTimeGutter is the column the time lives in, `09:12` and the space that
// keeps it off the text.
const dailyTimeGutter = 7

// minGutteredTextWidth is the narrowest the text may get before the gutter costs
// more than it gives, and the time goes back above its note. Below it, the seven
// columns are worth more to the prose than to the alignment.
const minGutteredTextWidth = 30

// renderDailyNote renders one note: its time in a gutter of its own, then the
// note's markdown beside it, spined so a note over several lines reads as one.
//
// Keeping the time out of the text is what makes a day scannable — the eye runs
// down the left edge rather than through the prose.
func renderDailyNote(t *Theme, note journal.DailyNote, width int) []string {
	spine := t.Rule.Render("│") + " "
	// The time of a note written without one, so the gutter still lines up.
	head := note.Time
	if head == "" {
		head = "·"
	}

	if textWidth := width - 2 - dailyTimeGutter; textWidth >= minGutteredTextWidth {
		body := wrapped(t, note.Text, textWidth)
		gutter := dailyTimeStyle(t).Render(head) +
			strings.Repeat(" ", dailyTimeGutter-lipgloss.Width(head))
		blank := strings.Repeat(" ", dailyTimeGutter)

		lines := make([]string, 0, len(body))
		for i, line := range body {
			if i == 0 {
				lines = append(lines, spine+gutter+line)
				continue
			}
			lines = append(lines, spine+blank+line)
		}
		return lines
	}

	// Too narrow for a gutter: the time keeps a line of its own, still standing
	// out from the text under it.
	textWidth := max(width-2, 8)
	lines := []string{spine + dailyTimeStyle(t).Render(head)}
	for _, line := range wrapped(t, note.Text, textWidth) {
		lines = append(lines, spine+line)
	}
	return lines
}

// dailyTimeStyle marks a note's time. It takes a colour of its own rather than
// the day heading's, so the two levels of the timeline stay apart.
func dailyTimeStyle(t *Theme) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(t.Palette.Info)
}

// wrapped renders markdown to a fixed width and returns its lines.
func wrapped(t *Theme, text string, width int) []string {
	body := lipgloss.NewStyle().Width(width).Render(RenderMarkdown(t, text, width))
	return strings.Split(body, "\n")
}

// renderGit lays the git activity out: one heading per repository, then its
// commits. While the helper runs, or when it cannot, this is where it is said.
func (m dailyModel) renderGit() string {
	t := m.theme
	width := m.git.Width

	if m.loading {
		return renderStandupReading(t, m.spin)
	}
	if m.standupErr != nil {
		return renderStandupProblem(t, m.standupErr, width)
	}
	if m.report.Empty() {
		return t.Muted.Render("No commit in this window.")
	}
	return strings.Join(renderStandupRepos(t, m.report, width), "\n")
}

// renderStandupReading is what a git pane shows while the helper walks the
// repositories, which takes seconds rather than milliseconds.
func renderStandupReading(t *Theme, spin spinner.Model) string {
	return spin.View() + t.Muted.Render("Reading git activity…")
}

// renderStandupRepos lays a report out: one heading per repository, then its
// commits. It is shared by the stand-up view and the calendar's git panel.
func renderStandupRepos(t *Theme, report standup.Report, width int) []string {
	var lines []string
	for i, repo := range report.Repos {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, spread(
			t.Accent.Bold(true).Render(truncate(repo.Name(), width-6)),
			t.Muted.Render(fmt.Sprint(len(repo.Commits))),
			width))
		for _, commit := range repo.Commits {
			lines = append(lines, renderCommit(t, commit, width)...)
		}
	}
	return lines
}

// renderStandupProblem explains why a git pane is empty, and what to do about it.
// A missing helper is the one case worth a hint of its own.
func renderStandupProblem(t *Theme, err error, width int) string {
	body := lipgloss.NewStyle().Width(width)

	if errors.Is(err, standup.ErrNotInstalled) {
		// Both lines are wrapped: the calendar's panel is narrow enough that even the
		// first one does not fit, and a line cut mid-word reads as a bug.
		return strings.Join([]string{
			body.Render(t.Warning.Render(standup.Binary + " is not installed.")),
			"",
			body.Render(t.Muted.Render(standup.InstallHint)),
		}, "\n")
	}
	return body.Render(t.Warning.Render(err.Error()))
}

// renderCommit renders one commit: its hash and subject, with the date pushed to
// the right when there is room for it and on a line of its own when there is not.
func renderCommit(t *Theme, commit standup.Commit, width int) []string {
	textWidth := max(width-2, 8)
	spine := t.Rule.Render("│") + " "

	hash := t.Muted.Render(commit.Hash) + " "
	when := t.Muted.Render(commit.When)
	subjectWidth := textWidth - lipgloss.Width(hash)

	// The date only earns its place on the line when the subject still has half
	// of it, which is what keeps `fix: …` from being cut down to `fi…`.
	if room := subjectWidth - lipgloss.Width(when) - 2; room >= subjectWidth/2 {
		return []string{spine + hash + spread(truncate(commit.Subject, room), when, subjectWidth)}
	}
	return []string{
		spine + hash + truncate(commit.Subject, subjectWidth),
		spine + t.Muted.Render(truncate(commit.When, textWidth)),
	}
}

// renderCaptureBox draws the input, whose border lights up while it has the
// keyboard.
func (m dailyModel) renderCaptureBox() string {
	t := m.theme
	width := m.width - 2 - paneChrome

	border := t.Palette.Border
	if m.typing {
		border = t.Palette.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, panePadding).
		Width(max(width, minDailyColumnWidth) + 2*panePadding).
		Render(m.input.View())
}

// renderFooter shows the key hints, or the outcome of the last capture or edit.
func (m dailyModel) renderFooter() string {
	t := m.theme
	switch {
	case m.err != nil:
		return t.Danger.Render("! " + truncate(m.err.Error(), m.width-4))
	case m.status != "":
		return t.Success.Render("✓ " + m.status)
	}
	return t.Keys(m.footerHints()...)
}

// footerHints is the hint table the footer shows, as it ends up on screen. The
// clicking is driven off the same list, so a hint that was dropped for want of
// room cannot be clicked either.
func (m dailyModel) footerHints() [][2]string {
	hints := [][2]string{{"i", "type a note"}, {"e", "edit today"}, {"tab", "pane"},
		{"j k", "scroll"}, {"r", "refresh"}, {"q", "quit"}}
	if m.typing {
		hints = [][2]string{{"enter", "add to today"}, {"esc", "stop typing"}, {"ctrl+c", "quit"}}
	}
	// Drop the least useful hints rather than let the footer wrap.
	for len(hints) > 1 && lipgloss.Width(m.theme.Keys(hints...)) > m.width-2 {
		hints = hints[:len(hints)-1]
	}
	return hints
}

// --- static output -----------------------------------------------------------

// RenderDailyStatic renders the stand-up without the interactive chrome, for
// piped output: the journal window, then the git activity, one under the other so
// the whole thing can be pasted into a stand-up thread.
func RenderDailyStatic(theme *Theme, daily journal.Daily, report standup.Report, standupErr error, width int) string {
	width = max(width, minDailyColumnWidth)
	var b strings.Builder

	b.WriteString("\n  " + theme.Title.Render("Daily") + "  " +
		theme.Value.Render(daily.To.Format("Monday 2 January 2006")) + "\n")
	b.WriteString("  " + theme.Muted.Render(
		daily.From.Format("Mon 2 Jan")+" → "+daily.To.Format("Mon 2 Jan")+"  ·  "+
			plural(daily.Notes(), "note", "notes")+"  ·  "+
			plural(daily.Words(), "word", "words")) + "\n\n")

	b.WriteString("  " + theme.Accent.Bold(true).Render("JOURNAL") + "\n")
	b.WriteString("  " + theme.HRule(width-2) + "\n")
	for _, day := range daily.Days {
		b.WriteString("\n  " + theme.Value.Render(day.Date.Format("Monday 2 January")) + "\n")
		if len(day.Notes) == 0 {
			b.WriteString("    " + theme.Muted.Render("nothing written") + "\n")
			continue
		}
		for i, note := range day.Notes {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(Indent(strings.Join(renderDailyNote(theme, note, width-4), "\n")) + "\n")
		}
	}

	b.WriteString("\n  " + theme.Accent.Bold(true).Render("GIT") + "\n")
	b.WriteString("  " + theme.HRule(width-2) + "\n")
	switch {
	case errors.Is(standupErr, standup.ErrNotInstalled):
		b.WriteString("\n  " + theme.Warning.Render(standup.Binary+" is not installed.") + "\n")
		b.WriteString("  " + theme.Muted.Render(standup.InstallHint) + "\n")
	case standupErr != nil:
		b.WriteString("\n" + Indent(theme.Warning.Render(standupErr.Error())) + "\n")
	case report.Empty():
		b.WriteString("\n  " + theme.Muted.Render("No commit in this window.") + "\n")
	default:
		for _, repo := range report.Repos {
			b.WriteString("\n  " + theme.Value.Render(repo.Name()) + "\n")
			for _, commit := range repo.Commits {
				b.WriteString(Indent(strings.Join(renderCommit(theme, commit, width-4), "\n")) + "\n")
			}
		}
	}
	b.WriteString("\n")
	return b.String()
}
