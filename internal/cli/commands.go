package cli

import (
	"strconv"
	"strings"

	"daily-journal/internal/journal"
)

// openCommand is the default: open one entry in the editor.
type openCommand struct{}

func (openCommand) Name() string    { return "open" }
func (openCommand) Usage() string   { return "open [DATE]" }
func (openCommand) Summary() string { return "Open an entry in your editor (default: today)" }

func (openCommand) Flags() []Flag {
	return []Flag{
		{Long: "last", Help: "Open the most recent entry instead of a date"},
	}
}

func (openCommand) Run(a *App, flags FlagValues, operands []string) error {
	// A date can arrive as several words: `journal open last friday`.
	date := strings.Join(operands, " ")
	last := flags.Bool("last")
	if last && date != "" {
		return usagef("--last cannot be combined with a date")
	}
	return a.openEntry(date, last)
}

// addCommand appends a line without opening the editor. It is what runs for the
// bare `journal "a thought"` form.
type addCommand struct{}

func (addCommand) Name() string    { return "add" }
func (addCommand) Usage() string   { return "add [--date DATE] TEXT..." }
func (addCommand) Summary() string { return "Append a timestamped line without opening the editor" }
func (addCommand) FreeText()       {}

func (addCommand) Flags() []Flag {
	return []Flag{
		{Long: "date", Short: "d", Arg: "DATE", Help: "Entry to append to (default: today)"},
	}
}

func (addCommand) Run(a *App, flags FlagValues, operands []string) error {
	text := strings.Join(operands, " ")
	if strings.TrimSpace(text) == "" {
		return usagef("nothing to add: give the text to append")
	}
	return a.quickCapture(flags.String("date"), text)
}

// searchCommand runs a full-text search, optionally narrowed by tag.
type searchCommand struct{}

func (searchCommand) Name() string  { return "search" }
func (searchCommand) Usage() string { return "search [--tag TAG]... QUERY..." }
func (searchCommand) Summary() string {
	return "Search the text of every entry, optionally within tags"
}

func (searchCommand) Flags() []Flag {
	return []Flag{
		{
			Long: "tag", Arg: "TAG", Repeatable: true,
			Help: "Only entries carrying #TAG; repeat to require several",
		},
	}
}

func (searchCommand) Run(a *App, flags FlagValues, operands []string) error {
	query := strings.Join(operands, " ")
	if strings.TrimSpace(query) == "" {
		return usagef("nothing to search for: give a query")
	}
	return a.search(query, flags.Strings("tag"))
}

// weekCommand opens the current week's entries.
type weekCommand struct{}

func (weekCommand) Name() string    { return "week" }
func (weekCommand) Usage() string   { return "week" }
func (weekCommand) Summary() string { return "Open every entry from the current week" }
func (weekCommand) Flags() []Flag   { return nil }

func (weekCommand) Run(a *App, _ FlagValues, operands []string) error {
	if err := noOperands("week", operands); err != nil {
		return err
	}
	return a.week()
}

// calendarCommand shows a month at a glance.
type calendarCommand struct{}

func (calendarCommand) Name() string    { return "calendar" }
func (calendarCommand) Usage() string   { return "calendar [YYYY-MM]" }
func (calendarCommand) Summary() string { return "Month calendar with entry indicators" }
func (calendarCommand) Flags() []Flag   { return nil }

func (calendarCommand) Run(a *App, _ FlagValues, operands []string) error {
	if len(operands) > 1 {
		return usagef("calendar takes at most one month")
	}
	month := ""
	if len(operands) == 1 {
		month = operands[0]
	}
	return a.calendar(month)
}

// dailyCommand shows the last day at a glance, for the stand-up.
type dailyCommand struct{}

func (dailyCommand) Name() string  { return "daily" }
func (dailyCommand) Usage() string { return "daily [--days N]" }
func (dailyCommand) Summary() string {
	return "Stand-up recap: the last day's notes and git commits"
}

func (dailyCommand) Flags() []Flag {
	return []Flag{
		{
			Long: "days", Short: "d", Arg: "N", Default: strconv.Itoa(journal.DefaultDailyDays),
			Help: "How many days back to cover, as git-standup counts them",
		},
	}
}

func (dailyCommand) Run(a *App, flags FlagValues, operands []string) error {
	if err := noOperands("daily", operands); err != nil {
		return err
	}
	return a.daily(flags.String("days"))
}

// reviewCommand shows the good/bad/mad notes of a period side by side.
type reviewCommand struct{}

func (reviewCommand) Name() string  { return "review" }
func (reviewCommand) Usage() string { return "review [--weeks N] [--from DATE] [--to DATE]" }
func (reviewCommand) Summary() string {
	return "Sprint review: #good, #bad and #mad notes side by side"
}

func (reviewCommand) Flags() []Flag {
	return []Flag{
		{
			Long: "weeks", Short: "w", Arg: "N", Default: strconv.Itoa(journal.DefaultReviewWeeks),
			Help: "Length of the rolling window, in weeks",
		},
		{Long: "from", Arg: "DATE", Help: "First day covered, instead of the rolling window"},
		{Long: "to", Arg: "DATE", Help: "Last day covered (default: today)"},
	}
}

func (reviewCommand) Run(a *App, flags FlagValues, operands []string) error {
	if err := noOperands("review", operands); err != nil {
		return err
	}
	return a.review(reviewWindow{
		Weeks: flags.String("weeks"),
		From:  flags.String("from"),
		To:    flags.String("to"),
	})
}

// versionCommand reports the build, which is what a bug report needs.
type versionCommand struct{}

func (versionCommand) Name() string    { return "version" }
func (versionCommand) Usage() string   { return "version" }
func (versionCommand) Summary() string { return "Print the version of this build" }
func (versionCommand) Flags() []Flag   { return nil }

func (versionCommand) Run(a *App, _ FlagValues, operands []string) error {
	if err := noOperands("version", operands); err != nil {
		return err
	}
	a.Printer.Println(a.Build.String())
	return nil
}

// helpCommand documents the tool.
type helpCommand struct{}

func (helpCommand) Name() string    { return "help" }
func (helpCommand) Usage() string   { return "help [COMMAND]" }
func (helpCommand) Summary() string { return "Show help for the tool or for one command" }
func (helpCommand) Flags() []Flag   { return nil }

func (helpCommand) Run(a *App, _ FlagValues, operands []string) error {
	switch len(operands) {
	case 0:
		a.showHelp()
		return nil
	case 1:
		cmd, ok := Lookup(operands[0])
		if !ok {
			return usagef("unknown command %q\nRun 'journal help' for the command list", operands[0])
		}
		a.showCommandHelp(cmd)
		return nil
	default:
		return usagef("help takes at most one command name")
	}
}

// noOperands rejects stray arguments on commands that take none.
func noOperands(name string, operands []string) error {
	if len(operands) == 0 {
		return nil
	}
	return usagef("%s takes no arguments, got %q", name, strings.Join(operands, " "))
}
