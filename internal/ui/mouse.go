package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mouse support is hit testing, not a widget tree: every view already lays
// itself out with arithmetic, so the same arithmetic says what a click landed
// on. The rectangles below are computed from the model, never from the rendered
// string, which keeps them testable without a terminal.

// rect is a rectangle of terminal cells, in screen coordinates. A rectangle of
// zero width or height is empty and contains nothing, which is what an off-screen
// pane returns.
type rect struct {
	x, y, w, h int
}

func (r rect) contains(x, y int) bool {
	return r.w > 0 && r.h > 0 &&
		x >= r.x && x < r.x+r.w &&
		y >= r.y && y < r.y+r.h
}

// programOptions are what every full-screen view starts with. Without the mouse,
// the terminal keeps it, and dragging the pointer selects text as it does
// anywhere else.
func programOptions(mouse bool) []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	return opts
}

// mouseAgain turns mouse reporting back on. Handing the terminal to the editor
// releases it, and Bubble Tea puts the alternate screen and the bracketed paste
// back afterwards but not the mouse, so every view asks for it again when the
// editor comes back — unless it was started without the mouse.
func mouseAgain(mouse bool) tea.Cmd {
	if !mouse {
		return nil
	}
	return tea.EnableMouseCellMotion
}

// leftClick reports whether the event is the left button going down. Releases
// and drags are ignored: acting on the press is what makes a click feel
// immediate.
func leftClick(msg tea.MouseMsg) bool {
	return msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft
}

// wheel returns -1 for a scroll up, 1 for a scroll down and 0 for anything else,
// so a caller can hand it straight to a scrolling function.
func wheel(msg tea.MouseMsg) int {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return -1
	case tea.MouseButtonWheelDown:
		return 1
	}
	return 0
}

// wheelLines is how far one notch of the wheel scrolls, the three lines a
// viewport moves by default.
const wheelLines = 3

// scrollBy moves a viewport n lines, up when n is negative.
func scrollBy(v *viewport.Model, n int) {
	if n < 0 {
		v.ScrollUp(-n)
		return
	}
	v.ScrollDown(n)
}

// --- key hints ---------------------------------------------------------------

// hintKeys maps the glyphs the hint tables show to the keys they stand for, so a
// click on "← →" reaches the same code as pressing the arrow.
var hintKeys = map[string]string{
	"←": "left",
	"→": "right",
	"↑": "up",
	"↓": "down",
}

// hintKeyAt returns the key a click at offset columns into a hint's key field
// stands for.
//
// A hint naming a single key answers for its whole row, label included: there is
// nothing else it could mean. A hint naming two, like "← →" or "p n", only
// answers on the keys themselves, because its label belongs to both.
func hintKeyAt(keys string, offset int, wholeRow bool) (string, bool) {
	tokens := strings.Fields(keys)
	switch {
	case len(tokens) == 0:
		return "", false
	case len(tokens) == 1 && wholeRow:
		return hintKey(tokens[0]), true
	}

	// Walk the field the way it is rendered, a single space between tokens. Each
	// token answers for its own cells and the space after it, so a click that
	// lands just beside a narrow glyph still counts.
	col := 0
	for _, token := range tokens {
		width := lipgloss.Width(token)
		if offset >= col && offset <= col+width {
			return hintKey(token), true
		}
		col += width + 1
	}
	return "", false
}

// hintKey is the key a hint glyph stands for, which is the glyph itself unless
// it is one of the arrows.
func hintKey(glyph string) string {
	if key, ok := hintKeys[glyph]; ok {
		return key
	}
	return glyph
}

// footerKeyAt returns the key a click at offset columns into a Theme.Keys footer
// stands for. It walks the layout Theme.Keys renders — each pair as its key, a
// space and its label, pairs joined by "  ·  " — so the two must change
// together.
func footerKeyAt(hints [][2]string, offset int) (string, bool) {
	const separator = 5 // "  ·  "

	col := 0
	for i, hint := range hints {
		if i > 0 {
			col += separator
		}
		keysWidth := lipgloss.Width(hint[0])
		width := keysWidth + 1 + lipgloss.Width(hint[1])
		if offset >= col && offset < col+width {
			// Single-key hints answer for their label too, pairs only for their keys.
			return hintKeyAt(hint[0], offset-col, true)
		}
		col += width
	}
	return "", false
}
