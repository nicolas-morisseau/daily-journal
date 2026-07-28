package journal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"daily-journal/internal/dateparse"
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

// TemplateVars are the placeholders substituted into template.md.
type TemplateVars struct {
	Date       string
	DayOfWeek  string
	WeekNumber string
	Time       string
}

// NewTemplateVars builds the placeholder set for an entry.
//
// `{{date}}` is the entry's date, while the remaining placeholders describe the
// moment of creation, matching the bash implementation.
func NewTemplateVars(entryDate string, now time.Time) TemplateVars {
	return TemplateVars{
		Date:       entryDate,
		DayOfWeek:  now.Format("Monday"),
		WeekNumber: dateparse.ISOWeek(now),
		Time:       now.Format("15:04"),
	}
}

// Render substitutes the placeholders in a template body.
func (v TemplateVars) Render(template string) string {
	return strings.NewReplacer(
		"{{date}}", v.Date,
		"{{day_of_week}}", v.DayOfWeek,
		"{{week_number}}", v.WeekNumber,
		"{{time}}", v.Time,
	).Replace(template)
}

// NewEntryOptions configures the creation of a missing entry file.
type NewEntryOptions struct {
	// Date is the entry date in YYYY-MM-DD form.
	Date string
}

// CreateEntry writes a new entry file and reports its path. It uses
// template.md when present, otherwise just a date heading.
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

	body, err := j.renderNewEntry(opts)
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", false, fmt.Errorf("creating entry %s: %w", path, err)
	}
	return path, true, nil
}

func (j *Journal) renderNewEntry(opts NewEntryOptions) (string, error) {
	if template, err := Read(j.TemplatePath()); err == nil {
		return NewTemplateVars(opts.Date, j.Now()).Render(template), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading template %s: %w", j.TemplatePath(), err)
	}

	return fmt.Sprintf("# %s\n\n", opts.Date), nil
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
