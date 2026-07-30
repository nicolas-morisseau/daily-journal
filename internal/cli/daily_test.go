package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daily-journal/internal/standup"
)

// dailyHarness fills a journal with notes today and yesterday, plus one two days
// back that the default window leaves out. Today is 2026-07-28.
func dailyHarness(t *testing.T, configLines ...string) *harness {
	t.Helper()
	h := newHarness(t, "", configLines...)
	writeEntry(t, h, "2026-07-28", "# 2026-07-28\n\n**09:12** - opened the parser\n")
	writeEntry(t, h, "2026-07-27", "# 2026-07-27\n\n**17:05** - shipped the migration\n")
	writeEntry(t, h, "2026-07-26", "# 2026-07-26\n\n**11:00** - the weekend's business\n")
	return h
}

// fakeStandup puts a scripted git-standup first on PATH, so `git standup`
// resolves to it, and returns a projects directory to point PROJECTS_DIR at.
func fakeStandup(t *testing.T, body string) string {
	t.Helper()
	bin := t.TempDir()
	path := filepath.Join(bin, standup.Binary)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return t.TempDir()
}

// TestDailyPrintsBothHalves covers the piped form, which is what makes the recap
// pasteable into wherever the stand-up is written down.
func TestDailyPrintsBothHalves(t *testing.T) {
	projects := fakeStandup(t,
		`printf '%s\n' "$PWD/stats-service"; printf 'abc1234 - fix: the classpath (3 hours ago) <dev> \\n\n'`)
	h := dailyHarness(t, "PROJECTS_DIR="+projects)

	if err := h.run("daily"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	for _, want := range []string{
		"Daily", "Tuesday 28 July 2026",
		"JOURNAL", "09:12", "opened the parser", "17:05", "shipped the migration",
		"GIT", "stats-service", "abc1234", "fix: the classpath",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout should hold %q:\n%s", want, out)
		}
	}
	// Two days back is outside the default window.
	if strings.Contains(out, "the weekend's business") {
		t.Errorf("the window should stop at yesterday:\n%s", out)
	}
}

func TestDailyDaysWidensTheWindow(t *testing.T) {
	projects := fakeStandup(t, `echo "days: $*"`)
	h := dailyHarness(t, "PROJECTS_DIR="+projects)

	if err := h.run("daily", "--days", "2"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "the weekend's business") {
		t.Errorf("--days 2 should reach back two days:\n%s", h.out())
	}

	// And the short form means the same thing.
	if err := h.run("daily", "-d", "2"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "the weekend's business") {
		t.Errorf("-d 2 should reach back two days:\n%s", h.out())
	}
}

func TestDailyRejectsNonsense(t *testing.T) {
	h := dailyHarness(t)
	assertUsageError(t, h.run("daily", "--days", "soon"), "daily", "--days", "soon")
	assertUsageError(t, h.run("daily", "--days", "-2"), "daily", "--days", "-2")
	assertUsageError(t, h.run("daily", "extra"), "daily", "extra")
}

// TestDailyWithoutProjectsDir: the git half is optional, so an unset
// PROJECTS_DIR is explained in place rather than failing the command.
func TestDailyWithoutProjectsDir(t *testing.T) {
	h := dailyHarness(t)

	if err := h.run("daily"); err != nil {
		t.Fatalf("daily should still work without PROJECTS_DIR: %v", err)
	}
	out := h.out()
	if !strings.Contains(out, "PROJECTS_DIR") {
		t.Errorf("the missing key should be named:\n%s", out)
	}
	if !strings.Contains(out, h.app.Config.Path) {
		t.Errorf("the hint should say which file to put it in:\n%s", out)
	}
	if !strings.Contains(out, "opened the parser") {
		t.Errorf("the journal half should still be printed:\n%s", out)
	}
}

func TestDailyWithoutTheHelper(t *testing.T) {
	// A PATH with nothing on it is the closest thing to git-standup not being
	// installed, and PROJECTS_DIR is set so the missing helper is what is left.
	projects := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	h := dailyHarness(t, "PROJECTS_DIR="+projects)

	if err := h.run("daily"); err != nil {
		t.Fatalf("daily should still work without %s: %v", standup.Binary, err)
	}
	out := h.out()
	if !strings.Contains(out, standup.Binary+" is not installed") {
		t.Errorf("the missing helper should be named:\n%s", out)
	}
	if !strings.Contains(out, "brew install git-standup") {
		t.Errorf("the hint should say how to get it:\n%s", out)
	}
	if !strings.Contains(out, "opened the parser") {
		t.Errorf("the journal half should still be printed:\n%s", out)
	}
}

func TestDailyReportsAnUnreadableProjectsDir(t *testing.T) {
	projects := fakeStandup(t, "true")
	h := dailyHarness(t, "PROJECTS_DIR="+filepath.Join(projects, "nope"))

	if err := h.run("daily"); err != nil {
		t.Fatalf("daily should still work: %v", err)
	}
	if !strings.Contains(h.out(), "no such directory") {
		t.Errorf("a wrong PROJECTS_DIR should be reported:\n%s", h.out())
	}
}

func TestDailyOnAnEmptyJournal(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("daily"); err != nil {
		t.Fatalf("daily should work on an empty journal: %v", err)
	}
	if !strings.Contains(h.out(), "nothing written") {
		t.Errorf("an empty window should say so:\n%s", h.out())
	}
}

// TestConfigurationShowsTheProjectsDir keeps `journal help` honest about where
// the git activity comes from.
func TestConfigurationShowsTheProjectsDir(t *testing.T) {
	h := newHarness(t, "")
	if err := h.run("help"); err != nil {
		t.Fatal(err)
	}
	out := h.out()
	if !strings.Contains(out, "PROJECTS_DIR") {
		t.Errorf("help should document PROJECTS_DIR:\n%s", out)
	}
	if !strings.Contains(out, "unset") {
		t.Errorf("an unset projects directory should say so:\n%s", out)
	}

	h = newHarness(t, "", "PROJECTS_DIR=/Users/dev/projects")
	if err := h.run("help"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out(), "/Users/dev/projects") {
		t.Errorf("help should show the configured directory:\n%s", h.out())
	}
}
