package config

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// shellVars holds the assignments found in a shell-style config file, in the
// order they appeared. The original bash implementation `source`s its config
// file, so we have to understand the subset of shell syntax people write there:
// comments, `export`, single/double quotes and variable expansion.
type shellVars struct {
	order  []string
	values map[string]string
}

func newShellVars() *shellVars {
	return &shellVars{values: map[string]string{}}
}

func (v *shellVars) set(key, value string) {
	if _, seen := v.values[key]; !seen {
		v.order = append(v.order, key)
	}
	v.values[key] = value
}

// get returns the value and whether the key was assigned at all. An empty
// assignment (`FOO=`) counts as absent, mirroring bash's `${FOO:-default}`.
func (v *shellVars) get(key string) (string, bool) {
	value, ok := v.values[key]
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

// lookupFunc resolves a variable name during expansion.
type lookupFunc func(string) string

// parseShellVars reads `KEY=VALUE` assignments. Anything it does not
// understand (function definitions, conditionals, commands) is skipped rather
// than treated as an error: a config file that bash tolerates must not break
// the CLI.
func parseShellVars(r io.Reader, lookup lookupFunc) *shellVars {
	vars := newShellVars()
	if lookup == nil {
		lookup = os.Getenv
	}
	// Values assigned earlier in the file are visible to later expansions,
	// exactly as bash would see them.
	resolve := func(name string) string {
		if value, ok := vars.values[name]; ok {
			return value
		}
		return lookup(name)
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		key, raw, ok := splitAssignment(scanner.Text())
		if !ok {
			continue
		}
		vars.set(key, unquote(raw, resolve))
	}
	// A malformed or oversized config file degrades to "whatever we could
	// read", which is friendlier than refusing to open today's entry.
	_ = scanner.Err()
	return vars
}

// splitAssignment isolates the key and the raw (still quoted) value of a shell
// assignment line, or reports that the line is not an assignment.
func splitAssignment(line string) (key, rawValue string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimSpace(line)

	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	key = line[:eq]
	if !isIdentifier(key) {
		return "", "", false
	}
	return key, strings.TrimSpace(line[eq+1:]), true
}

func isIdentifier(s string) bool {
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return s != ""
}

// unquote turns a raw shell value into its literal form: single quotes are
// taken verbatim, double quotes and bare words get variable expansion, and an
// unquoted trailing `# comment` is dropped.
func unquote(raw string, resolve lookupFunc) string {
	if raw == "" {
		return ""
	}

	var out strings.Builder
	var quote byte // 0, '\'' or '"'

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote != 0 && c == quote:
			quote = 0
		case quote == '\'':
			out.WriteByte(c)
		case c == '\\' && quote == '"' && i+1 < len(raw):
			i++
			out.WriteByte(raw[i])
		case c == '$' && quote != '\'':
			name, width := readVarName(raw[i+1:])
			if width == 0 {
				out.WriteByte(c)
				continue
			}
			out.WriteString(resolve(name))
			i += width
		case quote == 0 && (c == ' ' || c == '\t' || c == '#'):
			// End of the bare word: the rest is another word or a comment.
			return strings.TrimSpace(out.String())
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// readVarName reads a `$NAME` or `${NAME}` reference, returning the name and
// how many bytes of input it consumed after the `$`.
func readVarName(s string) (name string, width int) {
	if s == "" {
		return "", 0
	}
	if s[0] == '{' {
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return "", 0
		}
		if !isIdentifier(s[1:end]) {
			return "", 0
		}
		return s[1:end], end + 1
	}
	end := 0
	for end < len(s) && isIdentifier(s[:end+1]) {
		end++
	}
	if end == 0 {
		return "", 0
	}
	return s[:end], end
}
