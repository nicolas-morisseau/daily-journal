package journal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A journal root holds one directory per journal, each with its own entries/:
//
//	~/daily-journal/          the root, JOURNAL_DIR
//	├── default/entries/      one journal
//	└── work/entries/         another
//
// The root itself is the git repository, so every journal shares one history,
// one branch and one remote.

// MaxNameLen bounds a journal name, keeping it usable as a directory name on
// every filesystem we care about.
const MaxNameLen = 64

// reservedNames cannot be journal names because they would collide with the
// layout of an unmigrated root.
var reservedNames = map[string]bool{EntriesDirName: true}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ErrUnknownJournal is returned when a journal name has no directory.
var ErrUnknownJournal = errors.New("unknown journal")

// ValidateName reports whether a name can be a journal. The rules are those of
// a plain directory name: no separators, no leading dot, nothing that could
// escape the root.
func ValidateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("a journal name cannot be empty")
	case len(name) > MaxNameLen:
		return fmt.Errorf("journal name %q is too long (max %d characters)", name, MaxNameLen)
	case reservedNames[name]:
		return fmt.Errorf("%q is reserved and cannot be a journal name", name)
	case !nameRe.MatchString(name):
		return fmt.Errorf("invalid journal name %q: use letters, digits, '.', '-' and '_', starting with a letter or a digit", name)
	}
	return nil
}

// DirFor returns the directory of a journal inside root. It does not check that
// the journal exists.
func DirFor(root, name string) string { return filepath.Join(root, name) }

// HasJournal reports whether a journal exists on disk, which means its
// directory holds an entries/ directory.
func HasJournal(root, name string) bool {
	if ValidateName(name) != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(DirFor(root, name), EntriesDirName))
	return err == nil && info.IsDir()
}

// List returns the journals found under root, sorted by name.
//
// A directory is a journal when it contains an entries/ directory. Anything
// else living in the root — .git, notes, a stray README — is ignored, so the
// listing never invents journals.
func List(root string) ([]string, error) {
	dirEntries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing journals in %s: %w", root, err)
	}

	var names []string
	for _, d := range dirEntries {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if HasJournal(root, d.Name()) {
			names = append(names, d.Name())
		}
	}
	return names, nil
}

// Create makes a new journal and returns its directory. Creating one that
// already exists is an error, so a typo never silently reuses another journal.
func Create(root, name string) (dir string, err error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if HasJournal(root, name) {
		return "", fmt.Errorf("journal %q already exists", name)
	}
	dir = DirFor(root, name)
	if err := os.MkdirAll(filepath.Join(dir, EntriesDirName), 0o755); err != nil {
		return "", fmt.Errorf("creating journal %s: %w", dir, err)
	}
	return dir, nil
}

// Migration describes what MigrateRootEntries did.
type Migration struct {
	// From and To are the moved directory and its new location, both empty
	// when nothing moved.
	From, To string
	// Skipped is set when there was something to move but the destination was
	// already taken, which leaves the old directory in place.
	Skipped bool
}

// Moved reports whether the migration actually moved anything.
func (m Migration) Moved() bool { return m.From != "" && !m.Skipped }

// MigrateRootEntries moves a pre-multi-journal `<root>/entries` into
// `<root>/<name>/entries`, turning the single journal of earlier versions into
// the first journal of the root.
//
// It never overwrites: when `<root>/<name>` already exists the move is skipped
// and reported, leaving the caller to say so rather than silently merging two
// journals.
func MigrateRootEntries(root, name string) (Migration, error) {
	if err := ValidateName(name); err != nil {
		return Migration{}, err
	}

	from := filepath.Join(root, EntriesDirName)
	info, err := os.Stat(from)
	if err != nil || !info.IsDir() {
		return Migration{}, nil
	}

	dir := DirFor(root, name)
	to := filepath.Join(dir, EntriesDirName)
	if _, err := os.Stat(dir); err == nil {
		return Migration{From: from, To: to, Skipped: true}, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Migration{}, fmt.Errorf("creating journal %s: %w", dir, err)
	}
	if err := os.Rename(from, to); err != nil {
		// Leave the empty directory we just made rather than hiding the cause.
		return Migration{}, fmt.Errorf("moving %s to %s: %w", from, to, err)
	}
	return Migration{From: from, To: to}, nil
}
