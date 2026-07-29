package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFileString reads a file, failing the test if it cannot.
func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// testEnv builds an Env rooted at a temporary home directory.
func testEnv(t *testing.T, extra map[string]string) Env {
	t.Helper()
	home := t.TempDir()
	vars := map[string]string{"HOME": home}
	for k, v := range extra {
		vars[k] = v
	}
	return Env{Home: home, Getenv: func(key string) string { return vars[key] }}
}

func writeConfig(t *testing.T, env Env, content string) string {
	t.Helper()
	dir, file := Paths(env)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestLoadDefaults(t *testing.T) {
	env := testEnv(t, nil)
	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(env.Home, "daily-journal")
	if cfg.JournalDir != want {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, want)
	}
	if cfg.GitBranch != DefaultGitBranch {
		t.Errorf("GitBranch = %q, want %q", cfg.GitBranch, DefaultGitBranch)
	}
	if cfg.Autosync {
		t.Error("Autosync should default to false")
	}
}

func TestLoadKeepsAnExistingLegacyJournalDir(t *testing.T) {
	env := testEnv(t, nil)
	legacy := filepath.Join(env.Home, "journalot")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JournalDir != legacy {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, legacy)
	}
}

func TestLoadPrefersTheCurrentJournalDir(t *testing.T) {
	env := testEnv(t, nil)
	for _, name := range []string{"journalot", "daily-journal"} {
		if err := os.MkdirAll(filepath.Join(env.Home, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(env.Home, "daily-journal"); cfg.JournalDir != want {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, want)
	}
}

func TestLoadParsesShellConfig(t *testing.T) {
	env := testEnv(t, nil)
	writeConfig(t, env, `# journalot config
export JOURNAL_DIR="$HOME/my journal"   # with a comment
AUTOSYNC=true
GIT_BRANCH='master'
EDITOR=hx
`)

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(env.Home, "my journal"); cfg.JournalDir != want {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, want)
	}
	if !cfg.Autosync {
		t.Error("AUTOSYNC=true should enable Autosync")
	}
	if cfg.GitBranch != "master" {
		t.Errorf("GitBranch = %q, want master", cfg.GitBranch)
	}
	if cfg.Editor != "hx" {
		t.Errorf("Editor = %q, want hx", cfg.Editor)
	}
}

// TestLoadSprintStart: the value is carried through unparsed, so a typo is the
// review command's problem rather than every command's.
func TestLoadSprintStart(t *testing.T) {
	env := testEnv(t, nil)
	if cfg, err := Load(env); err != nil {
		t.Fatal(err)
	} else if cfg.SprintStart != "" {
		t.Errorf("SprintStart = %q, want empty without a config file", cfg.SprintStart)
	}

	writeConfig(t, env, "SPRINT_START=2026-07-06   # sprints start on this Monday\n")
	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SprintStart != "2026-07-06" {
		t.Errorf("SprintStart = %q, want 2026-07-06", cfg.SprintStart)
	}

	writeConfig(t, env, "SPRINT_START=whenever\n")
	if cfg, err := Load(env); err != nil {
		t.Errorf("a malformed date must not stop the config loading: %v", err)
	} else if cfg.SprintStart != "whenever" {
		t.Errorf("SprintStart = %q, want it passed through as written", cfg.SprintStart)
	}
}

// TestLoadIgnoresRetiredKeys covers config files written for earlier versions:
// keys that no longer exist must be skipped, never rejected.
func TestLoadIgnoresRetiredKeys(t *testing.T) {
	env := testEnv(t, nil)
	writeConfig(t, env, `MULTI_JOURNAL=false
ACTIVE_JOURNAL=work
DISABLE_PROMPTS=true
AUTOSYNC=true
`)

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	// ACTIVE_JOURNAL must not redirect the journal anywhere any more.
	if want := filepath.Join(env.Home, "daily-journal"); cfg.JournalDir != want {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, want)
	}
	if !cfg.Autosync {
		t.Error("the keys that still exist should still apply")
	}
}

func TestConfigFileWinsOverEnvironment(t *testing.T) {
	env := testEnv(t, map[string]string{"JOURNAL_DIR": "/from/env"})
	writeConfig(t, env, "JOURNAL_DIR=/from/file\n")

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	// The shell version sources the config after applying env defaults, so the
	// file wins. Keep that order.
	if cfg.JournalDir != "/from/file" {
		t.Errorf("JournalDir = %q, want /from/file", cfg.JournalDir)
	}
}

