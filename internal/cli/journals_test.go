package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daily-journal/internal/config"
	"daily-journal/internal/journal"
)

// reload rebuilds the application from the same environment, the way the next
// invocation of the binary would. It is how a test observes what survived the
// process: the recorded active journal, and nothing else.
func (h *harness) reload() {
	h.t.Helper()
	app, err := NewApp(h.env)
	if err != nil {
		h.t.Fatal(err)
	}
	app.Now = h.app.Now
	app.Journal = journal.New(app.Journal.Dir, app.Now)
	app.Printer = h.app.Printer
	app.Theme = h.app.Theme
	h.app = app
}

// root is the journals root of the harness.
func (h *harness) root() string { return h.app.Config.JournalsRoot }

func TestNewCreatesAndActivatesAJournal(t *testing.T) {
	h := newHarness(t, "")

	if err := h.run("new", "reading"); err != nil {
		t.Fatal(err)
	}
	if !journal.HasJournal(h.root(), "reading") {
		t.Fatalf("reading/entries missing under %s", h.root())
	}
	if !strings.Contains(h.out(), "Now using reading") {
		t.Errorf("stdout = %q", h.out())
	}

	// The capture must land in the new journal, not in the default one.
	if err := h.run("Finished Dune"); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(h.root(), "reading", "entries", "2026-07-28.md")
	if _, err := os.Stat(entry); err != nil {
		t.Errorf("capture did not land in the new journal: %v", err)
	}
}

func TestNewRefusesAnExistingJournal(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("new", "work"); err != nil {
		t.Fatal(err)
	}

	err := h.run("new", "work")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want it to say the journal exists", err)
	}
}

func TestNewRejectsANameThatIsNotADirectoryName(t *testing.T) {
	h := newHarness(t, "")
	assertUsageError(t, h.run("new", "../escape"), "new", "../escape")
}

func TestNewNeedsAName(t *testing.T) {
	h := newHarness(t, "")
	assertUsageError(t, h.run("new"), "new")
}

func TestTheActiveJournalSurvivesTheProcess(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("new", "work"); err != nil {
		t.Fatal(err)
	}

	h.reload()
	if got := h.app.Config.ActiveJournal; got != "work" {
		t.Errorf("active journal = %q, want work", got)
	}
	if got := h.app.Config.ActiveFrom; got != config.FromState {
		t.Errorf("active from = %v, want FromState", got)
	}
	if got := h.app.Journal.Dir; got != filepath.Join(h.root(), "work") {
		t.Errorf("journal dir = %q", got)
	}
}

func TestUseSwitchesBackAndForth(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("new", "work"); err != nil {
		t.Fatal(err)
	}

	h.reload()
	if err := h.run("use", config.DefaultJournalName); err != nil {
		t.Fatal(err)
	}
	h.reload()
	if got := h.app.Config.ActiveJournal; got != config.DefaultJournalName {
		t.Errorf("active journal = %q, want %q", got, config.DefaultJournalName)
	}
}

func TestUseRefusesAJournalThatDoesNotExist(t *testing.T) {
	h := newHarness(t, "")

	err := h.run("use", "nope")
	if err == nil {
		t.Fatal("switching to a journal that does not exist should fail")
	}
	if !strings.Contains(err.Error(), "journal new nope") {
		t.Errorf("error should point at 'journal new', got %v", err)
	}
	// A refused switch must not have recorded anything.
	h.reload()
	if got := h.app.Config.ActiveJournal; got != config.DefaultJournalName {
		t.Errorf("active journal = %q, want it unchanged", got)
	}
}

