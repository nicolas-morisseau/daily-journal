package cli

import (
	"errors"
	"strings"

	"daily-journal/internal/config"
)

// Command is one subcommand of the CLI. Adding a feature means adding a Command
// to Commands and nothing else: dispatch and help are both driven by this list.
type Command interface {
	// Name is the word the user types.
	Name() string
	// Usage is the one-line synopsis without the program name, e.g. "open [DATE]".
	Usage() string
	// Summary is the one-line description shown in the command list.
	Summary() string
	// Flags declares the command's options.
	Flags() []Flag
	// Run executes the command with its parsed flags and remaining operands.
	Run(a *App, flags FlagValues, operands []string) error
}

// freeTextCommand is implemented by commands whose operands are arbitrary text.
// Option parsing stops at their first operand so a note can contain anything.
type freeTextCommand interface {
	Command
	// FreeText distinguishes the interface; it does nothing.
	FreeText()
}

// rootCommand is implemented by commands that work on the set of journals, or
// on nothing at all. They run even when the active journal does not exist,
// which is what lets `journal new` be the way out of that situation.
type rootCommand interface {
	Command
	// AcrossJournals distinguishes the interface; it does nothing.
	AcrossJournals()
}

// Commands is the full command set, in the order the help screen lists them.
func Commands() []Command {
	return []Command{
		openCommand{},
		addCommand{},
		searchCommand{},
		weekCommand{},
		calendarCommand{},
		dailyCommand{},
		reviewCommand{},
		journalsCommand{},
		newCommand{},
		useCommand{},
		versionCommand{},
		helpCommand{},
	}
}

// DefaultCommand runs when no subcommand is given: open today's entry.
func DefaultCommand() Command { return openCommand{} }

// CaptureCommand runs when the first argument is neither a subcommand nor a
// flag, which makes `journal "a thought"` a quick capture.
func CaptureCommand() Command { return addCommand{} }

// Lookup finds a command by name.
func Lookup(name string) (Command, bool) {
	for _, cmd := range Commands() {
		if cmd.Name() == name {
			return cmd, true
		}
	}
	return nil, false
}

// Run resolves argv and executes the matching command.
//
// Resolution order, which is what lets quick capture coexist with subcommands:
//
//  0. --journal NAME          -> selects the journal, then drops out of argv
//  1. no arguments            -> open today's entry
//  2. -h / --help             -> the help screen
//  3. an exact command name   -> that command
//  4. `--`                    -> quick capture, verbatim
//  5. anything starting with - -> flags of the default command
//  6. anything else           -> quick capture
//
// Step 3 matches the whole argument, so quoted multi-word text can never be
// mistaken for a command: "list of things to buy" is not equal to "list".
func (a *App) Run(argv []string) error {
	argv, selected, err := extractJournalFlag(argv)
	if err != nil {
		return err
	}
	if selected != "" {
		if err := a.setJournal(selected, config.FromFlag); err != nil {
			return err
		}
	}

	if len(argv) == 0 {
		return a.exec(DefaultCommand(), nil)
	}

	first := argv[0]
	switch {
	case first == "-h" || first == "--help":
		a.showHelp()
		return nil
	case first == "-V" || first == "--version":
		a.Printer.Println(a.Build.String())
		return nil
	case first == "--":
		// parseFlags treats everything after `--` as operands, so hand the
		// separator through untouched.
		return a.exec(CaptureCommand(), argv)
	}
	if cmd, ok := Lookup(first); ok {
		return a.exec(cmd, argv[1:])
	}
	if strings.HasPrefix(first, "-") {
		return a.exec(DefaultCommand(), argv)
	}
	return a.exec(CaptureCommand(), argv)
}

// journalFlag is the global option selecting a journal for one invocation. It
// is not declared on any command: it is recognised before dispatch so that it
// works in front of a subcommand and in front of a quick capture alike.
const (
	journalFlagLong  = "--journal"
	journalFlagShort = "-j"
)

// extractJournalFlag pulls the global --journal option out of argv and returns
// what is left.
//
// It stops at `--`, so `journal -- -j` still captures the literal text. It
// accepts the same shapes as the per-command parser: `-j NAME`, `-jNAME`,
// `-j=NAME`, `--journal NAME` and `--journal=NAME`.
func extractJournalFlag(argv []string) (rest []string, name string, err error) {
	rest = make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			rest = append(rest, argv[i:]...)
			return rest, name, nil
		}

		var value string
		var hasValue bool
		switch {
		case arg == journalFlagLong || arg == journalFlagShort:
		case strings.HasPrefix(arg, journalFlagLong+"="):
			value, hasValue = strings.TrimPrefix(arg, journalFlagLong+"="), true
		case strings.HasPrefix(arg, journalFlagShort+"="):
			value, hasValue = strings.TrimPrefix(arg, journalFlagShort+"="), true
		case strings.HasPrefix(arg, journalFlagShort) && len(arg) > 2:
			value, hasValue = arg[2:], true
		default:
			rest = append(rest, arg)
			continue
		}

		if !hasValue {
			if i+1 >= len(argv) {
				return nil, "", usagef("option %s requires a journal name", arg)
			}
			value = argv[i+1]
			i++
		}
		if value == "" {
			return nil, "", usagef("option %s requires a journal name", arg)
		}
		name = value
	}
	return rest, name, nil
}

// exec parses a command's arguments and runs it.
func (a *App) exec(cmd Command, args []string) error {
	mode := parseAnywhere
	if _, freeText := cmd.(freeTextCommand); freeText {
		mode = parseUntilOperand
	}

	// `journal <cmd> --help` prints the command's help, even for free-text
	// commands. Use `journal add -- --help` to capture the literal text.
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		a.showCommandHelp(cmd)
		return nil
	}

	flags, operands, err := parseFlags(cmd.Flags(), args, mode)
	if err != nil {
		return withUsageHint(cmd, err)
	}
	if _, acrossJournals := cmd.(rootCommand); !acrossJournals {
		if err := a.requireJournal(); err != nil {
			return err
		}
	}
	return cmd.Run(a, flags, operands)
}

// withUsageHint appends the command's synopsis to a usage error, so a mistyped
// option always shows the right shape.
func withUsageHint(cmd Command, err error) error {
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		return err
	}
	return usagef("%s\n\nUsage: journal %s", usageErr.msg, cmd.Usage())
}
