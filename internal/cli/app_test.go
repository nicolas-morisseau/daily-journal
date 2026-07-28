package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daily-journal/internal/config"
	"daily-journal/internal/journal"
	"daily-journal/internal/ui"
)

// harness runs the CLI against a throwaway HOME with a frozen clock and a
// scripted editor, capturing everything the command prints.
type harness struct {
	t      *testing.T
	home   string
	app    *App
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// newHarness builds an App rooted at a temporary home. editorScript is the body
// of a shell script invoked as the editor; an empty script leaves files
// untouched.
func newHarness(t *testing.T, editorScript string, configLines ...string) *harness {
	t.Helper()
	home := t.TempDir()

	editorPath := filepath.Join(home, "fake-editor")
	body := "#!/bin/sh\n" + editorScript + "\n"
	if err := os.WriteFile(editorPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	env := config.Env{Home: home, Getenv: func(key string) string {
		switch key {
		case "HOME":
			return home
		case "EDITOR":
			return editorPath
		default:
			return ""
		}
	}}

	if len(configLines) > 0 {
		dir, file := config.Paths(env)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := strings.Join(configLines, "\n") + "\n"
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	app, err := NewApp(env)
	if err != nil {
		t.Fatal(err)
	}

	h := &harness{t: t, home: home, app: app, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	app.Now = func() time.Time { return time.Date(2026, 7, 28, 14, 30, 0, 0, time.UTC) }
	app.Journal = journal.New(app.Config.JournalDir, app.Now)
	app.Printer = &ui.Printer{Out: h.stdout, Err: h.stderr, In: strings.NewReader(""), Theme: ui.Default}
	app.Theme = ui.Default
	return h
}

// run executes the CLI with the given arguments.
func (h *harness) run(args ...string) error {
	h.t.Helper()
	h.stdout.Reset()
	h.stderr.Reset()
	return h.app.Run(args)
}

// out returns everything written to stdout by the last run.
func (h *harness) out() string { return h.stdout.String() }

// journalDir is the journal's directory.
func (h *harness) journalDir() string { return h.app.Journal.Dir }

// entry reads an entry's content.
func (h *harness) entry(slug string) string {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.journalDir(), "entries", slug+".md"))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

// initGit turns the journal directory into a committed git repository.
func (h *harness) initGit() {
	h.t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		h.t.Skip("git is not installed")
	}
	if err := os.MkdirAll(filepath.Join(h.journalDir(), "entries"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = h.journalDir()
		if out, err := cmd.CombinedOutput(); err != nil {
			h.t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitLog returns the repository's commit subjects, newest first.
func (h *harness) gitLog() string {
	h.t.Helper()
	cmd := exec.Command("git", "log", "--format=%s")
	cmd.Dir = h.journalDir()
	out, err := cmd.Output()
	if err != nil {
		h.t.Fatalf("git log: %v", err)
	}
	return string(out)
}

// assertUsageError fails unless err is a usage error, which is what earns exit
// status 2.
func assertUsageError(t *testing.T, err error, args ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("journal %s should have failed", strings.Join(args, " "))
	}
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Errorf("journal %s: error should be a UsageError, got %T: %v", strings.Join(args, " "), err, err)
	}
}

// --- resolution: quick capture next to subcommands ---------------------------

func TestBareTextIsQuickCapture(t *testing.T) {
	h := newHarness(t, `echo "the editor should not run" >> "$1"`)

	if err := h.run("A passing thought"); err != nil {
		t.Fatal(err)
	}
	content := h.entry("2026-07-28")
	if !strings.Contains(content, "**14:30** - A passing thought") {
		t.Errorf("capture missing:\n%s", content)
	}
	if strings.Contains(content, "the editor should not run") {
		t.Error("quick capture must not open the editor")
	}
	if !strings.Contains(h.out(), "Added to 2026-07-28.md") {
		t.Errorf("stdout = %q", h.out())
	}
}

// TestUnquotedTextIsJoined covers the bug the flag-based CLI had: an unquoted
// multi-word note used to keep only its last word.
func TestUnquotedTextIsJoined(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("several", "words", "without", "quotes"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.entry("2026-07-28"), "**14:30** - several words without quotes") {
		t.Errorf("the whole sentence should be captured:\n%s", h.entry("2026-07-28"))
	}
}

// TestQuotedTextNeverLooksLikeACommand is the safety property of the resolution
// rule: a quoted note is one argument, so it cannot equal a command name.
func TestQuotedTextNeverLooksLikeACommand(t *testing.T) {
	h := newHarness(t, "")
	for _, text := range []string{
		"list of things to buy",
		"search for a new flat",
		"week was rough, need a break",
		"open questions from the review",
	} {
		if err := h.run(text); err != nil {
			t.Errorf("journal %q failed: %v", text, err)
		}
	}
	content := h.entry("2026-07-28")
	for _, text := range []string{"list of things", "search for a new flat", "week was rough"} {
		if !strings.Contains(content, text) {
			t.Errorf("missing %q:\n%s", text, content)
		}
	}
}

// TestBareCommandWordRunsTheCommand is the flip side: a single bare word that is
// a command name runs the command, and the escape hatches are add and --.
func TestBareCommandWordRunsTheCommand(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run(); err != nil {
		t.Fatal(err)
	}

	// `journal week of the year` unquoted fails loudly rather than guessing.
	assertUsageError(t, h.run("week", "of", "the", "year"), "week", "of", "the", "year")

	// Both escape hatches capture the text instead.
	if err := h.run("add", "week", "of", "the", "year"); err != nil {
		t.Fatal(err)
	}
	if err := h.run("--", "search", "for", "a", "flat"); err != nil {
		t.Fatal(err)
	}
	content := h.entry("2026-07-28")
	for _, want := range []string{"week of the year", "search for a flat"} {
		if !strings.Contains(content, want) {
			t.Errorf("missing %q:\n%s", want, content)
		}
	}
}

func TestNoArgumentsOpensToday(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if got := h.entry("2026-07-28"); got != "# 2026-07-28\n\n" {
		t.Errorf("a new entry should hold just the date heading, got %q", got)
	}
	if !strings.Contains(h.out(), "Created new entry: 2026-07-28.md") {
		t.Errorf("stdout = %q", h.out())
	}
}

// TestLeadingFlagBelongsToOpen keeps `journal --last` working without naming a
// subcommand: a leading flag falls through to the default command.
func TestLeadingFlagBelongsToOpen(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("open", "2026-07-20"); err != nil {
		t.Fatal(err)
	}
	if err := h.run("--last"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "Opening existing entry: 2026-07-20.md") {
		t.Errorf("stdout = %q", h.out())
	}
}

// TestRemovedCommandsAreNoLongerCommands: words that used to be commands are
// now ordinary text, while the options that went with them are rejected.
func TestRemovedCommandsAreNoLongerCommands(t *testing.T) {
	h := newHarness(t, "")
	// A retired command name is just a one-word note now.
	for _, word := range []string{"list", "stats", "export"} {
		if err := h.run(word); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(h.entry("2026-07-28"), "- "+word) {
			t.Errorf("%q should be captured as text now:\n%s", word, h.entry("2026-07-28"))
		}
	}
	// Options only exist on the command that declares them.
	for _, args := range [][]string{
		{"open", "-t"}, {"open", "--time"},
		{"open", "--tag", "work"}, {"week", "--format", "json"},
	} {
		assertUsageError(t, h.run(args...), args...)
	}
}

func TestUnknownCommandIsCapturedAsText(t *testing.T) {
	h := newHarness(t, "")
	// "listt" is not a command, so it is a note.
	if err := h.run("listt"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.entry("2026-07-28"), "- listt") {
		t.Errorf("stray word should have been captured:\n%s", h.entry("2026-07-28"))
	}
}

// --- open --------------------------------------------------------------------

func TestOpenAcceptsDateExpressions(t *testing.T) {
	h := newHarness(t, "")
	cases := map[string]string{
		"":                     "2026-07-28",
		"yesterday":            "2026-07-27",
		"2025-01-15":           "2025-01-15",
		"last friday":          "2026-07-24",
		"3 days ago":           "2026-07-25",
		"multi word unhandled": "",
	}
	for expr, want := range cases {
		var args []string
		if expr == "" {
			args = []string{"open"}
		} else {
			args = append([]string{"open"}, strings.Fields(expr)...)
		}
		err := h.run(args...)
		if want == "" {
			assertUsageError(t, err, args...)
			continue
		}
		if err != nil {
			t.Errorf("journal %v failed: %v", args, err)
			continue
		}
		if _, err := os.Stat(filepath.Join(h.journalDir(), "entries", want+".md")); err != nil {
			t.Errorf("journal %v did not create %s.md: %v", args, want, err)
		}
	}
}

func TestOpenLast(t *testing.T) {
	h := newHarness(t, "")
	for _, date := range []string{"2026-07-20", "2026-07-22"} {
		if err := h.run("open", date); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.run("open", "--last"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "Opening existing entry: 2026-07-22.md") {
		t.Errorf("stdout = %q", h.out())
	}
}

func TestOpenLastWithoutEntriesFails(t *testing.T) {
	h := newHarness(t, "")
	err := h.run("open", "--last")
	if err == nil {
		t.Fatal("--last should fail on an empty journal")
	}
	if !strings.Contains(err.Error(), "no journal entries found") {
		t.Errorf("error = %v", err)
	}
}

func TestOpenLastRejectsADate(t *testing.T) {
	h := newHarness(t, "")
	assertUsageError(t, h.run("open", "--last", "yesterday"), "open", "--last", "yesterday")
}

func TestOpenUsesConfiguredJournalDir(t *testing.T) {
	h := newHarness(t, "", `JOURNAL_DIR="$HOME/notes"`)
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(h.home, "notes")
	if h.journalDir() != want {
		t.Errorf("journal dir = %q, want %q", h.journalDir(), want)
	}
	if _, err := os.Stat(filepath.Join(want, "entries", "2026-07-28.md")); err != nil {
		t.Errorf("entry not created in the configured directory: %v", err)
	}
}

// --- add ---------------------------------------------------------------------

func TestAddToAnotherDay(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("add", "--date", "yesterday", "a", "late", "note"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.entry("2026-07-27"), "a late note") {
		t.Errorf("capture should land on yesterday:\n%s", h.entry("2026-07-27"))
	}
}

func TestAddWithoutTextFails(t *testing.T) {
	h := newHarness(t, "")
	assertUsageError(t, h.run("add"), "add")
}

// TestAddKeepsFlagLikeTextAfterTheFirstWord checks the free-text parse mode.
func TestAddKeepsFlagLikeTextAfterTheFirstWord(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("add", "down", "--5", "degrees", "today"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.entry("2026-07-28"), "down --5 degrees today") {
		t.Errorf("text after the first word must be verbatim:\n%s", h.entry("2026-07-28"))
	}
}

// --- other commands ----------------------------------------------------------

// TestSearchTagFilter covers --tag end to end, including several tags at once.
func TestSearchTagFilter(t *testing.T) {
	h := newHarness(t, "")
	writeEntry(t, h, "2026-07-25", "# 2026-07-25\n\nroadmap talk, untagged\n")
	writeEntry(t, h, "2026-07-26", "# 2026-07-26\n\nroadmap review [#work]\n")
	writeEntry(t, h, "2026-07-27", "# 2026-07-27\n\nroadmap #work #urgent\n")

	cases := []struct {
		name    string
		args    []string
		present []string
		absent  []string
	}{
		{
			name:    "no tag searches everything",
			args:    []string{"search", "roadmap"},
			present: []string{"2026-07-25", "2026-07-26", "2026-07-27"},
		},
		{
			name:    "one tag narrows it",
			args:    []string{"search", "--tag", "work", "roadmap"},
			present: []string{"2026-07-26", "2026-07-27"},
			absent:  []string{"2026-07-25"},
		},
		{
			name:    "repeating the flag requires both tags",
			args:    []string{"search", "--tag", "work", "--tag", "urgent", "roadmap"},
			present: []string{"2026-07-27"},
			absent:  []string{"2026-07-25", "2026-07-26"},
		},
		{
			name:    "a comma-separated list means the same",
			args:    []string{"search", "--tag", "work,urgent", "roadmap"},
			present: []string{"2026-07-27"},
			absent:  []string{"2026-07-25", "2026-07-26"},
		},
		{
			name:    "the flag may follow the query",
			args:    []string{"search", "roadmap", "--tag", "urgent"},
			present: []string{"2026-07-27"},
			absent:  []string{"2026-07-26"},
		},
		{
			name:    "a leading # is accepted",
			args:    []string{"search", "--tag", "#urgent", "roadmap"},
			present: []string{"2026-07-27"},
			absent:  []string{"2026-07-26"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := h.run(c.args...); err != nil {
				t.Fatal(err)
			}
			out := h.out()
			for _, want := range c.present {
				if !strings.Contains(out, want) {
					t.Errorf("missing %s:\n%s", want, out)
				}
			}
			for _, unwanted := range c.absent {
				if strings.Contains(out, unwanted) {
					t.Errorf("%s should have been filtered out:\n%s", unwanted, out)
				}
			}
		})
	}
}

// TestSearchTagFilterReportsWhatItLookedAt keeps the summary line honest.
func TestSearchTagFilterReportsWhatItLookedAt(t *testing.T) {
	h := newHarness(t, "")
	writeEntry(t, h, "2026-07-26", "# 2026-07-26\n\nroadmap [#work]\n")
	writeEntry(t, h, "2026-07-27", "# 2026-07-27\n\nsomething else [#work]\n")

	if err := h.run("search", "--tag", "work", "roadmap"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "Found in 1 entry, of 2 entries carrying #work") {
		t.Errorf("stdout = %q", h.out())
	}

	// No match still says what was searched, so an empty result is explainable.
	if err := h.run("search", "--tag", "work", "nothing at all"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	if !strings.Contains(out, "No matches found") || !strings.Contains(out, "2 entries carrying #work") {
		t.Errorf("stdout = %q", out)
	}
}

// TestSearchStillNeedsAQuery: --tag narrows a search, it does not replace one.
func TestSearchStillNeedsAQuery(t *testing.T) {
	h := newHarness(t, "")
	writeEntry(t, h, "2026-07-26", "# 2026-07-26\n\nroadmap [#work]\n")
	assertUsageError(t, h.run("search", "--tag", "work"), "search", "--tag", "work")
}

// TestSearchFindsTags is how tags are browsed now that `list --tag` is gone.
func TestSearchFindsTags(t *testing.T) {
	h := newHarness(t, "")
	writeEntry(t, h, "2026-07-26", "# 2026-07-26\n\nabout #work today\n")
	writeEntry(t, h, "2026-07-27", "# 2026-07-27\n\nabout #home today\n")

	if err := h.run("search", "#work"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	if !strings.Contains(out, "2026-07-26") {
		t.Errorf("tagged entry missing:\n%s", out)
	}
	if strings.Contains(out, "2026-07-27") {
		t.Errorf("untagged entry should not match:\n%s", out)
	}

	if err := h.run("search", "#nothing"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "No matches found") {
		t.Errorf("stdout = %q", h.out())
	}
}

func TestSearchJoinsItsQuery(t *testing.T) {
	h := newHarness(t, "")
	writeEntry(t, h, "2026-07-27", "# 2026-07-27\n\nfound the missing keys\n")

	if err := h.run("search", "missing", "keys"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "missing keys") {
		t.Errorf("stdout = %q", h.out())
	}
	assertUsageError(t, h.run("search"), "search")
}

func TestWeekListsEntriesOfCurrentWeek(t *testing.T) {
	h := newHarness(t, "")
	for _, date := range []string{"2026-07-26", "2026-07-27", "2026-07-28"} {
		if err := h.run("open", date); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.run("week"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	// The week of Tuesday 2026-07-28 starts on Monday the 27th.
	if strings.Contains(out, "2026-07-26") {
		t.Errorf("Sunday 26 belongs to the previous week:\n%s", out)
	}
	for _, want := range []string{"2026-07-27", "2026-07-28"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s:\n%s", want, out)
		}
	}
}

func TestCalendarMonthArgument(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if err := h.run("calendar"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "July 2026") {
		t.Errorf("stdout = %q", h.out())
	}
	if err := h.run("calendar", "2026-02"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "February 2026") {
		t.Errorf("stdout = %q", h.out())
	}
	assertUsageError(t, h.run("calendar", "nope"), "calendar", "nope")
	assertUsageError(t, h.run("calendar", "2026-02", "extra"), "calendar", "2026-02", "extra")
}

func TestCommandsNeedingEntriesGiveAHint(t *testing.T) {
	h := newHarness(t, "")
	for _, args := range [][]string{{"search", "x"}} {
		err := h.run(args...)
		if err == nil {
			t.Errorf("%v should fail without an entries directory", args)
			continue
		}
		if !strings.Contains(err.Error(), "To create your first entry") {
			t.Errorf("%v error = %v", args, err)
		}
	}
}

// --- git ---------------------------------------------------------------------

func TestOpenAutosyncCommitsChanges(t *testing.T) {
	h := newHarness(t, `echo "a new thought" >> "$1"`, "AUTOSYNC=true")
	h.initGit()

	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "auto-committed") {
		t.Errorf("stdout = %q", h.out())
	}
	if !strings.Contains(h.gitLog(), "Journal entry for 2026-07-28") {
		t.Errorf("commit missing:\n%s", h.gitLog())
	}
}

func TestOpenAutosyncSkipsUnchangedEntry(t *testing.T) {
	h := newHarness(t, "", "AUTOSYNC=true")
	h.initGit()

	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "No changes detected.") {
		t.Errorf("stdout = %q", h.out())
	}
	if strings.Contains(h.gitLog(), "Journal entry") {
		t.Errorf("nothing should have been committed:\n%s", h.gitLog())
	}
}

func TestOpenWithoutGitKeepsGoing(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "Continuing without git version control") {
		t.Errorf("stdout = %q", h.out())
	}
}

// --- help --------------------------------------------------------------------

func TestHelpListsEveryCommand(t *testing.T) {
	h := newHarness(t, "")
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		if err := h.run(args...); err != nil {
			t.Fatal(err)
		}
		out := h.out()
		for _, cmd := range Commands() {
			if !strings.Contains(out, cmd.Name()) {
				t.Errorf("journal %v does not mention %s", args, cmd.Name())
			}
		}
	}
}

func TestCommandHelp(t *testing.T) {
	h := newHarness(t, "")
	// Both spellings reach the same screen.
	for _, args := range [][]string{{"help", "open"}, {"open", "--help"}, {"open", "-h"}} {
		if err := h.run(args...); err != nil {
			t.Fatal(err)
		}
		out := h.out()
		for _, want := range []string{"journal open [DATE]", "--last"} {
			if !strings.Contains(out, want) {
				t.Errorf("journal %v output missing %q:\n%s", args, want, out)
			}
		}
	}

	// A command without options says so rather than printing an empty section.
	if err := h.run("help", "week"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "takes no options") {
		t.Errorf("stdout = %q", h.out())
	}

	assertUsageError(t, h.run("help", "nope"), "help", "nope")
	assertUsageError(t, h.run("help", "open", "extra"), "help", "open", "extra")
}

// TestHelpDoesNotMentionRemovedFeatures guards against stale help text.
func TestHelpDoesNotMentionRemovedFeatures(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("help"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	for _, gone := range []string{
		"--prompt", "--random", "--diff", "--oldest-first", "--shared",
		"--backup", "--restore", "--archive", "--switch", "--list-journals",
		"--new-journal", "--time", "--export", "--format", "DISABLE_PROMPTS",
		"MULTI_JOURNAL", "ACTIVE_JOURNAL",
	} {
		if strings.Contains(out, gone) {
			t.Errorf("help still mentions the removed %s", gone)
		}
	}
}

// TestEveryCommandHasHelp makes sure a new command cannot ship undocumented.
func TestEveryCommandHasHelp(t *testing.T) {
	h := newHarness(t, "")
	for _, cmd := range Commands() {
		if cmd.Usage() == "" || cmd.Summary() == "" {
			t.Errorf("%s is missing a usage line or a summary", cmd.Name())
		}
		if !strings.HasPrefix(cmd.Usage(), cmd.Name()) {
			t.Errorf("%s: usage %q should start with the command name", cmd.Name(), cmd.Usage())
		}
		if err := h.run("help", cmd.Name()); err != nil {
			t.Errorf("journal help %s failed: %v", cmd.Name(), err)
		}
		for _, flag := range cmd.Flags() {
			if flag.Help == "" {
				t.Errorf("%s: option --%s has no description", cmd.Name(), flag.Long)
			}
			if !strings.Contains(h.out(), "--"+flag.Long) {
				t.Errorf("journal help %s does not document --%s", cmd.Name(), flag.Long)
			}
		}
	}
}

func writeEntry(t *testing.T, h *harness, slug, content string) {
	t.Helper()
	dir := filepath.Join(h.journalDir(), "entries")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// --- version ------------------------------------------------------------------

func TestBuildInfoString(t *testing.T) {
	cases := []struct {
		name  string
		build BuildInfo
		want  string
	}{
		{
			name:  "a release build shows everything",
			build: BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-07-28T16:00:00Z"},
			want:  "1.2.3 (abc1234, 2026-07-28T16:00:00Z)",
		},
		{
			// These are the defaults a plain `go build` leaves behind, and they
			// carry no information worth showing.
			name:  "a local build hides the placeholders",
			build: BuildInfo{Version: "dev", Commit: "none", Date: "unknown"},
			want:  "dev",
		},
		{
			name:  "an empty build still says something",
			build: BuildInfo{},
			want:  "dev",
		},
		{
			name:  "a partial build shows what it has",
			build: BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "unknown"},
			want:  "1.2.3 (abc1234)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.build.String(); got != c.want {
				t.Errorf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	h := newHarness(t, "")
	h.app.Build = BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-07-28T16:00:00Z"}

	// The subcommand and both global spellings agree.
	for _, args := range [][]string{{"version"}, {"--version"}, {"-V"}} {
		if err := h.run(args...); err != nil {
			t.Fatalf("journal %v failed: %v", args, err)
		}
		if got := strings.TrimSpace(h.out()); got != "1.2.3 (abc1234, 2026-07-28T16:00:00Z)" {
			t.Errorf("journal %v printed %q", args, got)
		}
	}

	assertUsageError(t, h.run("version", "extra"), "version", "extra")
}
