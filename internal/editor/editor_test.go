package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrefersConfiguredEditor(t *testing.T) {
	ed, err := Resolve("code --wait")
	if err != nil {
		t.Fatal(err)
	}
	if ed.Command != "code" {
		t.Errorf("Command = %q, want code", ed.Command)
	}
	if strings.Join(ed.Args, " ") != "--wait" {
		t.Errorf("Args = %v, want [--wait]", ed.Args)
	}
	if ed.String() != "code --wait" {
		t.Errorf("String = %q", ed.String())
	}
}

func TestResolveIgnoresBlankEditor(t *testing.T) {
	// With no configured editor, Resolve falls back to whatever is installed.
	// On a machine with none of them, it must report a clear error.
	ed, err := Resolve("   ")
	if err != nil {
		if err != ErrNoEditor {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	found := false
	for _, candidate := range Fallbacks {
		if ed.Command == candidate {
			found = true
		}
	}
	if !found {
		t.Errorf("Command = %q, want one of %v", ed.Command, Fallbacks)
	}
}

func TestOpenPassesArgumentsAndFiles(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "editor.sh")
	log := filepath.Join(dir, "args.log")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + log + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	ed, err := Resolve(script + " --flag")
	if err != nil {
		t.Fatal(err)
	}
	if err := ed.Open("a.md", "b.md"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "--flag\na.md\nb.md\n"
	if string(got) != want {
		t.Errorf("editor received %q, want %q", got, want)
	}
}

func TestOpenWithoutFilesIsNoop(t *testing.T) {
	ed := Editor{Command: "/nonexistent/editor"}
	if err := ed.Open(); err != nil {
		t.Errorf("Open() with no files should do nothing, got %v", err)
	}
}
