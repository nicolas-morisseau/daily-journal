// Package config resolves daily-journal's runtime configuration. It reads the
// very same shell-style file as the original journalot script
// (`$XDG_CONFIG_HOME/journalot/config`), including the legacy `~/.journalotrc`
// migration, so this binary can be dropped in place of the script.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Keys recognised in the config file. Anything else is ignored, which keeps
// older or hand-edited config files working.
const (
	KeyJournalDir  = "JOURNAL_DIR"
	KeyGitBranch   = "GIT_BRANCH"
	KeyAutosync    = "AUTOSYNC"
	KeyEditor      = "EDITOR"
	KeySprintStart = "SPRINT_START"
)

// AppName names the config directory under $XDG_CONFIG_HOME.
const AppName = "daily-journal"

// DefaultJournalDirName is the directory created under $HOME when JOURNAL_DIR is
// not set. Set JOURNAL_DIR to put the entries anywhere else.
const DefaultJournalDirName = "daily-journal"

// LegacyJournalDirName is the default journal directory of earlier versions,
// still used when it exists and DefaultJournalDirName does not.
const LegacyJournalDirName = "journalot"

// DefaultGitBranch matches the bash default.
const DefaultGitBranch = "main"

// Config is the resolved configuration for one invocation.
type Config struct {
	// JournalDir is the directory holding the journal.
	JournalDir string

	GitBranch string
	Autosync  bool
	// Editor is EDITOR as seen by the script: the environment variable, or an
	// override from the config file.
	Editor string

	// SprintStart is the first day of any one sprint, as written in the config
	// file. `journal review` counts its fixed windows from it; empty leaves the
	// window rolling with today. It is kept unparsed so that a typo fails the one
	// command that reads it rather than every command.
	SprintStart string

	// Path is the config file location, whether or not it exists.
	Path string
	// Dir is the directory holding Path.
	Dir string
	// MigratedFrom is the older config location moved into place during this
	// run, empty when nothing was migrated. The caller reports it so a rename
	// never happens silently.
	MigratedFrom string
}

// Env abstracts the process environment so configuration is testable.
type Env struct {
	Home   string
	Getenv func(string) string
}

// OSEnv returns an Env backed by the real process environment.
func OSEnv() Env {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return Env{Home: home, Getenv: os.Getenv}
}

func (e Env) get(key string) string {
	if e.Getenv == nil {
		return ""
	}
	return e.Getenv(key)
}

// configHome is $XDG_CONFIG_HOME, defaulting to ~/.config.
func configHome(env Env) string {
	if xdg := env.get("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	return filepath.Join(env.Home, ".config")
}

// Paths returns the config directory and file for an environment.
func Paths(env Env) (dir, file string) {
	dir = filepath.Join(configHome(env), AppName)
	return dir, filepath.Join(dir, "config")
}

// legacyPaths lists the config locations used by earlier versions, newest first.
// The first one that exists is migrated to the current location.
func legacyPaths(env Env) []string {
	return []string{
		// Before the tool was renamed to daily-journal.
		filepath.Join(configHome(env), "journalot", "config"),
		// The original bash script, before XDG.
		filepath.Join(env.Home, ".journalotrc"),
	}
}

// Load resolves the configuration, migrating a legacy config file when needed.
//
// Precedence follows the bash script: environment first, then the config file,
// which wins because the script `source`s it after applying env defaults.
func Load(env Env) (*Config, error) {
	dir, file := Paths(env)

	cfg := &Config{
		Dir:          dir,
		Path:         file,
		GitBranch:    DefaultGitBranch,
		Editor:       env.get(KeyEditor),
		JournalDir:   env.get(KeyJournalDir),
		MigratedFrom: "",
		Autosync:     false,
	}

	migratedFrom, err := migrateLegacy(env, dir, file)
	if err != nil {
		return nil, err
	}
	cfg.MigratedFrom = migratedFrom

	vars, err := readFile(file, env.get)
	if err != nil {
		return nil, err
	}
	cfg.apply(vars)

	if cfg.JournalDir == "" {
		cfg.JournalDir = defaultJournalDir(env)
	}
	cfg.JournalDir = expandHome(cfg.JournalDir, env.Home)
	return cfg, nil
}

func (c *Config) apply(vars *shellVars) {
	if vars == nil {
		return
	}
	if v, ok := vars.get(KeyJournalDir); ok {
		c.JournalDir = v
	}
	if v, ok := vars.get(KeyGitBranch); ok {
		c.GitBranch = v
	}
	if v, ok := vars.get(KeyEditor); ok {
		c.Editor = v
	}
	if v, ok := vars.get(KeyAutosync); ok {
		c.Autosync = v == "true"
	}
	if v, ok := vars.get(KeySprintStart); ok {
		c.SprintStart = v
	}
}

func readFile(path string, lookup lookupFunc) (*shellVars, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newShellVars(), nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	defer f.Close()
	return parseShellVars(f, lookup), nil
}

// migrateLegacy moves a config file left by an earlier version into the current
// location, and returns where it came from. An existing current config always
// wins, so nothing is ever overwritten.
func migrateLegacy(env Env, dir, file string) (from string, err error) {
	if _, err := os.Stat(file); err == nil {
		return "", nil
	}
	for _, old := range legacyPaths(env) {
		if _, err := os.Stat(old); err != nil {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("creating config dir %s: %w", dir, err)
		}
		if err := os.Rename(old, file); err != nil {
			return "", fmt.Errorf("migrating %s to %s: %w", old, file, err)
		}
		// Tidy up the directory we just emptied. Remove only succeeds when it
		// is empty, so anything else living there is left alone.
		_ = os.Remove(filepath.Dir(old))
		return old, nil
	}
	return "", nil
}

// defaultJournalDir is ~/daily-journal, falling back to an existing
// ~/journalot. Nothing is moved.
func defaultJournalDir(env Env) string {
	current := filepath.Join(env.Home, DefaultJournalDirName)
	if _, err := os.Stat(current); err == nil {
		return current
	}
	legacy := filepath.Join(env.Home, LegacyJournalDirName)
	if info, err := os.Stat(legacy); err == nil && info.IsDir() {
		return legacy
	}
	return current
}

// expandHome turns a leading `~` into an absolute path.
func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
