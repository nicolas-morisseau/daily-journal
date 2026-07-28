// Package ui renders everything daily-journal shows: styled one-shot output and
// the interactive Bubble Tea views.
//
// Every style lives in Theme so the look can be changed in one place, and every
// interactive view degrades to plain text when stdout is not a terminal.
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Palette is the colour set shared by all views. Colours are adaptive so the
// output stays readable on light and dark terminals alike.
type Palette struct {
	Accent  lipgloss.TerminalColor
	Success lipgloss.TerminalColor
	Warning lipgloss.TerminalColor
	Danger  lipgloss.TerminalColor
	Info    lipgloss.TerminalColor
	Muted   lipgloss.TerminalColor
	Border  lipgloss.TerminalColor
	// Inverse is used as foreground on filled backgrounds.
	Inverse lipgloss.TerminalColor
}

// DefaultPalette is a calm violet-and-amber set with plenty of contrast.
var DefaultPalette = Palette{
	Accent:  lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"},
	Success: lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#86EFAC"},
	Warning: lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"},
	Danger:  lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"},
	Info:    lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#7DD3FC"},
	Muted:   lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B93A1"},
	Border:  lipgloss.AdaptiveColor{Light: "#D8D5E4", Dark: "#3F3D56"},
	Inverse: lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1A1726"},
}

// Theme holds every style used by the CLI and the TUI.
type Theme struct {
	Palette Palette

	// Title is the heading of a screen or section.
	Title lipgloss.Style
	// Subtitle sits under a title.
	Subtitle lipgloss.Style
	// Muted is for secondary information.
	Muted lipgloss.Style
	// Label names a value in a key/value pair.
	Label lipgloss.Style
	// Value renders the value of a key/value pair.
	Value lipgloss.Style
	// Accent highlights something notable.
	Accent lipgloss.Style
	// Success, Warning and Danger carry status semantics.
	Success lipgloss.Style
	Warning lipgloss.Style
	Danger  lipgloss.Style
	// Badge is a filled pill, used for the active journal and counters.
	Badge lipgloss.Style
	// Panel is a rounded bordered container.
	Panel lipgloss.Style
	// Rule draws a thin horizontal separator.
	Rule lipgloss.Style
	// SelectedItem and Item style list rows.
	SelectedItem lipgloss.Style
	Item         lipgloss.Style
	// Highlight marks a search hit inside a line.
	Highlight lipgloss.Style
	// Help renders the key hints footer.
	Help lipgloss.Style
	// HelpKey renders a single key inside the footer.
	HelpKey lipgloss.Style
}

// NewTheme builds a theme from a palette.
func NewTheme(p Palette) *Theme {
	return &Theme{
		Palette:      p,
		Title:        lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Subtitle:     lipgloss.NewStyle().Foreground(p.Muted),
		Muted:        lipgloss.NewStyle().Foreground(p.Muted),
		Label:        lipgloss.NewStyle().Foreground(p.Muted),
		Value:        lipgloss.NewStyle().Bold(true),
		Accent:       lipgloss.NewStyle().Foreground(p.Accent),
		Success:      lipgloss.NewStyle().Foreground(p.Success),
		Warning:      lipgloss.NewStyle().Foreground(p.Warning),
		Danger:       lipgloss.NewStyle().Foreground(p.Danger),
		Badge:        lipgloss.NewStyle().Foreground(p.Inverse).Background(p.Accent).Bold(true).Padding(0, 1),
		Panel:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).Padding(1, 2),
		Rule:         lipgloss.NewStyle().Foreground(p.Border),
		SelectedItem: lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Item:         lipgloss.NewStyle(),
		Highlight:    lipgloss.NewStyle().Bold(true).Foreground(p.Warning),
		Help:         lipgloss.NewStyle().Foreground(p.Muted),
		HelpKey:      lipgloss.NewStyle().Foreground(p.Accent),
	}
}

// Default is the theme used unless a caller supplies another one.
var Default = NewTheme(DefaultPalette)

// Rule renders a horizontal separator of the given width.
func (t *Theme) HRule(width int) string {
	if width < 1 {
		return ""
	}
	line := make([]rune, width)
	for i := range line {
		line[i] = '─'
	}
	return t.Rule.Render(string(line))
}

// Keys renders a footer of key/description pairs, e.g. "↑↓ move · enter open".
func (t *Theme) Keys(pairs ...[2]string) string {
	out := ""
	for i, pair := range pairs {
		if i > 0 {
			out += t.Help.Render("  ·  ")
		}
		out += t.HelpKey.Render(pair[0]) + t.Help.Render(" "+pair[1])
	}
	return out
}

// TerminalWidth returns the usable width of stdout, defaulting to 80 columns
// when the size cannot be determined.
func TerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

// TerminalHeight returns the usable height of stdout, defaulting to 24 rows.
func TerminalHeight() int {
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || height <= 0 {
		return 24
	}
	return height
}

// ContentWidth returns a comfortable text width: the terminal width, capped so
// prose never stretches across very wide windows.
func ContentWidth() int {
	width := TerminalWidth() - 4
	if width > 96 {
		width = 96
	}
	if width < 40 {
		width = 40
	}
	return width
}

// Interactive reports whether both stdin and stdout are terminals, which is the
// prerequisite for running a Bubble Tea program.
func Interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
