package config

import (
	"strings"
	"testing"
)

func TestParseShellVars(t *testing.T) {
	lookup := func(key string) string {
		return map[string]string{"HOME": "/home/nm", "OTHER": "x"}[key]
	}

	input := `# a comment
   # indented comment

export JOURNAL_DIR="$HOME/journalot"
QUOTED='literal $HOME'
BRACED="${HOME}/braced"
BARE=value
BARE_COMMENT=value   # trailing comment
EMPTY=
SPACED = notanassignment
ESCAPED="a\"b"
if [ -f x ]; then
  echo skipped
fi
9BAD=nope
CONCAT="$HOME:$OTHER"
LATER=$JOURNAL_DIR/entries
`
	vars := parseShellVars(strings.NewReader(input), lookup)

	cases := []struct{ key, want string }{
		{"JOURNAL_DIR", "/home/nm/journalot"},
		{"QUOTED", "literal $HOME"},
		{"BRACED", "/home/nm/braced"},
		{"BARE", "value"},
		{"BARE_COMMENT", "value"},
		{"ESCAPED", `a"b`},
		{"CONCAT", "/home/nm:x"},
		{"LATER", "/home/nm/journalot/entries"},
	}
	for _, c := range cases {
		got, ok := vars.get(c.key)
		if !ok {
			t.Errorf("%s missing", c.key)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.key, got, c.want)
		}
	}

	// An empty assignment counts as absent, like bash's ${FOO:-default}.
	if _, ok := vars.get("EMPTY"); ok {
		t.Error("EMPTY should be treated as unset")
	}
	for _, key := range []string{"SPACED", "9BAD", "echo", "fi"} {
		if _, ok := vars.get(key); ok {
			t.Errorf("%s should not have been parsed as an assignment", key)
		}
	}
}

func TestSplitAssignment(t *testing.T) {
	cases := []struct {
		line    string
		key     string
		value   string
		isAssig bool
	}{
		{"FOO=bar", "FOO", "bar", true},
		{"  export FOO=bar  ", "FOO", "bar", true},
		{"# FOO=bar", "", "", false},
		{"", "", "", false},
		{"=bar", "", "", false},
		{"FOO", "", "", false},
		{"FOO-BAR=x", "", "", false},
	}
	for _, c := range cases {
		key, value, ok := splitAssignment(c.line)
		if ok != c.isAssig || key != c.key || value != c.value {
			t.Errorf("splitAssignment(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.line, key, value, ok, c.key, c.value, c.isAssig)
		}
	}
}
