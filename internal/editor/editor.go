// Package editor locates and launches the user's text editor.
package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Fallbacks are tried in order when EDITOR is not set, matching the original
// `code > vim > nano` preference.
var Fallbacks = []string{"code", "vim", "nano"}

// ErrNoEditor is returned when nothing usable can be found.
var ErrNoEditor = errors.New(
	"no suitable editor found. Please set the EDITOR environment variable or install code/vim/nano")

// Editor is a resolved editor command.
type Editor struct {
	// Command is the executable name or path.
	Command string
	// Args are the leading arguments from EDITOR, e.g. `code --wait`.
	Args []string
}

// String renders the editor as the user configured it.
func (e Editor) String() string {
	return strings.TrimSpace(e.Command + " " + strings.Join(e.Args, " "))
}

// Resolve picks an editor. preferred is the configured EDITOR value, which may
// include arguments; it is used as-is when non-empty.
func Resolve(preferred string) (Editor, error) {
	if fields := strings.Fields(preferred); len(fields) > 0 {
		return Editor{Command: fields[0], Args: fields[1:]}, nil
	}
	for _, candidate := range Fallbacks {
		if _, err := exec.LookPath(candidate); err == nil {
			return Editor{Command: candidate}, nil
		}
	}
	return Editor{}, ErrNoEditor
}

// Cmd builds the editor invocation for the given files without running it.
//
// Bubble Tea needs the command rather than a blocking call, so it can suspend
// the interface, hand the terminal over and resume afterwards.
func (e Editor) Cmd(paths ...string) *exec.Cmd {
	return exec.Command(e.Command, append(append([]string{}, e.Args...), paths...)...)
}

// Open launches the editor on the given files and waits for it to exit.
func (e Editor) Open(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	cmd := e.Cmd(paths...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running editor %s: %w", e.String(), err)
	}
	return nil
}
