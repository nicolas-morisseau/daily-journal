package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StateFileName holds the name of the active journal, one line, no syntax.
//
// It lives apart from the config file on purpose. The config file is written by
// hand and read by the tool; this one is written by the tool ten times a day.
// Keeping them separate means `journal use` never has to rewrite — and so never
// has to reformat — a file the user owns.
const StateFileName = "active"

// stateHome is $XDG_STATE_HOME, defaulting to ~/.local/state as the XDG base
// directory specification says.
func stateHome(env Env) string {
	if xdg := env.get("XDG_STATE_HOME"); xdg != "" {
		return xdg
	}
	return filepath.Join(env.Home, ".local", "state")
}

// StatePaths returns the state directory and the active-journal file.
func StatePaths(env Env) (dir, file string) {
	dir = filepath.Join(stateHome(env), AppName)
	return dir, filepath.Join(dir, StateFileName)
}

// readActive returns the journal name recorded in the state file, or an empty
// string when there is none. An unreadable state file is not an error: the
// resolution order simply falls through to the configured default.
func readActive(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// WriteActive records the active journal.
func WriteActive(path, name string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(name+"\n"), 0o644); err != nil {
		return fmt.Errorf("recording the active journal in %s: %w", path, err)
	}
	return nil
}
