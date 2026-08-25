package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateEntryDefaultBody(t *testing.T) {
	j := newTestJournal(t)

	path, created, err := j.CreateEntry(NewEntryOptions{Date: "2026-07-28"})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("first call should report the entry as created")
	}
	content := readFile(t, path)
	want := "# 2026-07-28\n\n"
	if content != want {
		t.Errorf("content = %q, want %q", content, want)
	}

	// A second call must not touch the file.
	if _, created, err := j.CreateEntry(NewEntryOptions{Date: "2026-07-28"}); err != nil {
		t.Fatal(err)
	} else if created {
		t.Error("existing entry should not be reported as created")
	}
	if readFile(t, path) != want {
		t.Error("existing entry was overwritten")
	}
}

func TestCreateEntryPath(t *testing.T) {
	j := newTestJournal(t)
	path, _, err := j.CreateEntry(NewEntryOptions{Date: "2026-07-28"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "2026-07-28.md" {
		t.Errorf("path = %s", path)
	}
}

func TestCreateEntryWritesADateHeading(t *testing.T) {
	j := newTestJournal(t)

	path, _, err := j.CreateEntry(NewEntryOptions{Date: "2026-07-28"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "# 2026-07-28\n\n"; readFile(t, path) != want {
		t.Errorf("content = %q, want %q", readFile(t, path), want)
	}
}

func TestQuickCaptureCreatesAndAppends(t *testing.T) {
	j := newTestJournal(t)

	path, err := j.QuickCapture("2026-07-28", "first thought")
	if err != nil {
		t.Fatal(err)
	}
	want := "# 2026-07-28\n\n\n**14:30** - first thought\n"
	if got := readFile(t, path); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}

	if _, err := j.QuickCapture("2026-07-28", "second thought"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Count(got, "# 2026-07-28") != 1 {
		t.Errorf("heading duplicated:\n%s", got)
	}
	if !strings.HasSuffix(got, "**14:30** - second thought\n") {
		t.Errorf("second capture not appended:\n%s", got)
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
