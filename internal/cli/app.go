// Package cli wires configuration, the journal domain and the UI together into
// the `journal` command.
package cli

import (
	"errors"
	"fmt"
	"path/filepath"
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
		Journal: journal.New(journal.DirFor(cfg.JournalsRoot, cfg.ActiveJournal), now),
		Printer: printer,
		Theme:   printer.Theme,
		Now:     now,
	}
	if cfg.MigratedFrom != "" {
		printer.Warn("Migrated config from %s to %s", cfg.MigratedFrom, cfg.Path)
	}
	if err := app.migrateRootJournal(); err != nil {
		return nil, err
	}
	return app, nil
}

// migrateRootJournal moves the single journal of earlier versions into a named
// journal of its own, so every journal has the same shape.
//
// Earlier versions kept entries directly under JOURNAL_DIR; journals now live
// one directory down. The move happens once, and is always reported: entries
// are the user's files and they do not change place silently.
func (a *App) migrateRootJournal() error {
	migration, err := journal.MigrateRootEntries(a.Config.JournalsRoot, a.Config.DefaultJournal)
	if err != nil {
		return err
	}
	switch {
	case migration.Moved():
		a.Printer.Warn("Moved %s to %s", migration.From, migration.To)
		a.Printer.Muted("Your journal is now one of several; run 'journal journals' to see them.")
	case migration.Skipped:
		a.Printer.Warn("%s is still there, but %s already exists, so nothing was moved.",
			migration.From, filepath.Dir(migration.To))
		a.Printer.Muted("Merge the two directories by hand, or move %s under a name of its own.", migration.From)
	}
	return nil
}

// setJournal points the application at another journal.
func (a *App) setJournal(name string, from config.ActiveSource) error {
	if err := journal.ValidateName(name); err != nil {
		return usagef("%s", err.Error())
	}
	a.Config.ActiveJournal, a.Config.ActiveFrom = name, from
	a.Journal = journal.New(journal.DirFor(a.Config.JournalsRoot, name), a.Now)
	return nil
}

// requireJournal rejects a command that would work inside a journal that does
// not exist, rather than letting it create one on the way past.
//
// The default journal is exempt: a first run has to be able to create it, which
// is what makes `journal` work on a fresh install with no setup at all.
func (a *App) requireJournal() error {
	name := a.Config.ActiveJournal
	if name == a.Config.DefaultJournal || journal.HasJournal(a.Config.JournalsRoot, name) {
		return nil
	}
	return fmt.Errorf("no journal named %q (selected by %s)\n\nCreate it with:\n  journal new %s\n\nOr list the journals you have:\n  journal journals",
		name, a.Config.ActiveFrom, name)
}

// repo returns a git handle on the journals root, which is the repository every
// journal shares.
func (a *App) repo() *gitrepo.Repo {
	return gitrepo.New(a.Config.JournalsRoot, a.Config.GitBranch)
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
