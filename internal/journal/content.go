package journal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// WordCount counts whitespace-separated words, matching `wc -w`.
func WordCount(content string) int {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	scanner.Split(bufio.ScanWords)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}

// WordCountFile counts the words in a file, reporting 0 when unreadable.
func WordCountFile(path string) int {
	content, err := Read(path)
	if err != nil {
		return 0
	}
	return WordCount(content)
}

// NewEntryOptions configures the creation of a missing entry file.
type NewEntryOptions struct {
	// Date is the entry date in YYYY-MM-DD form.
	Date string
}

// CreateEntry writes a new entry file, holding just a date heading, and reports
// its path.
//
// It is a no-op when the file already exists.
func (j *Journal) CreateEntry(opts NewEntryOptions) (path string, created bool, err error) {
	path = j.PathFor(opts.Date)
	if Exists(path) {
		return path, false, nil
	}
	if err := j.EnsureDirs(); err != nil {
		return "", false, err
	}

	body := fmt.Sprintf("# %s\n\n", opts.Date)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", false, fmt.Errorf("creating entry %s: %w", path, err)
	}
	return path, true, nil
}

// QuickCapture appends a timestamped one-liner to the given day's entry,
// creating the file with a heading when it does not exist yet.
func (j *Journal) QuickCapture(date, text string) (path string, err error) {
	if err := j.EnsureDirs(); err != nil {
		return "", err
	}
	path = j.PathFor(date)

	var b strings.Builder
	if !Exists(path) {
		fmt.Fprintf(&b, "# %s\n\n", date)
	}
	fmt.Fprintf(&b, "\n**%s** - %s\n", j.Now().Format("15:04"), text)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", fmt.Errorf("opening entry %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		return "", fmt.Errorf("appending to entry %s: %w", path, err)
	}
	return path, nil
}
