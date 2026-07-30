// Package standup reads the day's git activity through git-standup, the
// third-party helper that walks a directory of repositories and lists what their
// owner committed there.
//
// The helper is optional: when it is not installed the caller gets
// ErrNotInstalled and can say so, rather than the stand-up failing over a tool
// nobody asked for.
package standup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Binary is the helper this package shells out to, as `git standup` resolves it.
const Binary = "git-standup"

// DefaultDays is how far back the report reaches: one day, which covers
// yesterday and today. It is what a stand-up is about, and what `-d 1` means to
// git-standup.
const DefaultDays = 1

// Timeout bounds one run. Walking a hundred repositories takes seconds, and a
// run that hangs must not take the view it feeds down with it.
const Timeout = 90 * time.Second

// ErrNotInstalled is returned when git-standup cannot be found on PATH.
var ErrNotInstalled = fmt.Errorf("%s is not installed", Binary)

// InstallHint tells the user how to get the helper, since a missing optional
// tool is worth one line rather than silence.
const InstallHint = "Install it with 'brew install git-standup', " +
	"or see github.com/kamranahmedse/git-standup"

// Commit is one commit git-standup reported.
type Commit struct {
	// Hash is the abbreviated hash.
	Hash string
	// Subject is the commit's first line.
	Subject string
	// When is the date as git-standup prints it, relative by default, e.g.
	// "3 hours ago". It is kept as text: reformatting it would only lose what the
	// helper already said well.
	When string
	// Author is the commit's author name.
	Author string
}

// Repo is one repository's commits, newest first.
type Repo struct {
	// Path is the repository's working tree.
	Path string
	// Commits are the commits found in the window.
	Commits []Commit
}

// Name is the repository's directory name, which is what people call it.
func (r Repo) Name() string { return filepath.Base(r.Path) }

// Report is one run of the helper: every repository that saw activity.
type Report struct {
	// Dir is the directory that was walked.
	Dir string
	// Days is how many days back the run reached.
	Days int
	// Repos are the repositories with at least one commit, in the order
	// git-standup walked them.
	Repos []Repo
}

// Commits counts every commit in the report.
func (r Report) Commits() int {
	n := 0
	for _, repo := range r.Repos {
		n += len(repo.Commits)
	}
	return n
}

// Empty reports whether nothing was committed in the window.
func (r Report) Empty() bool { return len(r.Repos) == 0 }

// Available reports whether git-standup can be found.
func Available() bool {
	_, err := exec.LookPath(Binary)
	return err == nil
}

// Run reports the git activity of the last days in dir.
//
// It runs `git standup -s -d <days>`, which lists the commits the repository's
// own user made and stays quiet about the repositories that saw none.
func Run(ctx context.Context, dir string, days int) (Report, error) {
	if days < 1 {
		days = DefaultDays
	}
	if !Available() {
		return Report{}, ErrNotInstalled
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Report{}, fmt.Errorf("no such directory: %s", dir)
	}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "standup", "-s", "-d", strconv.Itoa(days))
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// A timeout still carries whatever the helper managed to print, and a partial
	// report beats none: the repositories it did walk are the recent ones.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		report := Parse(stdout.String())
		report.Dir, report.Days = dir, days
		return report, fmt.Errorf("%s gave up after %s in %s", Binary, Timeout, dir)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return Report{}, fmt.Errorf("%s in %s: %w\n  %s", Binary, dir, err, msg)
		}
		return Report{}, fmt.Errorf("%s in %s: %w", Binary, dir, err)
	}

	report := Parse(stdout.String())
	report.Dir, report.Days = dir, days
	return report, nil
}

var (
	// commitRe matches one commit line, `a83cd70 - subject (3 hours ago) <name>`.
	// The subject is greedy so that a conventional-commit scope such as
	// `fix(log): …` stays part of it and the trailing `(date) <author>` is still
	// what gets captured.
	commitRe = regexp.MustCompile(`^([0-9a-fA-F]{6,40}) - (.+) \((.+)\) <(.+)>$`)
	// ansiRe strips the colours git-standup emits when it thinks it is talking to
	// a terminal.
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

// Parse reads git-standup's output into a report.
//
// Its format is one repository path per line followed by that repository's
// commits, and every commit line ends with a literal `\n` the helper's own
// format string leaves behind. Anything else — the "no activity" notice, a
// warning — is skipped rather than mistaken for a repository.
func Parse(out string) Report {
	var report Report
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(ansiRe.ReplaceAllString(raw, ""))
		line = strings.TrimSpace(strings.TrimSuffix(line, `\n`))
		if line == "" {
			continue
		}

		if m := commitRe.FindStringSubmatch(line); m != nil {
			// A commit before any repository header has nowhere to go, which only
			// happens on truncated output.
			if len(report.Repos) == 0 {
				continue
			}
			last := &report.Repos[len(report.Repos)-1]
			last.Commits = append(last.Commits, Commit{
				Hash:    m[1],
				Subject: strings.TrimSpace(m[2]),
				When:    strings.TrimSpace(m[3]),
				Author:  strings.TrimSpace(m[4]),
			})
			continue
		}
		if !isRepoPath(line) {
			continue
		}
		report.Repos = append(report.Repos, Repo{Path: line})
	}

	// Drop the repositories the helper announced without listing anything for,
	// which is what happens when it runs without `-s`.
	kept := report.Repos[:0]
	for _, repo := range report.Repos {
		if len(repo.Commits) > 0 {
			kept = append(kept, repo)
		}
	}
	report.Repos = kept
	return report
}

// isRepoPath reports whether a line looks like the path of a repository rather
// than one of the helper's notices, such as "No commits during this period".
//
// It only has to be roughly right: a notice mistaken for a repository ends up
// with no commits under it, and Parse drops those.
func isRepoPath(line string) bool {
	return strings.Contains(line, string(filepath.Separator))
}
