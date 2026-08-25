package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeState(t *testing.T, env Env, name string) string {
	t.Helper()
	_, file := StatePaths(env)
	if err := WriteActive(file, name); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestStatePathsFollowXDGStateHome(t *testing.T) {
	env := testEnv(t, nil)
	_, file := StatePaths(env)
	if want := filepath.Join(env.Home, ".local", "state", AppName, StateFileName); file != want {
		t.Errorf("state file = %q, want %q", file, want)
	}

	env = testEnv(t, map[string]string{"XDG_STATE_HOME": "/somewhere/state"})
	if _, file := StatePaths(env); file != filepath.Join("/somewhere/state", AppName, StateFileName) {
		t.Errorf("state file = %q", file)
	}
}

func TestWriteActiveCreatesTheDirectory(t *testing.T) {
	env := testEnv(t, nil)
	file := writeState(t, env, "work")

	if got := readFileString(t, file); got != "work\n" {
		t.Errorf("state = %q, want %q", got, "work\n")
	}
}

func TestLoadDefaultsToTheDefaultJournal(t *testing.T) {
	cfg, err := Load(testEnv(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveJournal != DefaultJournalName {
		t.Errorf("ActiveJournal = %q, want %q", cfg.ActiveJournal, DefaultJournalName)
	}
	if cfg.ActiveFrom != FromDefault {
		t.Errorf("ActiveFrom = %v, want FromDefault", cfg.ActiveFrom)
	}
}

func TestLoadReadsDefaultJournalFromTheConfigFile(t *testing.T) {
	env := testEnv(t, nil)
	writeConfig(t, env, "DEFAULT_JOURNAL=perso\n")

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultJournal != "perso" || cfg.ActiveJournal != "perso" {
		t.Errorf("DefaultJournal = %q, ActiveJournal = %q, want both perso", cfg.DefaultJournal, cfg.ActiveJournal)
	}
}

func TestTheStateFileWinsOverTheConfiguredDefault(t *testing.T) {
	env := testEnv(t, nil)
	writeConfig(t, env, "DEFAULT_JOURNAL=perso\n")
	writeState(t, env, "work")

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveJournal != "work" || cfg.ActiveFrom != FromState {
		t.Errorf("ActiveJournal = %q from %v, want work from FromState", cfg.ActiveJournal, cfg.ActiveFrom)
	}
	if cfg.DefaultJournal != "perso" {
		t.Errorf("DefaultJournal = %q, want perso: the fallback is not overwritten", cfg.DefaultJournal)
	}
}

func TestTheEnvironmentWinsOverTheStateFile(t *testing.T) {
	env := testEnv(t, map[string]string{EnvJournal: "reading"})
	writeConfig(t, env, "DEFAULT_JOURNAL=perso\n")
	writeState(t, env, "work")

	cfg, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveJournal != "reading" || cfg.ActiveFrom != FromEnv {
		t.Errorf("ActiveJournal = %q from %v, want reading from FromEnv", cfg.ActiveJournal, cfg.ActiveFrom)
	}
}

func TestAnUnreadableStateFileFallsBackToTheDefault(t *testing.T) {
	env := testEnv(t, nil)
	dir, file := StatePaths(env)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory where the state file should be: unreadable, but not fatal.
	if err := os.MkdirAll(file, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(env)
	if err != nil {
		t.Fatalf("a broken state file must not stop the tool: %v", err)
	}
	if cfg.ActiveJournal != DefaultJournalName {
		t.Errorf("ActiveJournal = %q, want %q", cfg.ActiveJournal, DefaultJournalName)
	}
}

func TestActiveSourceNamesItself(t *testing.T) {
	cases := map[ActiveSource]string{
		FromFlag:    "--journal",
		FromEnv:     "$JOURNAL",
		FromState:   "journal use",
		FromDefault: KeyDefaultJournal,
	}
	for source, want := range cases {
		if got := source.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", source, got, want)
		}
	}
}
