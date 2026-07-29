package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// examples sit under the command list and cover the shapes that a synopsis
// alone does not make obvious.
var examples = [][2]string{
	{`journal`, "Open today's entry"},
	{`journal "Had a breakthrough today"`, "Quick capture, no editor"},
	{`journal open yesterday`, "Any date expression works"},
	{`journal open "last friday"`, ""},
	{`journal calendar`, "Browse the month, read and edit entries"},
	{`journal review`, "Good, bad and mad over the last two weeks"},
	{`journal review --weeks 3`, "Any window length"},
	{`journal search roadmap`, "Full-text search"},
	{`journal search --tag work --tag bad`, "Narrow it to tagged entries"},
}

// configRows documents the keys read from the config file.
var configRows = [][2]string{
	{"AUTOSYNC=true", "Commit and push without asking"},
	{"GIT_BRANCH=main", "Branch used for pull and push"},
	{`JOURNAL_DIR="$HOME/journal"`, "Where the journal lives"},
	{"SPRINT_START=2026-07-06", "First day of a sprint, for 'journal review'"},
}

// showHelp prints the top-level usage screen.
func (a *App) showHelp() {
	p, t := a.Printer, a.Theme

	p.Blank()
	p.Println("  " + t.Title.Render("daily-journal") + "  " + t.Muted.Render("minimal journaling for the terminal"))
	p.Blank()
	p.Println("  " + t.Label.Render("Usage") + "  " + t.Value.Render("journal [COMMAND] [OPTIONS]"))
	p.Println("  " + t.Label.Render("     ") + "  " + t.Value.Render(`journal "text to capture"`))
	p.Blank()

	commands := Commands()
	width := 0
	for _, cmd := range commands {
		if n := lipgloss.Width(cmd.Usage()); n > width {
			width = n
		}
	}

	p.Println("  " + t.Accent.Render("COMMANDS"))
	for _, cmd := range commands {
		pad := strings.Repeat(" ", width-lipgloss.Width(cmd.Usage()))
		p.Println("    " + t.Value.Render(cmd.Usage()) + pad + "   " + t.Muted.Render(cmd.Summary()))
	}
	p.Blank()

	p.Println("  " + t.Accent.Render("EXAMPLES"))
	exampleWidth := 0
	for _, row := range examples {
		if n := lipgloss.Width(row[0]); n > exampleWidth {
			exampleWidth = n
		}
	}
	for _, row := range examples {
		pad := strings.Repeat(" ", exampleWidth-lipgloss.Width(row[0]))
		line := "    " + t.Value.Render(row[0]) + pad
		if row[1] != "" {
			line += "   " + t.Muted.Render(row[1])
		}
		p.Println(line)
	}
	p.Blank()

	p.Println("  " + t.Muted.Render("Run 'journal help COMMAND' for a command's options."))
	p.Println("  " + t.Muted.Render("Text starting with a command name needs 'journal add' or 'journal -- text'."))
	p.Blank()

	a.showConfiguration()
}

// showCommandHelp prints the synopsis and options of one command.
func (a *App) showCommandHelp(cmd Command) {
	p, t := a.Printer, a.Theme

	p.Blank()
	p.Println("  " + t.Title.Render(cmd.Name()) + "  " + t.Muted.Render(cmd.Summary()))
	p.Blank()
	p.Println("  " + t.Label.Render("Usage") + "  " + t.Value.Render("journal "+cmd.Usage()))
	p.Blank()

	flags := cmd.Flags()
	if len(flags) == 0 {
		p.Println("  " + t.Muted.Render("This command takes no options."))
		p.Blank()
		return
	}

	width := 0
	for _, flag := range flags {
		if n := lipgloss.Width(flag.Names()); n > width {
			width = n
		}
	}
	p.Println("  " + t.Accent.Render("OPTIONS"))
	for _, flag := range flags {
		pad := strings.Repeat(" ", width-lipgloss.Width(flag.Names()))
		line := "    " + t.Value.Render(flag.Names()) + pad + "   " + t.Muted.Render(flag.Help)
		if flag.Default != "" {
			line += t.Muted.Render(" (default: " + flag.Default + ")")
		}
		p.Println(line)
	}
	p.Blank()
}

// showConfiguration prints where the journal and its settings live.
func (a *App) showConfiguration() {
	p, t := a.Printer, a.Theme

	p.Println("  " + t.Accent.Render("CONFIGURATION"))
	p.Field("Journal directory", a.Journal.Dir, 18)
	p.Field("Config file", a.Config.Path, 18)
	p.Field("Editor", editorLabel(a.Config.Editor), 18)
	p.Blank()

	width := 0
	for _, row := range configRows {
		if n := lipgloss.Width(row[0]); n > width {
			width = n
		}
	}
	p.Println("    " + t.Muted.Render("Keys in that file:"))
	for _, row := range configRows {
		pad := strings.Repeat(" ", width-lipgloss.Width(row[0]))
		p.Println("      " + t.Value.Render(row[0]) + pad + "   " + t.Muted.Render(row[1]))
	}
	p.Blank()
	p.Println("  " + t.Muted.Render("Entry template: ") + t.Value.Render(a.Journal.TemplatePath()) +
		t.Muted.Render("  ({{date}}, {{day_of_week}}, {{week_number}}, {{time}})"))
	p.Blank()
}

func editorLabel(configured string) string {
	if configured == "" {
		return "$EDITOR (fallback: code > vim > nano)"
	}
	return configured
}
