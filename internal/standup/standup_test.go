package standup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sample is what git-standup prints when it is not talking to a terminal: a
// repository path, then its commits, each line ending in a literal `\n` its own
// format string leaves behind.
const sample = "/Users/dev/projects/stats-service\n" +
	`676ee1818 - chore(log): bump logback-config-library to 3.20.1 (8 days ago) <dev> \n` + "\n" +
	`468a5d8e7 - fix: runtime error with missing classpath (3 weeks ago) <Dev Eloper> \n` + "\n" +
	"/Users/dev/projects/mailer-service\n" +
	`9329a3a - feat: migrate to GitHub Packages (2 hours ago) <dev> \n` + "\n"

func TestParseReadsReposAndCommits(t *testing.T) {
	report := Parse(sample)

	if len(report.Repos) != 2 {
		t.Fatalf("Repos = %d, want 2: %+v", len(report.Repos), report.Repos)
	}
	if got := report.Repos[0].Name(); got != "stats-service" {
		t.Errorf("Name = %q, want stats-service", got)
	}
	if got := report.Commits(); got != 3 {
		t.Errorf("Commits = %d, want 3", got)
	}

	first := report.Repos[0].Commits[0]
	if first.Hash != "676ee1818" {
		t.Errorf("Hash = %q", first.Hash)
	}
	// The subject carries a conventional-commit scope, whose parentheses must not
	// be mistaken for the date at the end of the line.
	if first.Subject != "chore(log): bump logback-config-library to 3.20.1" {
		t.Errorf("Subject = %q", first.Subject)
	}
	if first.When != "8 days ago" {
		t.Errorf("When = %q, want the relative date", first.When)
	}
	if first.Author != "dev" {
		t.Errorf("Author = %q", first.Author)
	}
	if got := report.Repos[0].Commits[1].Author; got != "Dev Eloper" {
		t.Errorf("Author = %q, want a name with a space", got)
	}
}

func TestParseDropsNoticesAndEmptyRepos(t *testing.T) {
	// Without -s the helper announces the repositories that saw nothing, and with
	// -f it says what it is fetching. Neither is a repository with commits.
	out := "Fetching commits in dead-repo\n" +
		"/Users/dev/projects/dead-repo\n" +
		"No commits from dev during this period.\n" +
		"/Users/dev/projects/live-repo\n" +
		`abc1234 - fix: it (1 hour ago) <dev> \n` + "\n"

	report := Parse(out)
	if len(report.Repos) != 1 {
		t.Fatalf("Repos = %+v, want only live-repo", report.Repos)
	}
	if got := report.Repos[0].Name(); got != "live-repo" {
		t.Errorf("Name = %q, want live-repo", got)
	}
	if report.Empty() {
		t.Error("a report with a commit is not empty")
	}
}

func TestParseStripsColours(t *testing.T) {
	// git-standup colours its output when it thinks it has a terminal.
	out := "\x1b[1m\x1b[4m\x1b[33m/Users/dev/projects/api\x1b[0m\n" +
		"\x1b[31mabc1234\x1b[0m - shipped it \x1b[32m(2 hours ago) \x1b[1;34m<dev>\x1b[0m \\n\n"

	report := Parse(out)
	if len(report.Repos) != 1 || len(report.Repos[0].Commits) != 1 {
		t.Fatalf("Parse lost the coloured lines: %+v", report.Repos)
	}
	if got := report.Repos[0].Commits[0].Subject; got != "shipped it" {
		t.Errorf("Subject = %q, want it free of escapes", got)
	}
}

func TestParseOfNothing(t *testing.T) {
	report := Parse("")
	if !report.Empty() || report.Commits() != 0 {
		t.Errorf("an empty run should give an empty report, got %+v", report)
	}
}

// fakeHelper puts a scripted git-standup first on PATH, so `git standup` resolves
// to it. body is the script's shell body.
func fakeHelper(t *testing.T, body string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, Binary)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunCallsTheHelperInTheProjectsDirectory(t *testing.T) {
	// The script reports how it was called, which is what has to be right: -s so
	// idle repositories stay quiet, and -d with the window.
	fakeHelper(t, `echo "args: $*"; echo "cwd: $(pwd)"; printf '%s\n' "$PWD/repo"; printf 'abc1234 - did it (1 hour ago) <dev> \\n\n'`)

	projects := t.TempDir()
	report, err := Run(context.Background(), projects, 2)
	if err != nil {
		t.Fatal(err)
	}
	if report.Dir != projects || report.Days != 2 {
		t.Errorf("report describes %s over %d days, want %s over 2", report.Dir, report.Days, projects)
	}
	if len(report.Repos) != 1 {
		t.Fatalf("Repos = %+v, want the one the helper printed", report.Repos)
	}
	if got := report.Repos[0].Name(); got != "repo" {
		t.Errorf("Name = %q, want repo", got)
	}

	// The "args:" and "cwd:" lines are notices, so they must not have become
	// repositories; the commit landing under the real path proves it.
	if got := report.Commits(); got != 1 {
		t.Errorf("Commits = %d, want 1", got)
	}
}

func TestRunPassesTheWindowAndDefaultsIt(t *testing.T) {
	fakeHelper(t, `printf '%s\n' "$PWD/args-$*"`)

	for _, tc := range []struct{ days, want int }{{0, DefaultDays}, {-3, DefaultDays}, {5, 5}} {
		report, err := Run(context.Background(), t.TempDir(), tc.days)
		if err != nil {
			t.Fatal(err)
		}
		if report.Days != tc.want {
			t.Errorf("days %d resolved to %d, want %d", tc.days, report.Days, tc.want)
		}
	}
}

func TestRunReportsAMissingDirectory(t *testing.T) {
	fakeHelper(t, "true")

	_, err := Run(context.Background(), filepath.Join(t.TempDir(), "nope"), 1)
	if err == nil || !strings.Contains(err.Error(), "no such directory") {
		t.Errorf("err = %v, want a missing directory", err)
	}
}

func TestRunReportsAFailingHelper(t *testing.T) {
	fakeHelper(t, "echo 'something broke' >&2; exit 1")

	_, err := Run(context.Background(), t.TempDir(), 1)
	if err == nil {
		t.Fatal("a failing helper should be reported")
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Errorf("err = %v, want the helper's own message", err)
	}
}

func TestRunWithoutTheHelper(t *testing.T) {
	// An empty PATH is the closest thing to git-standup not being installed.
	t.Setenv("PATH", t.TempDir())
	if Available() {
		t.Fatal("Available should be false with the helper off PATH")
	}
	if _, err := Run(context.Background(), t.TempDir(), 1); err != ErrNotInstalled {
		t.Errorf("err = %v, want ErrNotInstalled", err)
	}
}

func TestRunHonoursACancelledContext(t *testing.T) {
	fakeHelper(t, "sleep 30")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, t.TempDir(), 1); err == nil {
		t.Error("a cancelled run should be reported")
	}
}
