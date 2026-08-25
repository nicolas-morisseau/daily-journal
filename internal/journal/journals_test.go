package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeJournal creates root/name/entries and returns the root.
func makeJournal(t *testing.T, root, name string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, name, EntriesDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestValidateNameAcceptsPlainNames(t *testing.T) {
	for _, name := range []string{"default", "work", "side-project", "notes_2026", "a", "v1.2"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateNameRejectsAnythingThatIsNotADirectoryName(t *testing.T) {
	cases := map[string]string{
		"empty":       "",
		"separator":   "work/notes",
		"parent":      "..",
		"hidden":      ".secret",
		"space":       "my journal",
		"reserved":    EntriesDirName,
		"too long":    strings.Repeat("x", MaxNameLen+1),
		"leading dot": ".work",
	}
	for label, name := range cases {
		if err := ValidateName(name); err == nil {
			t.Errorf("%s: ValidateName(%q) = nil, want an error", label, name)
		}
	}
}

func TestListReturnsOnlyDirectoriesHoldingEntries(t *testing.T) {
	root := t.TempDir()
	makeJournal(t, root, "work")
	makeJournal(t, root, "default")
	// Neither of these is a journal: no entries/ directory.
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"default", "work"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestListOfAMissingRootIsEmpty(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "nothing-here"))
	if err != nil {
		t.Fatalf("a missing root is not an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want none", got)
	}
}

func TestCreateMakesTheEntriesDirectory(t *testing.T) {
	root := t.TempDir()

	dir, err := Create(root, "work")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(root, "work") {
		t.Errorf("dir = %q", dir)
	}
	if !HasJournal(root, "work") {
		t.Error("the journal should exist after Create")
	}
}

func TestCreateRefusesAnExistingJournal(t *testing.T) {
	root := makeJournal(t, t.TempDir(), "work")

	if _, err := Create(root, "work"); err == nil {
		t.Error("creating an existing journal should fail")
	}
}

func TestCreateRefusesAnInvalidName(t *testing.T) {
	root := t.TempDir()

	if _, err := Create(root, "../escape"); err == nil {
		t.Error("a name with a separator should be refused")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape")); err == nil {
		t.Error("nothing should have been created outside the root")
	}
}

func TestMigrateRootEntriesMovesTheOldLayout(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, EntriesDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, EntriesDirName, "2026-07-28.md")
	if err := os.WriteFile(entry, []byte("# 2026-07-28\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	migration, err := MigrateRootEntries(root, "default")
	if err != nil {
		t.Fatal(err)
	}
	if !migration.Moved() {
		t.Fatalf("migration = %+v, want a move", migration)
	}
	if !HasJournal(root, "default") {
		t.Error("the old journal should now be the default one")
	}
	if _, err := os.Stat(filepath.Join(root, "default", EntriesDirName, "2026-07-28.md")); err != nil {
		t.Errorf("the entry did not follow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, EntriesDirName)); err == nil {
		t.Error("the old entries directory should be gone")
	}
}

func TestMigrateRootEntriesIsANoOpOnANewRoot(t *testing.T) {
	root := makeJournal(t, t.TempDir(), "default")

	migration, err := MigrateRootEntries(root, "default")
	if err != nil {
		t.Fatal(err)
	}
	if migration.Moved() || migration.Skipped {
		t.Errorf("migration = %+v, want nothing to do", migration)
	}
}

func TestMigrateRootEntriesNeverOverwrites(t *testing.T) {
	root := makeJournal(t, t.TempDir(), "default")
	if err := os.MkdirAll(filepath.Join(root, EntriesDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(root, EntriesDirName, "2026-07-28.md")
	if err := os.WriteFile(kept, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	migration, err := MigrateRootEntries(root, "default")
	if err != nil {
		t.Fatal(err)
	}
	if !migration.Skipped {
		t.Errorf("migration = %+v, want it skipped", migration)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("the old entry should still be where it was: %v", err)
	}
}
