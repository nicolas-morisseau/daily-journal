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
	KeyProjectsDir = "PROJECTS_DIR"
	KeyReviewTags  = "REVIEW_TAGS"
)

// AppName names the config directory under $XDG_CONFIG_HOME.
const AppName = "daily-journal"

// DefaultJournalDirName is the directory created under $HOME when JOURNAL_DIR is
// not set. Set JOURNAL_DIR to put the entries anywhere else.
const DefaultJournalDirName = "daily-journal"

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

	// ProjectsDir holds the git repositories `journal daily` reads the day's
	// commits from, by running git-standup there. Empty leaves the stand-up
	// without its git side, which is not an error: the journal half still works.
	ProjectsDir string

	// Path is the config file location, whether or not it exists.
	Path string
	// Dir is the directory holding Path.
	Dir        string
	ReviewTags []string
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

// Load resolves the configuration, migrating a legacy config file when needed.
//
// Precedence follows the bash script: environment first, then the config file,
// which wins because the script `source`s it after applying env defaults.
func Load(env Env) (*Config, error) {
	dir, file := Paths(env)

	cfg := &Config{
		Dir:         dir,
		Path:        file,
		GitBranch:   DefaultGitBranch,
		Editor:      env.get(KeyEditor),
		JournalDir:  env.get(KeyJournalDir),
		ProjectsDir: env.get(KeyProjectsDir),
		Autosync:    false,
	}

	vars, err := readFile(file, env.get)
	if err != nil {
		return nil, err
	}
	cfg.apply(vars)

	if cfg.JournalDir == "" {
		cfg.JournalDir = defaultJournalDir(env)
	}
	cfg.JournalDir = expandHome(cfg.JournalDir, env.Home)
	cfg.ProjectsDir = expandHome(cfg.ProjectsDir, env.Home)
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
	if v, ok := vars.get(KeyProjectsDir); ok {
		c.ProjectsDir = v
	}
	if v, ok := vars.get(KeyReviewTags); ok {
		c.ReviewTags = strings.Split(v, ",")
		for index, value := range c.ReviewTags {
			c.ReviewTags[index] = strings.TrimSpace(value)
		}
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

// defaultJournalDir is ~/daily-journal, falling back to an existing
// ~/journalot. Nothing is moved.
func defaultJournalDir(env Env) string {
	current := filepath.Join(env.Home, DefaultJournalDirName)
	if _, err := os.Stat(current); err == nil {
		return current
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
