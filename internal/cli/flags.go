package cli

import (
	"fmt"
	"strings"
)

// Flag declares one option of a subcommand.
//
// Declaring flags as data rather than through calls means the parser and the
// help screen read from the same source, so they cannot drift apart.
type Flag struct {
	// Long is the option name without its leading dashes.
	Long string
	// Short is the single-letter form without its dash, empty when there is none.
	Short string
	// Arg is the metavariable for options that take a value. An empty Arg makes
	// the flag a boolean switch.
	Arg string
	// Help is the one-line description shown in the help screen.
	Help string
	// Default is the value used when the flag is absent, and the one advertised
	// in the help screen. Only meaningful for options that take a value.
	Default string
	// Repeatable lets the option be given several times, collecting every value.
	// A single occurrence may also carry a comma-separated list. Read the result
	// with FlagValues.Strings.
	Repeatable bool
}

// takesValue reports whether the flag expects a value.
func (f Flag) takesValue() bool { return f.Arg != "" }

// Names renders the flag the way the help screen shows it, e.g. "-t, --time".
func (f Flag) Names() string {
	var b strings.Builder
	if f.Short != "" {
		b.WriteString("-" + f.Short + ", ")
	} else {
		b.WriteString("    ")
	}
	b.WriteString("--" + f.Long)
	if f.takesValue() {
		b.WriteString(" " + f.Arg)
	}
	return b.String()
}

// FlagValues holds the result of parsing a command's flags.
type FlagValues struct {
	values map[string]string
	lists  map[string][]string
}

// Bool reports whether a boolean flag was given.
func (v FlagValues) Bool(long string) bool { return v.values[long] == "true" }

// String returns the value of an option, or its declared default. For a
// repeatable option it is the last value given.
func (v FlagValues) String(long string) string { return v.values[long] }

// Strings returns every value collected for a repeatable option, in the order
// they were given, or nil when the option was absent.
func (v FlagValues) Strings(long string) []string { return v.lists[long] }

// UsageError marks a mistake in how the command was invoked, as opposed to a
// failure while running it. The two get different exit statuses.
type UsageError struct {
	msg string
}

func (e *UsageError) Error() string { return e.msg }

// usagef builds a UsageError.
func usagef(format string, args ...any) error {
	return &UsageError{msg: fmt.Sprintf(format, args...)}
}

// parseMode controls how aggressively parseFlags looks for options.
type parseMode int

const (
	// parseAnywhere accepts flags before and after the operands.
	parseAnywhere parseMode = iota
	// parseUntilOperand stops treating arguments as flags once the first
	// operand appears. Quick capture uses it so a note can contain anything.
	parseUntilOperand
)

// parseFlags splits args into flag values and operands.
//
// It understands `--long`, `--long value`, `--long=value`, `-s`, `-s value`,
// `-svalue` and `-s=value`, and `--` ends option parsing as POSIX requires.
func parseFlags(decls []Flag, args []string, mode parseMode) (FlagValues, []string, error) {
	values := make(map[string]string, len(decls))
	for _, decl := range decls {
		values[decl.Long] = decl.Default
	}
	lists := map[string][]string{}
	parsed := FlagValues{values: values, lists: lists}

	// collect records one occurrence, splitting a comma-separated list so that
	// `--tag a,b` and `--tag a --tag b` mean the same thing.
	collect := func(decl Flag, value string) {
		values[decl.Long] = value
		if !decl.Repeatable {
			return
		}
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				lists[decl.Long] = append(lists[decl.Long], part)
			}
		}
	}

	byLong := make(map[string]Flag, len(decls))
	byShort := make(map[string]Flag, len(decls))
	for _, decl := range decls {
		byLong[decl.Long] = decl
		if decl.Short != "" {
			byShort[decl.Short] = decl
		}
	}

	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--":
			// Everything after the separator is an operand, verbatim.
			operands = append(operands, args[i+1:]...)
			return parsed, operands, nil

		case strings.HasPrefix(arg, "--"):
			name, inline, hasInline := strings.Cut(arg[2:], "=")
			decl, ok := byLong[name]
			if !ok {
				return parsed, nil, usagef("unknown option --%s", name)
			}
			value, consumed, err := flagValue(decl, "--"+name, inline, hasInline, args, i)
			if err != nil {
				return parsed, nil, err
			}
			collect(decl, value)
			i += consumed

		case len(arg) > 1 && arg[0] == '-':
			short := arg[1:2]
			decl, ok := byShort[short]
			if !ok {
				return parsed, nil, usagef("unknown option -%s", short)
			}
			inline, hasInline := strings.TrimPrefix(arg[2:], "="), len(arg) > 2
			value, consumed, err := flagValue(decl, "-"+short, inline, hasInline, args, i)
			if err != nil {
				return parsed, nil, err
			}
			collect(decl, value)
			i += consumed

		default:
			if mode == parseUntilOperand {
				operands = append(operands, args[i:]...)
				return parsed, operands, nil
			}
			operands = append(operands, arg)
		}
	}
	return parsed, operands, nil
}

// flagValue resolves one flag's value and reports how many extra arguments it
// consumed.
func flagValue(decl Flag, shown, inline string, hasInline bool, args []string, i int) (value string, consumed int, err error) {
	if !decl.takesValue() {
		if hasInline {
			return "", 0, usagef("option %s takes no value", shown)
		}
		return "true", 0, nil
	}
	if hasInline {
		return inline, 0, nil
	}
	if i+1 >= len(args) {
		return "", 0, usagef("option %s requires a value", shown)
	}
	return args[i+1], 1, nil
}
