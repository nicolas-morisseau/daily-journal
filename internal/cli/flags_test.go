package cli

import (
	"errors"
	"strings"
	"testing"
)

var testDecls = []Flag{
	{Long: "time", Short: "t", Help: "boolean"},
	{Long: "last", Help: "boolean, no short form"},
	{Long: "format", Short: "f", Arg: "FORMAT", Default: "html", Help: "takes a value"},
}

func TestParseFlagsForms(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		time     bool
		last     bool
		format   string
		operands string
	}{
		{"nothing", nil, false, false, "html", ""},
		{"long boolean", []string{"--time"}, true, false, "html", ""},
		{"short boolean", []string{"-t"}, true, false, "html", ""},
		{"boolean without a short form", []string{"--last"}, false, true, "html", ""},
		{"long with a separate value", []string{"--format", "json"}, false, false, "json", ""},
		{"long with an inline value", []string{"--format=json"}, false, false, "json", ""},
		{"short with a separate value", []string{"-f", "json"}, false, false, "json", ""},
		{"short with an attached value", []string{"-fjson"}, false, false, "json", ""},
		{"short with an inline value", []string{"-f=json"}, false, false, "json", ""},
		{"empty inline value", []string{"--format="}, false, false, "", ""},
		{"operands only", []string{"a", "b"}, false, false, "html", "a b"},
		{"flags before operands", []string{"-t", "a", "b"}, true, false, "html", "a b"},
		{"flags after operands", []string{"a", "b", "-t"}, true, false, "html", "a b"},
		{"flags around operands", []string{"-t", "a", "--format", "pdf", "b"}, true, false, "pdf", "a b"},
		{"separator makes everything an operand", []string{"--", "-t", "--format"}, false, false, "html", "-t --format"},
		{"separator with flags before it", []string{"-t", "--", "-f"}, true, false, "html", "-f"},
		{"a lone dash is an operand", []string{"-"}, false, false, "html", "-"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flags, operands, err := parseFlags(testDecls, c.args, parseAnywhere)
			if err != nil {
				t.Fatalf("parseFlags(%v) failed: %v", c.args, err)
			}
			if flags.Bool("time") != c.time {
				t.Errorf("time = %v, want %v", flags.Bool("time"), c.time)
			}
			if flags.Bool("last") != c.last {
				t.Errorf("last = %v, want %v", flags.Bool("last"), c.last)
			}
			if flags.String("format") != c.format {
				t.Errorf("format = %q, want %q", flags.String("format"), c.format)
			}
			if got := strings.Join(operands, " "); got != c.operands {
				t.Errorf("operands = %q, want %q", got, c.operands)
			}
		})
	}
}

func TestParseFlagsErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown long option", []string{"--nope"}},
		{"unknown short option", []string{"-z"}},
		{"value on a boolean", []string{"--time=yes"}},
		{"missing value at the end", []string{"--format"}},
		{"missing short value at the end", []string{"-f"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := parseFlags(testDecls, c.args, parseAnywhere)
			if err == nil {
				t.Fatalf("parseFlags(%v) should have failed", c.args)
			}
			var usageErr *UsageError
			if !errors.As(err, &usageErr) {
				t.Errorf("error should be a UsageError, got %T", err)
			}
		})
	}
}

// TestParseFlagsUntilOperand covers quick capture: once free text starts, the
// rest is verbatim even when it looks like a flag.
func TestParseFlagsUntilOperand(t *testing.T) {
	decls := []Flag{{Long: "date", Short: "d", Arg: "DATE", Help: "when"}}

	cases := []struct {
		args     []string
		date     string
		operands string
	}{
		{[]string{"a", "note"}, "", "a note"},
		{[]string{"--date", "yesterday", "a", "note"}, "yesterday", "a note"},
		{[]string{"-d", "yesterday", "a", "note"}, "yesterday", "a note"},
		// A note may contain anything once it has started.
		{[]string{"a", "note", "--date", "x"}, "", "a note --date x"},
		{[]string{"-5", "degrees"}, "", ""}, // leading flag-like token is an error, checked below
		{[]string{"--", "-5", "degrees"}, "", "-5 degrees"},
	}

	for _, c := range cases {
		flags, operands, err := parseFlags(decls, c.args, parseUntilOperand)
		if c.operands == "" && err != nil {
			continue // the -5 case: rejected as an unknown option, as intended
		}
		if err != nil {
			t.Errorf("parseFlags(%v) failed: %v", c.args, err)
			continue
		}
		if flags.String("date") != c.date {
			t.Errorf("parseFlags(%v) date = %q, want %q", c.args, flags.String("date"), c.date)
		}
		if got := strings.Join(operands, " "); got != c.operands {
			t.Errorf("parseFlags(%v) operands = %q, want %q", c.args, got, c.operands)
		}
	}

	// Spelled out: a note starting with something flag-shaped needs `--`.
	if _, _, err := parseFlags(decls, []string{"-5", "degrees"}, parseUntilOperand); err == nil {
		t.Error("a leading -5 should be reported as an unknown option")
	}
}

func TestFlagNames(t *testing.T) {
	cases := []struct {
		flag Flag
		want string
	}{
		{Flag{Long: "time", Short: "t"}, "-t, --time"},
		{Flag{Long: "last"}, "    --last"},
		{Flag{Long: "format", Short: "f", Arg: "FORMAT"}, "-f, --format FORMAT"},
		{Flag{Long: "tag", Arg: "TAG"}, "    --tag TAG"},
	}
	for _, c := range cases {
		if got := c.flag.Names(); got != c.want {
			t.Errorf("Names() = %q, want %q", got, c.want)
		}
	}
}

// TestParseFlagsRepeatable covers options that collect several values.
func TestParseFlagsRepeatable(t *testing.T) {
	decls := []Flag{
		{Long: "tag", Arg: "TAG", Repeatable: true},
		{Long: "format", Arg: "FORMAT", Default: "html"},
	}

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"absent", nil, nil},
		{"once", []string{"--tag", "work"}, []string{"work"}},
		{"repeated", []string{"--tag", "work", "--tag", "urgent"}, []string{"work", "urgent"}},
		{"comma separated", []string{"--tag", "work,urgent"}, []string{"work", "urgent"}},
		{"inline value", []string{"--tag=work", "--tag=urgent"}, []string{"work", "urgent"}},
		{"mixed forms", []string{"--tag", "a,b", "--tag=c"}, []string{"a", "b", "c"}},
		{"whitespace and empties are dropped", []string{"--tag", " a , ,b "}, []string{"a", "b"}},
		{"order is preserved", []string{"--tag", "z", "--tag", "a"}, []string{"z", "a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flags, _, err := parseFlags(decls, c.args, parseAnywhere)
			if err != nil {
				t.Fatal(err)
			}
			got := flags.Strings("tag")
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("Strings(tag) = %v, want %v", got, c.want)
			}
		})
	}

	// A non-repeatable option keeps last-one-wins and collects nothing.
	flags, _, err := parseFlags(decls, []string{"--format", "html", "--format", "json"}, parseAnywhere)
	if err != nil {
		t.Fatal(err)
	}
	if flags.String("format") != "json" {
		t.Errorf("String(format) = %q, want json", flags.String("format"))
	}
	if flags.Strings("format") != nil {
		t.Errorf("Strings on a non-repeatable option should be nil, got %v", flags.Strings("format"))
	}
}
