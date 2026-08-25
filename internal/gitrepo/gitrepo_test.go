package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo returns an initialised Repo on a temporary directory. It skips the
// test when git is unavailable or cannot commit (no identity configured).
func newRepo(t *testing.T) *Repo {
	t.Helper()
	if err := CheckAvailable(); err != nil {
		t.Skip("git is not installed")
	}
	unsignCommits(t)
	repo := New(t.TempDir(), "main")
	if err := repo.Init(); err != nil {
		t.Skipf("git cannot commit in this environment: %v", err)
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		if _, err := repo.run(args...); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// unsignCommits turns commit signing off for every git command this test
// process runs, so a developer with `commit.gpgsign = true` does not have the
// suite block on a passphrase prompt with nowhere to appear.
func unsignCommits(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "commit.gpgsign")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	t.Setenv("GIT_CONFIG_KEY_1", "tag.gpgsign")
	t.Setenv("GIT_CONFIG_VALUE_1", "false")
}

func TestInitCreatesRepoAndGitignore(t *testing.T) {
	repo := newRepo(t)

	if !repo.IsRepo() {
		t.Error("IsRepo should be true after Init")
	}
	gitignore := readFile(t, filepath.Join(repo.Dir, ".gitignore"))
	for _, want := range []string{"*.swp", ".DS_Store"} {
		if !strings.Contains(gitignore, want) {
			t.Errorf(".gitignore missing %q:\n%s", want, gitignore)
		}
	}
	if repo.HasRemote() {
		t.Error("a fresh repository has no origin")
	}
}

func TestIsRepoFalseOnPlainDirectory(t *testing.T) {
	if New(t.TempDir(), "main").IsRepo() {
		t.Error("a plain directory is not a repository")
	}
}

func TestInitKeepsExistingGitignore(t *testing.T) {
	if err := CheckAvailable(); err != nil {
		t.Skip("git is not installed")
	}
	repo := New(t.TempDir(), "main")
	if err := os.WriteFile(filepath.Join(repo.Dir, ".gitignore"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.Init(); err != nil {
		t.Skipf("git cannot commit in this environment: %v", err)
	}
	if got := readFile(t, filepath.Join(repo.Dir, ".gitignore")); got != "custom\n" {
		t.Errorf(".gitignore = %q, want it left alone", got)
	}
}

func TestAddAndCommit(t *testing.T) {
	repo := newRepo(t)

	entries := filepath.Join(repo.Dir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(entries, "2026-07-28.md")
	if err := os.WriteFile(path, []byte("# 2026-07-28\n\nfirst line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := repo.Add(path); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit("Journal entry for 2026-07-28"); err != nil {
		t.Fatal(err)
	}

	out, err := repo.run("log", "--format=%s", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Journal entry for 2026-07-28", "entries/2026-07-28.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
}

func TestPullFailsWithoutRemote(t *testing.T) {
	repo := newRepo(t)
	if err := repo.PullRebase(); err == nil {
		t.Error("PullRebase should fail without a remote")
	}
}

func TestErrorCarriesStderr(t *testing.T) {
	repo := newRepo(t)
	_, err := repo.run("checkout", "does-not-exist")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should include git's message, got %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