func TestEnvironmentUsedWithoutConfigFile(t *testing.T) {
	env := testEnv(t, map[string]string{"JOURNAL_DIR": "/from/env"})
	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JournalDir != "/from/env" {
		t.Errorf("JournalDir = %q, want /from/env", cfg.JournalDir)
	}
}

func TestTildeExpanded(t *testing.T) {
	env := testEnv(t, nil)
	writeConfig(t, env, "JOURNAL_DIR=~/notes\n")

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(env.Home, "notes"); cfg.JournalDir != want {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, want)
	}
}

func TestConfigPath(t *testing.T) {
	env := testEnv(t, nil)
	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(env.Home, ".config", AppName, "config")
	if cfg.Path != want {
		t.Errorf("Path = %q, want %q", cfg.Path, want)
	}
}

func TestXDGConfigHomeRespected(t *testing.T) {
	custom := t.TempDir()
	env := testEnv(t, map[string]string{"XDG_CONFIG_HOME": custom})
	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(custom, AppName, "config")
	if cfg.Path != want {
		t.Errorf("Path = %q, want %q", cfg.Path, want)
	}
}

// TestMigrationFromRenamedConfigDir covers the daily-journal rename: a config
// left in .config/journalot must be picked up and moved, not ignored.
func TestMigrationFromRenamedConfigDir(t *testing.T) {
	env := testEnv(t, nil)
	oldDir := filepath.Join(env.Home, ".config", "journalot")
	oldFile := filepath.Join(oldDir, "config")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFile, []byte("AUTOSYNC=true\nJOURNAL_DIR=$HOME/journal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MigratedFrom != oldFile {
		t.Errorf("MigratedFrom = %q, want %q", cfg.MigratedFrom, oldFile)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("the old config should have been moved")
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("the emptied config directory should have been removed")
	}
	// The settings must survive the move.
	if !cfg.Autosync {
		t.Error("AUTOSYNC did not survive the migration")
	}
	if want := filepath.Join(env.Home, "journal"); cfg.JournalDir != want {
		t.Errorf("JournalDir = %q, want %q", cfg.JournalDir, want)
	}
	if content := readFileString(t, cfg.Path); !strings.Contains(content, "AUTOSYNC=true") {
		t.Errorf("migrated file = %q", content)
	}
}

// TestMigrationLeavesASharedConfigDirAlone: .config/journalot may hold other
// files, and then only our own is taken.
func TestMigrationLeavesASharedConfigDirAlone(t *testing.T) {
	env := testEnv(t, nil)
	oldDir := filepath.Join(env.Home, ".config", "journalot")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"config": "AUTOSYNC=true\n", "notes.txt": "keep me\n"} {
		if err := os.WriteFile(filepath.Join(oldDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Load(env); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(oldDir, "notes.txt")); err != nil {
		t.Errorf("an unrelated file was removed: %v", err)
	}
}

func TestLegacyRcFileMigration(t *testing.T) {
	env := testEnv(t, nil)
	legacy := filepath.Join(env.Home, ".journalotrc")
	if err := os.WriteFile(legacy, []byte("AUTOSYNC=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MigratedFrom != legacy {
		t.Errorf("MigratedFrom = %q, want %q", cfg.MigratedFrom, legacy)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("legacy config should have been moved")
	}
	if !cfg.Autosync {
		t.Error("migrated config should be applied")
	}
}

// TestMigrationPrefersTheNewerLegacyLocation: with both older locations present,
// the more recent one wins.
func TestMigrationPrefersTheNewerLegacyLocation(t *testing.T) {
	env := testEnv(t, nil)
	oldDir := filepath.Join(env.Home, ".config", "journalot")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "config"), []byte("GIT_BRANCH=from-xdg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.Home, ".journalotrc"), []byte("GIT_BRANCH=from-rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitBranch != "from-xdg" {
		t.Errorf("GitBranch = %q, want from-xdg", cfg.GitBranch)
	}
}

func TestMigrationSkippedWhenTargetExists(t *testing.T) {
	env := testEnv(t, nil)
	if err := os.WriteFile(filepath.Join(env.Home, ".journalotrc"), []byte("AUTOSYNC=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, env, "AUTOSYNC=false\n")

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MigratedFrom != "" {
		t.Error("should not migrate over an existing config")
	}
	if cfg.Autosync {
		t.Error("the existing config should win")
	}
}
