package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Printer writes daily-journal's non-interactive output. Keeping the streams in a
// struct means tests can capture everything, and it gives one place to apply
// the theme.
type Printer struct {
	Out   io.Writer
	Err   io.Writer
	In    io.Reader
	Theme *Theme
}

// NewPrinter returns a Printer bound to the process streams.
func NewPrinter() *Printer {
	return &Printer{Out: os.Stdout, Err: os.Stderr, In: os.Stdin, Theme: Default}
}

// Printf writes unstyled formatted output.
func (p *Printer) Printf(format string, args ...any) {
	fmt.Fprintf(p.Out, format, args...)
}

// Println writes an unstyled line.
func (p *Printer) Println(args ...any) {
	fmt.Fprintln(p.Out, args...)
}

// Blank writes an empty line.
func (p *Printer) Blank() { fmt.Fprintln(p.Out) }

// Heading writes a section heading followed by a blank line.
func (p *Printer) Heading(text string) {
	fmt.Fprintln(p.Out, p.Theme.Title.Render(text))
	fmt.Fprintln(p.Out)
}

// Success writes a positive status line.
func (p *Printer) Success(format string, args ...any) {
	fmt.Fprintln(p.Out, p.Theme.Success.Render(fmt.Sprintf(format, args...)))
}

// Warn writes a warning line.
func (p *Printer) Warn(format string, args ...any) {
	fmt.Fprintln(p.Out, p.Theme.Warning.Render(fmt.Sprintf(format, args...)))
}

// Info writes an informational line in the accent colour.
func (p *Printer) Info(format string, args ...any) {
	fmt.Fprintln(p.Out, p.Theme.Accent.Render(fmt.Sprintf(format, args...)))
}

// Muted writes a secondary line.
func (p *Printer) Muted(format string, args ...any) {
	fmt.Fprintln(p.Out, p.Theme.Muted.Render(fmt.Sprintf(format, args...)))
}

// Error writes an error to stderr, prefixed like the original CLI.
func (p *Printer) Error(err error) {
	if err == nil {
		return
	}
	lines := strings.Split(err.Error(), "\n")
	fmt.Fprintln(p.Err, p.Theme.Danger.Render("Error: "+lines[0]))
	for _, line := range lines[1:] {
		fmt.Fprintln(p.Err, line)
	}
}

// Field writes an aligned label/value pair, e.g. "  Total entries   42".
func (p *Printer) Field(label, value string, labelWidth int) {
	padded := label
	if len(label) < labelWidth {
		padded += strings.Repeat(" ", labelWidth-len(label))
	}
	fmt.Fprintf(p.Out, "  %s  %s\n", p.Theme.Label.Render(padded), p.Theme.Value.Render(value))
}

// Confirm asks a yes/no question and reports the answer. Anything other than
// y/Y is a no, matching the original prompts. When stdin is not interactive it
// returns fallback without asking.
func (p *Printer) Confirm(question string, fallback bool) bool {
	if !Interactive() {
		return fallback
	}
	fmt.Fprintf(p.Out, "%s %s ", p.Theme.Accent.Render("?"), question+p.Theme.Muted.Render(" (y/n)"))
	answer, ok := p.readLine()
	if !ok {
		return fallback
	}
	answer = strings.TrimSpace(answer)
	return answer == "y" || answer == "Y"
}

func (p *Printer) readLine() (string, bool) {
	reader, ok := p.In.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(p.In)
		p.In = reader
	}
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return line, true
}

// Indent prefixes every non-empty line of text with two spaces.
func Indent(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}
