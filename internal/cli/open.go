package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"daily-journal/internal/dateparse"
	"daily-journal/internal/gitrepo"
	"daily-journal/internal/journal"
)

// resolveDate turns a date expression into a YYYY-MM-DD slug. An empty
// expression means today; useLast follows the most recent entry instead.
//
// A date that cannot be understood is a usage error, not a runtime one, so the
// exit status tells a script it mistyped rather than that the journal is empty.
func (a *App) resolveDate(expr string, useLast bool) (string, error) {
	if useLast {
		last, err := a.Journal.Last()
		if err != nil {
			if errors.Is(err, journal.ErrNoEntriesDir) || errors.Is(err, journal.ErrNoEntries) {
				return "", fmt.Errorf("no journal entries found\n\nTo create your first entry, run:\n  journal")
			}
			return "", err
		}
		return last.Slug, nil
	}
	if expr == "" {
		return a.Now().Format(dateparse.Layout), nil
	}
	date, err := dateparse.ParseToString(expr, a.Now())
	if err != nil {
		return "", usagef("%s", err.Error())
	}
	return date, nil
}

// quickCapture appends a timestamped line to a day's entry without opening an
// editor. It backs both `journal add` and the bare `journal "a thought"` form.
func (a *App) quickCapture(dateExpr, text string) error {
	date, err := a.resolveDate(dateExpr, false)
	if err != nil {
		return err
	}
	if _, err := a.Journal.QuickCapture(date, text); err != nil {
		return err
	}
	a.Printer.Success("Added to %s.md", date)
	return nil
}

// openEntry makes sure the entry exists, opens it in the editor, then offers to
// commit what changed. It backs `journal open` and the bare `journal` form.
func (a *App) openEntry(dateExpr string, useLast bool) error {
	if err := gitrepo.CheckAvailable(); err != nil {
		return err
	}
	date, err := a.resolveDate(dateExpr, useLast)
	if err != nil {
		return err
	}

	ed, err := a.resolveEditor()
	if err != nil {
		return err
	}

	if err := a.Journal.EnsureDirs(); err != nil {
		return fmt.Errorf("cannot create journal directory: %s\n\n"+
			"Check that you have write permissions to the parent directory.\n"+
			"You can also set a custom location with:\n  export JOURNAL_DIR=\"/path/to/your/journal\"",
			a.Journal.Dir)
	}

	repo := a.repo()
	useGit := true
	if !repo.IsRepo() {
		useGit = a.initRepo(repo)
	}
	if useGit && repo.HasRemote() {
		a.pullBeforeWriting(repo)
	}

	path, created, err := a.Journal.CreateEntry(journal.NewEntryOptions{Date: date})
	if err != nil {
		return err
	}
	name := journal.Entry{Path: path}.FileName()
	if created {
		a.Printer.Success("Created new entry: %s", name)
	} else {
		a.Printer.Muted("Opening existing entry: %s", name)
	}

	before := fileDigest(path)
	if err := ed.Open(path); err != nil {
		return err
	}
	after := fileDigest(path)

	if !useGit {
		return nil
	}
	if before == after {
		a.Printer.Muted("No changes detected.")
		return nil
	}
	a.Printer.Blank()
	return a.saveChanges(repo, path, date)
}

// initRepo offers to create a git repository, reporting whether git can be used
// afterwards.
func (a *App) initRepo(repo *gitrepo.Repo) bool {
	a.Printer.Warn("Git repository not initialized in %s", repo.Dir)
	if !a.Printer.Confirm("Would you like to initialize it now?", false) {
		a.Printer.Warn("Continuing without git version control…")
		return false
	}
	if err := repo.Init(); err != nil {
		a.Printer.Error(err)
		a.Printer.Warn("Continuing without git version control…")
		return false
	}
	a.Printer.Success("Git repository initialized.")
	a.Printer.Warn("Don't forget to add a remote: git remote add origin <your-repo-url>")
	return true
}

// pullBeforeWriting pulls from origin so the editor opens the latest content. A
// failure is not fatal: writing locally matters more than being in sync.
func (a *App) pullBeforeWriting(repo *gitrepo.Repo) {
	if err := repo.PullRebase(); err != nil {
		a.Printer.Warn("Failed to pull from remote. Continuing with local changes…")
	}
}

// saveChanges commits the edited entry. AUTOSYNC commits and pushes without
// asking; otherwise the user is prompted for each step.
func (a *App) saveChanges(repo *gitrepo.Repo, path, date string) error {
	message := "Journal entry for " + date

	if a.Config.Autosync {
		if err := commit(repo, path, message); err != nil {
			return err
		}
		if !repo.HasRemote() {
			a.Printer.Success("Changes auto-committed locally.")
			return nil
		}
		if err := repo.Push(); err != nil {
			a.Printer.Warn("Failed to push. Run 'git push' manually when ready.")
			return nil
		}
		a.Printer.Success("Changes auto-committed and pushed.")
		return nil
	}

	if !a.Printer.Confirm("Commit changes?", false) {
		return nil
	}
	if err := commit(repo, path, message); err != nil {
		return err
	}
	if !repo.HasRemote() {
		a.Printer.Success("Changes committed locally.")
		return nil
	}
	if !a.Printer.Confirm("Push to remote?", false) {
		a.Printer.Success("Changes committed locally.")
		return nil
	}
	if err := repo.Push(); err != nil {
		a.Printer.Warn("Failed to push. Run 'git push' manually when ready.")
		return nil
	}
	a.Printer.Success("Changes committed and pushed.")
	return nil
}

func commit(repo *gitrepo.Repo, path, message string) error {
	if err := repo.Add(path); err != nil {
		return err
	}
	return repo.Commit(message)
}

// fileDigest hashes a file so we can tell whether the editor changed it.
// Unreadable files hash to the empty string.
func fileDigest(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
