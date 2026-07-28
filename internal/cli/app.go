// Package cli wires configuration, the journal domain and the UI together into
// the `journal` command.
package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"daily-journal/internal/config"
	"daily-journal/internal/editor"
	"daily-journal/internal/gitrepo"
	"daily-journal/internal/journal"
	"daily-journal/internal/ui"
)

// BuildInfo describes the binary, filled in at link time. It is empty for a
// plain `go build`, which reports itself as a development build.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// String renders the build for `journal version`, keeping the commit and date
// out of the way when they are not known.
func (b BuildInfo) String() string {
	version := b.Version
	if version == "" {
		version = "dev"
	}
	details := make([]string, 0, 2)
	if b.Commit != "" && b.Commit != "none" {
		details = append(details, b.Commit)
	}
	if b.Date != "" && b.Date != "unknown" {
		details = append(details, b.Date)
	}
	if len(details) == 0 {
		return version
	}
	return version + " (" + strings.Join(details, ", ") + ")"
}

// App holds everything a command needs. Building it is cheap and side-effect
// free apart from reading the config file.
type App struct {
	Config  *config.Config
	Journal *journal.Journal
	Printer *ui.Printer
	Theme   *ui.Theme
	// Build describes this binary, for `journal version`.
	Build BuildInfo
	// Now supplies the current time, so tests can freeze it.
	Now func() time.Time
}

// NewApp loads the configuration and assembles the application.
func NewApp(env config.Env) (*App, error) {
	cfg, err := config.Load(env)
	if err != nil {
		return nil, err
	}
	now := time.Now
	printer := ui.NewPrinter()

	app := &App{
		Config:  cfg,
		Journal: journal.New(cfg.JournalDir, now),
		Printer: printer,
		Theme:   printer.Theme,
		Now:     now,
	}
	if cfg.MigratedFrom != "" {
		printer.Warn("Migrated config from %s to %s", cfg.MigratedFrom, cfg.Path)
	}
	return app, nil
}

// repo returns a git handle on the journal.
func (a *App) repo() *gitrepo.Repo {
	return gitrepo.New(a.Journal.Dir, a.Config.GitBranch)
}

// resolveEditor picks the editor to launch.
func (a *App) resolveEditor() (editor.Editor, error) {
	return editor.Resolve(a.Config.Editor)
}

// entriesOrHint returns the journal entries, replacing the "no entries
// directory" error with the friendly first-run hint.
func (a *App) entriesOrHint(order journal.SortOrder) ([]journal.Entry, error) {
	entries, err := a.Journal.EntriesSorted(order)
	if err != nil {
		if errors.Is(err, journal.ErrNoEntriesDir) {
			return nil, fmt.Errorf("no entries directory found at %s\n\nTo create your first entry, run:\n  journal", a.Journal.EntriesDir())
		}
		return nil, err
	}
	return entries, nil
}