func TestJournalsListsThemAndMarksTheActiveOne(t *testing.T) {
	h := newHarness(t, "")
	for _, name := range []string{"work", "reading"} {
		if err := h.run("new", name); err != nil {
			t.Fatal(err)
		}
	}

	if err := h.run("journals"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	for _, want := range []string{"work", "reading", "* reading"} {
		if !strings.Contains(out, want) {
			t.Errorf("listing missing %q:\n%s", want, out)
		}
	}
	// Only journals that exist are listed: the default one was never written
	// in, so there is nothing on disk to list.
	if strings.Contains(out, config.DefaultJournalName) {
		t.Errorf("listing should not invent a journal that was never created:\n%s", out)
	}
}

func TestJournalsShowsTheActiveJournalBeforeItExists(t *testing.T) {
	h := newHarness(t, "")

	if err := h.run("journals"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	// Nothing has been written yet, so the default journal has no directory.
	// Listing nothing at all would be a lie: it is the journal about to be used.
	if !strings.Contains(out, config.DefaultJournalName) || !strings.Contains(out, "not created yet") {
		t.Errorf("listing should name the active journal and say it is not there yet:\n%s", out)
	}
}

func TestJournalsCountsEntries(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("a first thought"); err != nil {
		t.Fatal(err)
	}

	if err := h.run("journals"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "1 entry") {
		t.Errorf("listing should count the entry:\n%s", h.out())
	}
}

func TestJournalFlagAppliesToOneCommandOnly(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("new", "reading"); err != nil {
		t.Fatal(err)
	}
	if err := h.run("use", config.DefaultJournalName); err != nil {
		t.Fatal(err)
	}

	h.reload()
	if err := h.run("-j", "reading", "Finished Dune"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.root(), "reading", "entries", "2026-07-28.md")); err != nil {
		t.Errorf("--journal did not redirect the capture: %v", err)
	}

	// The next invocation is back on the recorded journal: the flag was for one
	// command, and recorded nothing.
	h.reload()
	if err := h.run("a thought"); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(h.root(), config.DefaultJournalName, "entries", "2026-07-28.md")
	if _, err := os.Stat(entry); err != nil {
		t.Errorf("the flag outlived its command: %v", err)
	}
}

func TestJournalFlagShapes(t *testing.T) {
	for _, args := range [][]string{
		{"-j", "work", "note"},
		{"-jwork", "note"},
		{"-j=work", "note"},
		{"--journal", "work", "note"},
		{"--journal=work", "note"},
		{"add", "--journal", "work", "note"},
	} {
		h := newHarness(t, "")
		if err := h.run("new", "work"); err != nil {
			t.Fatal(err)
		}
		if err := h.run("use", config.DefaultJournalName); err != nil {
			t.Fatal(err)
		}
		h.reload()

		if err := h.run(args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		entry := filepath.Join(h.root(), "work", "entries", "2026-07-28.md")
		if _, err := os.Stat(entry); err != nil {
			t.Errorf("%v did not select the journal: %v", args, err)
		}
	}
}

func TestJournalFlagNeedsAValue(t *testing.T) {
	h := newHarness(t, "")
	assertUsageError(t, h.run("-j"), "-j")
}

func TestJournalFlagIsLiteralAfterTheSeparator(t *testing.T) {
	h := newHarness(t, "")

	if err := h.run("--", "-j", "work"); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(h.root(), config.DefaultJournalName, "entries", "2026-07-28.md")
	content, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "-j work") {
		t.Errorf("the text after -- should have been captured verbatim:\n%s", content)
	}
}

func TestCommandsRefuseAJournalThatDoesNotExist(t *testing.T) {
	h := newHarness(t, "")

	err := h.run("-j", "ghost", "a thought")
	if err == nil {
		t.Fatal("writing to a journal that was never created should fail")
	}
	if !strings.Contains(err.Error(), "journal new ghost") {
		t.Errorf("error should point at 'journal new', got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(h.root(), "ghost")); statErr == nil {
		t.Error("nothing should have been created")
	}
}

func TestTheDefaultJournalIsCreatedOnFirstUse(t *testing.T) {
	h := newHarness(t, "")

	if err := h.run("a first thought"); err != nil {
		t.Fatal(err)
	}
	if !journal.HasJournal(h.root(), config.DefaultJournalName) {
		t.Error("the default journal should be created without being declared")
	}
}

func TestAConfiguredDefaultJournalIsUsed(t *testing.T) {
	h := newHarness(t, "", "DEFAULT_JOURNAL=perso")

	if err := h.run("a first thought"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.root(), "perso", "entries", "2026-07-28.md")); err != nil {
		t.Errorf("DEFAULT_JOURNAL was not followed: %v", err)
	}
}

func TestTheOldLayoutIsMovedIntoTheDefaultJournal(t *testing.T) {
	h := newHarness(t, "")

	// What an earlier version left behind: entries directly under the root.
	oldEntries := filepath.Join(h.root(), "entries")
	if err := os.MkdirAll(oldEntries, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldEntries, "2026-07-27.md"), []byte("# 2026-07-27\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.app.migrateRootJournal(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "Moved") {
		t.Errorf("the move must be reported:\n%s", h.out())
	}
	moved := filepath.Join(h.root(), config.DefaultJournalName, "entries", "2026-07-27.md")
	if _, err := os.Stat(moved); err != nil {
		t.Errorf("the old entry did not follow: %v", err)
	}

	// And the entry is readable from the CLI afterwards.
	h.reload()
	if err := h.run("open", "--last"); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationLeavesTheOldLayoutAloneWhenTheNameIsTaken(t *testing.T) {
	h := newHarness(t, "")
	if err := os.MkdirAll(filepath.Join(h.root(), "entries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(h.root(), config.DefaultJournalName, "entries"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := h.app.migrateRootJournal(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.root(), "entries")); err != nil {
		t.Errorf("the old directory should have been left where it is: %v", err)
	}
	if !strings.Contains(h.out(), "nothing was moved") {
		t.Errorf("the skipped migration must be reported:\n%s", h.out())
	}
}

func TestEveryJournalSharesOneRepository(t *testing.T) {
	h := newHarness(t, `echo "a new thought" >> "$1"`, "AUTOSYNC=true")
	if err := h.run("new", "work"); err != nil {
		t.Fatal(err)
	}
	h.initGit()

	h.reload()
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.gitLog(), "Journal entry for 2026-07-28") {
		t.Errorf("the entry was not committed to the root repository:\n%s", h.gitLog())
	}
}
