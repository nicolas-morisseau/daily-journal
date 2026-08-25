package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"daily-journal/internal/config"
	"daily-journal/internal/journal"
)

// journalsCommand lists the journals of the root.
type journalsCommand struct{}

func (journalsCommand) Name() string    { return "journals" }
func (journalsCommand) Usage() string   { return "journals" }
func (journalsCommand) Summary() string { return "List your journals and show the active one" }
func (journalsCommand) Flags() []Flag   { return nil }
func (journalsCommand) AcrossJournals() {}

func (journalsCommand) Run(a *App, _ FlagValues, operands []string) error {
	if err := noOperands("journals", operands); err != nil {
		return err
	}
	return a.listJournals()
}

// newCommand creates a journal and switches to it.
type newCommand struct{}

func (newCommand) Name() string    { return "new" }
func (newCommand) Usage() string   { return "new NAME" }
func (newCommand) Summary() string { return "Create a journal and make it the active one" }
func (newCommand) Flags() []Flag   { return nil }
func (newCommand) AcrossJournals() {}

func (newCommand) Run(a *App, _ FlagValues, operands []string) error {
	name, err := oneJournalName("new", operands)
	if err != nil {
		return err
	}
	return a.newJournal(name)
}

// useCommand switches the active journal.
type useCommand struct{}

func (useCommand) Name() string    { return "use" }
func (useCommand) Usage() string   { return "use NAME" }
func (useCommand) Summary() string { return "Switch the active journal until you switch again" }
func (useCommand) Flags() []Flag   { return nil }
func (useCommand) AcrossJournals() {}

func (useCommand) Run(a *App, _ FlagValues, operands []string) error {
	name, err := oneJournalName("use", operands)
	if err != nil {
		return err
	}
	return a.useJournal(name)
}

// oneJournalName reads the single journal name these commands take. A name is
// one word: anything else is a mistyped command line, not a journal.
func oneJournalName(command string, operands []string) (string, error) {
	switch len(operands) {
	case 1:
		return operands[0], nil
	case 0:
		return "", usagef("%s needs a journal name\n\nRun 'journal journals' to see the ones you have", command)
	default:
		return "", usagef("%s takes one journal name, got %q", command, strings.Join(operands, " "))
	}
}

// listJournals prints every journal of the root, marking the active one.
func (a *App) listJournals() error {
	names, err := journal.List(a.Config.JournalsRoot)
	if err != nil {
		return err
	}

	// The active journal belongs in the listing even before it exists on disk:
	// on a first run nothing has been created yet, and an empty list with no
	// mention of the journal about to be written would be a lie.
	active := a.Config.ActiveJournal
	missing := !slices.Contains(names, active)
	if missing {
		names = append(names, active)
		slices.Sort(names)
	}

	p, t := a.Printer, a.Theme
	p.Blank()
	p.Println("  " + t.Accent.Render("JOURNALS"))

	width := 0
	for _, name := range names {
		if n := lipgloss.Width(name); n > width {
			width = n
		}
	}
	for _, name := range names {
		marker, style := "  ", t.Value
		if name == active {
			marker, style = t.Accent.Render("* "), t.Title
		}
		pad := strings.Repeat(" ", width-lipgloss.Width(name))
		line := "    " + marker + style.Render(name) + pad + "   " + t.Muted.Render(a.journalDetail(name))
		p.Println(line)
	}
	p.Blank()
	p.Field("Journals root", a.Config.JournalsRoot, 18)
	p.Field("Active journal", activeLabel(a.Config), 18)
	p.Blank()
	return nil
}

// journalDetail describes one journal in the listing: how much is in it, or why
// there is nothing to count.
func (a *App) journalDetail(name string) string {
	if !journal.HasJournal(a.Config.JournalsRoot, name) {
		return "not created yet"
	}
	entries, err := journal.New(journal.DirFor(a.Config.JournalsRoot, name), a.Now).DatedEntries()
	if err != nil {
		return "unreadable"
	}
	if len(entries) == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", len(entries))
}

// newJournal creates a journal and activates it. Creating one you are not about
// to write in is not a thing anyone wants, so the two go together.
func (a *App) newJournal(name string) error {
	if err := journal.ValidateName(name); err != nil {
		return usagef("%s", err.Error())
	}
	dir, err := journal.Create(a.Config.JournalsRoot, name)
	if err != nil {
		return err
	}
	a.Printer.Success("Created journal %s at %s", name, dir)
	return a.activate(name)
}

// useJournal switches to an existing journal.
func (a *App) useJournal(name string) error {
	if err := journal.ValidateName(name); err != nil {
		return usagef("%s", err.Error())
	}
	// The default journal is allowed not to exist yet: it is the one the first
	// run creates, so switching back to it must always work.
	exists := journal.HasJournal(a.Config.JournalsRoot, name) || name == a.Config.DefaultJournal
	if name == a.Config.ActiveJournal && exists {
		a.Printer.Muted("Already using %s.", name)
		return nil
	}
	if !exists {
		return fmt.Errorf("no journal named %q in %s\n\nCreate it with:\n  journal new %s",
			name, a.Config.JournalsRoot, name)
	}
	return a.activate(name)
}

// activate records the journal to use from now on, and warns when the
// environment will keep overriding what was just recorded.
func (a *App) activate(name string) error {
	overridden := a.Config.ActiveFrom == config.FromEnv

	if err := config.WriteActive(a.Config.StatePath, name); err != nil {
		return err
	}
	if err := a.setJournal(name, config.FromState); err != nil {
		return err
	}
	a.Printer.Success("Now using %s", name)

	if overridden {
		a.Printer.Warn("%s is set and takes precedence, so other commands will keep using another journal.",
			config.EnvJournal)
	}
	return nil
}
