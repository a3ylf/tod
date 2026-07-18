package todo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigRoundTripAndDatabasePathResolution(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	wantPath := filepath.Join(t.TempDir(), "tasks.json")
	if err := SaveConfig(configPath, Config{DBPath: "  " + wantPath + "  "}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.DBPath != wantPath {
		t.Fatalf("DBPath = %q, want %q", config.DBPath, wantPath)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(configPath))
	if err != nil {
		t.Fatalf("read config directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary config file remains: %s", entry.Name())
		}
	}

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)
	t.Setenv(DatabasePathEnv, "")
	if err := SaveConfig("", Config{DBPath: wantPath}); err != nil {
		t.Fatalf("SaveConfig default path: %v", err)
	}
	got, err := ResolveDatabasePath("")
	if err != nil {
		t.Fatalf("ResolveDatabasePath: %v", err)
	}
	if got != wantPath {
		t.Fatalf("resolved path = %q, want %q", got, wantPath)
	}
}

func TestResolveDatabasePathPrecedence(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)
	configPath := filepath.Join(configHome, "todos", "config.json")
	configValue := filepath.Join(t.TempDir(), "config.json")
	envValue := filepath.Join(t.TempDir(), "env.json")
	explicitValue := filepath.Join(t.TempDir(), "explicit.json")
	if err := SaveConfig(configPath, Config{DBPath: configValue}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	t.Setenv(DatabasePathEnv, "")
	got, err := ResolveDatabasePath("")
	if err != nil {
		t.Fatalf("config resolution: %v", err)
	}
	if got != configValue {
		t.Fatalf("config path = %q, want %q", got, configValue)
	}

	t.Setenv(DatabasePathEnv, envValue)
	got, err = ResolveDatabasePath("")
	if err != nil {
		t.Fatalf("environment resolution: %v", err)
	}
	if got != envValue {
		t.Fatalf("environment path = %q, want %q", got, envValue)
	}

	got, err = ResolveDatabasePath(explicitValue)
	if err != nil {
		t.Fatalf("explicit resolution: %v", err)
	}
	if got != explicitValue {
		t.Fatalf("explicit path = %q, want %q", got, explicitValue)
	}
}

func TestResolveDatabasePathRequiresConfiguration(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)
	t.Setenv(DatabasePathEnv, "")
	_, err := ResolveDatabasePath("")
	if !errors.Is(err, ErrDatabasePathNotConfigured) {
		t.Fatalf("ResolveDatabasePath error = %v, want ErrDatabasePathNotConfigured", err)
	}
}

func TestNormalizeDatabasePath(t *testing.T) {
	got, err := NormalizeDatabasePath(`"./tasks.json"`)
	if err != nil {
		t.Fatalf("NormalizeDatabasePath: %v", err)
	}
	want, err := filepath.Abs("tasks.json")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if got != want {
		t.Fatalf("normalized path = %q, want %q", got, want)
	}
	if _, err := NormalizeDatabasePath(`""`); !errors.Is(err, ErrDatabasePathNotConfigured) {
		t.Fatalf("empty quoted path error = %v, want ErrDatabasePathNotConfigured", err)
	}
}

func TestDatabaseOpenSaveAndExclusiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tasks.json")
	db, store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if store.NextID != 1 {
		t.Fatalf("new store NextID = %d, want 1", store.NextID)
	}

	updated := Store{
		NextID: 2,
		Tasks:  []Task{{ID: 1, Title: "shared", Project: "Inbox", Priority: 4, CreatedAt: time.Now()}},
	}
	if err := db.Save(updated); err != nil {
		t.Fatalf("Database.Save: %v", err)
	}
	_, _, err = Open(path)
	if !errors.Is(err, ErrDatabaseLocked) {
		t.Fatalf("second Open error = %v, want ErrDatabaseLocked", err)
	}
	var lockErr *DatabaseLockError
	if !errors.As(err, &lockErr) {
		t.Fatalf("second Open error type = %T, want *DatabaseLockError", err)
	}
	if lockErr.Owner.PID != os.Getpid() {
		t.Fatalf("lock owner pid = %d, want %d", lockErr.Owner.PID, os.Getpid())
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Database.Close: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock after close error = %v, want not exist", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after close: %v", err)
	}
	if len(loaded.Tasks) != 1 || loaded.Tasks[0].Title != "shared" {
		t.Fatalf("loaded store = %+v, want saved task", loaded)
	}
}

func TestDatabaseStaleLockRequiresManualRemoval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := EnsureDatabaseDirectory(path); err != nil {
		t.Fatalf("EnsureDatabaseDirectory: %v", err)
	}
	if err := os.WriteFile(path+".lock", []byte(`{"pid":123}`+"\n"), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	_, _, err := Open(path)
	if !errors.Is(err, ErrDatabaseLocked) {
		t.Fatalf("Open with stale lock error = %v, want ErrDatabaseLocked", err)
	}
	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatalf("remove stale lock: %v", err)
	}
	db, _, err := Open(path)
	if err != nil {
		t.Fatalf("Open after stale lock removal: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close after stale lock removal: %v", err)
	}
}

func TestAtomicSaveReplacesExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	first := Store{NextID: 2, Tasks: []Task{{ID: 1, Title: "first"}}}
	second := Store{NextID: 3, Tasks: []Task{{ID: 1, Title: "first"}, {ID: 2, Title: "second"}}}
	if err := Save(path, first); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := Save(path, second); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Tasks) != 2 || loaded.Tasks[1].Title != "second" {
		t.Fatalf("loaded store = %+v, want second version", loaded)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read database directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tasks.json.tmp-") {
			t.Fatalf("temporary database file remains: %s", entry.Name())
		}
	}
}

func TestLoadMalformedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write malformed database: %v", err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "read "+path) {
		t.Fatalf("Load error = %v, want path-aware malformed JSON error", err)
	}
}
