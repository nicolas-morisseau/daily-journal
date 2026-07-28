// Package journal implements daily-journal's domain model: a directory of dated
// markdown entries, plus everything derived from it (statistics, search, tags,
// calendars).
//
// Nothing in this package prints, prompts or talks to git. It returns data and
// errors so the CLI and TUI layers can present them however they like.
package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"daily-journal/internal/dateparse"
)

// Directory names inside a journal.
const (
	EntriesDirName = "entries"
	// TemplateFileName is the optional per-journal entry template.
	TemplateFileName = "template.md"
)

// ErrNoEntriesDir is returned when a journal has no entries directory yet.
var ErrNoEntriesDir = errors.New("no entries directory")

// Journal is the journal rooted at Dir.
type Journal struct {
	// Dir is the journal root, which holds entries/ and the optional
	// template.md.
	Dir string
	// Now supplies the current time; tests substitute it.
	Now func() time.Time
}

// New builds a Journal. now may be nil, in which case time.Now is used.
func New(dir string, now func() time.Time) *Journal {
	if now == nil {
		now = time.Now
	}
	return &Journal{Dir: dir, Now: now}
}

// Today is the current date, truncated to midnight.
func (j *Journal) Today() time.Time { return dateparse.Truncate(j.Now()) }

// EntriesDir is the directory holding entry files.
func (j *Journal) EntriesDir() string { return filepath.Join(j.Dir, EntriesDirName) }

// TemplatePath is the optional template file for new entries.
func (j *Journal) TemplatePath() string { return filepath.Join(j.Dir, TemplateFileName) }

// EnsureDirs creates the journal and its entries directory.
func (j *Journal) EnsureDirs() error {
	if err := os.MkdirAll(j.EntriesDir(), 0o755); err != nil {
		return fmt.Errorf("cannot create journal directory %s: %w", j.Dir, err)
	}
	return nil
}

// Entry is one markdown file in the journal: one day, one file.
type Entry struct {
	// Path is the absolute path on disk.
	Path string
	// Slug is the file name without the .md extension, e.g. "2025-01-15".
	Slug string
	// Date is the entry date; zero when the file name is not date-shaped.
	Date time.Time
}

// FileName is the entry's base file name.
func (e Entry) FileName() string { return filepath.Base(e.Path) }

// IsDated reports whether the file name is a `YYYY-MM-DD.md` day entry. Other
// markdown files living in entries/ are listed but never counted as days.
func (e Entry) IsDated() bool { return !e.Date.IsZero() }

var datedRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// parseSlug extracts the date from an entry file name, or the zero time when the
// name is not date-shaped.
func parseSlug(slug string, loc *time.Location) time.Time {
	if !datedRe.MatchString(slug) {
		return time.Time{}
	}
	t, err := time.ParseInLocation(dateparse.Layout, slug, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SortOrder controls how entry listings are ordered.
type SortOrder int

const (
	// NewestFirst is the default listing order.
	NewestFirst SortOrder = iota
	// OldestFirst reverses it (`--oldest-first`).
	OldestFirst
)

// Entries returns every markdown entry, sorted by file name ascending.
//
// It returns ErrNoEntriesDir (wrapped) when the entries directory is missing,
// so callers can print the "run journal to create your first entry" hint.
func (j *Journal) Entries() ([]Entry, error) {
	dir := j.EntriesDir()
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w at %s", ErrNoEntriesDir, dir)
	}

	loc := j.Now().Location()
	var entries []Entry
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable subdirectory should not hide the rest.
			return nil //nolint:nilerr // best-effort walk
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		slug := strings.TrimSuffix(d.Name(), ".md")
		entries = append(entries, Entry{Path: path, Slug: slug, Date: parseSlug(slug, loc)})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("listing entries in %s: %w", dir, walkErr)
	}

	sort.Slice(entries, func(a, b int) bool { return entries[a].Slug < entries[b].Slug })
	return entries, nil
}

// EntriesSorted returns entries in the requested order.
func (j *Journal) EntriesSorted(order SortOrder) ([]Entry, error) {
	entries, err := j.Entries()
	if err != nil {
		return nil, err
	}
	if order == NewestFirst {
		reverse(entries)
	}
	return entries, nil
}

// DatedEntries returns only plain day entries, oldest first.
func (j *Journal) DatedEntries() ([]Entry, error) {
	entries, err := j.Entries()
	if err != nil {
		return nil, err
	}
	dated := entries[:0:0]
	for _, e := range entries {
		if e.IsDated() {
			dated = append(dated, e)
		}
	}
	return dated, nil
}

// Last returns the most recent plain day entry.
func (j *Journal) Last() (Entry, error) {
	dated, err := j.DatedEntries()
	if err != nil {
		return Entry{}, err
	}
	if len(dated) == 0 {
		return Entry{}, ErrNoEntries
	}
	return dated[len(dated)-1], nil
}

// ErrNoEntries signals an empty journal.
var ErrNoEntries = errors.New("no journal entries found")

// PathFor returns the file path for a date.
func (j *Journal) PathFor(date string) string {
	return filepath.Join(j.EntriesDir(), date+".md")
}

// Exists reports whether a path is an existing regular file.
func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Read returns an entry's content.
func Read(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WeekEntries returns the existing day entries of the week containing t,
// Monday through Sunday, in chronological order.
func (j *Journal) WeekEntries(t time.Time) ([]Entry, error) {
	start := dateparse.WeekStart(t)
	loc := t.Location()
	var found []Entry
	for i := range 7 {
		day := start.AddDate(0, 0, i)
		slug := day.Format(dateparse.Layout)
		path := j.PathFor(slug)
		if !Exists(path) {
			continue
		}
		found = append(found, Entry{Path: path, Slug: slug, Date: parseSlug(slug, loc)})
	}
	return found, nil
}

func reverse[T any](s []T) {
	for i, k := 0, len(s)-1; i < k; i, k = i+1, k-1 {
		s[i], s[k] = s[k], s[i]
	}
}
