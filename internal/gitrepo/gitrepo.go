// Package gitrepo wraps the handful of git operations daily-journal needs. Errors
// carry the command's stderr so the CLI can show something actionable.
package gitrepo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrGitMissing is returned when the git binary is not on PATH.
var ErrGitMissing = errors.New("git is not installed. Please install git first")

// Repo is a git working tree.
type Repo struct {
	// Dir is the working tree root.
	Dir string
	// Branch is the branch used for pull and push.
	Branch string
}

// New builds a Repo handle. It performs no I/O.
func New(dir, branch string) *Repo { return &Repo{Dir: dir, Branch: branch} }

// CheckAvailable reports whether the git binary can be found.
func CheckAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrGitMissing
	}
	return nil
}

// run executes git and returns its stdout.
func (r *Repo) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return stdout.String(), fmt.Errorf("git %s: %w%s", strings.Join(args, " "), err, indent(msg))
	}
	return stdout.String(), nil
}

// runVisible executes git with its output attached to the terminal, for
// commands whose progress the user should see (commit, push).
func (r *Repo) runVisible(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func indent(msg string) string {
	if msg == "" {
		return ""
	}
	return "\n  " + strings.ReplaceAll(msg, "\n", "\n  ")
}

// IsRepo reports whether Dir contains a .git directory.
func (r *Repo) IsRepo() bool {
	_, err := os.Stat(filepath.Join(r.Dir, ".git"))
	return err == nil
}

// Init initialises a repository with a .gitignore and an initial commit,
// mirroring what the bash version does on first run.
func (r *Repo) Init() error {
	if _, err := r.run("init"); err != nil {
		return err
	}
	gitignore := filepath.Join(r.Dir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("*.swp\n.DS_Store\n"), 0o644); err != nil {
			return fmt.Errorf("writing .gitignore: %w", err)
		}
	}
	if _, err := r.run("add", ".gitignore"); err != nil {
		return err
	}
	if _, err := r.run("commit", "-m", "Initial commit"); err != nil {
		return err
	}
	return nil
}

// HasRemote reports whether an `origin` remote is configured.
func (r *Repo) HasRemote() bool {
	_, err := r.run("remote", "get-url", "origin")
	return err == nil
}

// PullRebase pulls with rebase from origin.
func (r *Repo) PullRebase() error {
	_, err := r.run("pull", "origin", r.Branch, "--rebase")
	return err
}

// Push pushes the configured branch, showing git's own output.
func (r *Repo) Push() error {
	return r.runVisible("push", "origin", r.Branch)
}

// Add stages a path.
func (r *Repo) Add(path string) error {
	_, err := r.run("add", "--", path)
	return err
}

// Commit records a commit, showing git's own summary.
func (r *Repo) Commit(message string) error {
	return r.runVisible("commit", "-m", message)
}
