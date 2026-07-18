package todo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const DatabasePathEnv = "TODOS_DB_PATH"

var ErrDatabasePathNotConfigured = errors.New("database path is not configured")
var ErrDatabaseLocked = errors.New("database is locked")

type Config struct {
	DBPath string `json:"db_path"`
}

type LockMetadata struct {
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	Platform  string    `json:"platform"`
	StartedAt time.Time `json:"started_at"`
}

type DatabaseLockError struct {
	DatabasePath string
	LockPath     string
	Owner        LockMetadata
}

func (e *DatabaseLockError) Error() string {
	message := fmt.Sprintf("database %s is already in use (lock: %s)", e.DatabasePath, e.LockPath)
	if e.Owner.PID > 0 {
		message += fmt.Sprintf("; owner pid %d", e.Owner.PID)
		if e.Owner.Hostname != "" {
			message += " on " + e.Owner.Hostname
		}
		if !e.Owner.StartedAt.IsZero() {
			message += " since " + e.Owner.StartedAt.Format(time.RFC3339)
		}
	}
	message += "; if no instance is running, remove the lock file manually"
	return message
}

func (e *DatabaseLockError) Unwrap() error {
	return ErrDatabaseLocked
}

type Database struct {
	mu       sync.Mutex
	Path     string
	lockPath string
	lockFile *os.File
}

func ConfigPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configHome, "todos", "config.json"), nil
}

func LoadConfig(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return Config{}, err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(raw, &config); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	config.DBPath = strings.TrimSpace(config.DBPath)
	return config, nil
}

func SaveConfig(path string, config Config) error {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return err
		}
	}
	config.DBPath = strings.TrimSpace(config.DBPath)
	if config.DBPath == "" {
		return ErrDatabasePathNotConfigured
	}
	return atomicWriteJSON(path, config)
}

func ResolveDatabasePath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return NormalizeDatabasePath(explicit)
	}
	if envPath := strings.TrimSpace(os.Getenv(DatabasePathEnv)); envPath != "" {
		return NormalizeDatabasePath(envPath)
	}

	config, err := LoadConfig("")
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrDatabasePathNotConfigured
	}
	if err != nil {
		return "", err
	}
	if config.DBPath == "" {
		return "", ErrDatabasePathNotConfigured
	}
	return NormalizeDatabasePath(config.DBPath)
}

func NormalizeDatabasePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if len(path) >= 2 {
		if (path[0] == '"' && path[len(path)-1] == '"') ||
			(path[0] == '\'' && path[len(path)-1] == '\'') {
			path = strings.TrimSpace(path[1 : len(path)-1])
		}
	}
	if path == "" {
		return "", ErrDatabasePathNotConfigured
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolve database path %q: %w", path, err)
		}
		path = absolute
	}
	return path, nil
}

func EnsureDatabaseDirectory(path string) error {
	path, err := NormalizeDatabasePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	return nil
}

func Open(path string) (*Database, Store, error) {
	path, err := NormalizeDatabasePath(path)
	if err != nil {
		return nil, Store{}, err
	}
	if err := EnsureDatabaseDirectory(path); err != nil {
		return nil, Store{}, err
	}

	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, Store{}, &DatabaseLockError{
				DatabasePath: path,
				LockPath:     lockPath,
				Owner:        readLockMetadata(lockPath),
			}
		}
		return nil, Store{}, fmt.Errorf("lock database %s: %w", path, err)
	}

	metadata := LockMetadata{
		PID:       os.Getpid(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		StartedAt: time.Now(),
	}
	metadata.Hostname, _ = os.Hostname()
	if err := json.NewEncoder(lockFile).Encode(metadata); err != nil {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
		return nil, Store{}, fmt.Errorf("write database lock: %w", err)
	}
	if err := lockFile.Sync(); err != nil {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
		return nil, Store{}, fmt.Errorf("flush database lock: %w", err)
	}

	db := &Database{Path: path, lockPath: lockPath, lockFile: lockFile}
	store, err := loadStore(path)
	if err != nil {
		_ = db.Close()
		return nil, Store{}, err
	}
	return db, store, nil
}

func (db *Database) Save(store Store) error {
	if db == nil {
		return errors.New("database is closed")
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.lockFile == nil {
		return errors.New("database is closed")
	}
	return writeStore(db.Path, store)
}

func (db *Database) Close() error {
	if db == nil {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.lockFile == nil {
		return nil
	}
	closeErr := db.lockFile.Close()
	db.lockFile = nil
	removeErr := os.Remove(db.lockPath)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	if closeErr != nil && removeErr != nil {
		return fmt.Errorf("close database lock: %v; remove lock: %w", closeErr, removeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close database lock: %w", closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("remove database lock: %w", removeErr)
	}
	return nil
}

func DefaultPath() (string, error) {
	return ResolveDatabasePath("")
}

// TestPath returns the storage path used by the --test command-line mode.
// It lives alongside the normal task file but is kept separate from it.
func TestPath() (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return TestPathFor(path)
}

// TestPathFor returns the testing storage path alongside path.
func TestPathFor(path string) (string, error) {
	path, err := NormalizeDatabasePath(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "tasks-test.json"), nil
}

func Load(path string) (Store, error) {
	if path == "" {
		var err error
		path, err = ResolveDatabasePath("")
		if err != nil {
			return Store{}, err
		}
	}
	path, err := NormalizeDatabasePath(path)
	if err != nil {
		return Store{}, err
	}
	return loadStore(path)
}

func Save(path string, store Store) error {
	if path == "" {
		var err error
		path, err = ResolveDatabasePath("")
		if err != nil {
			return err
		}
	}
	path, err := NormalizeDatabasePath(path)
	if err != nil {
		return err
	}
	return writeStore(path, store)
}

func loadStore(path string) (Store, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewStore(), nil
	}
	if err != nil {
		return Store{}, err
	}
	var store Store
	if err := json.Unmarshal(raw, &store); err != nil {
		return Store{}, fmt.Errorf("read %s: %w", path, err)
	}
	if store.NextID < 1 {
		store.NextID = 1
	}
	for _, task := range store.Tasks {
		if task.ID >= store.NextID {
			store.NextID = task.ID + 1
		}
	}
	return store, nil
}

func writeStore(path string, store Store) error {
	if err := EnsureDatabaseDirectory(path); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return atomicWrite(path, raw, 0o600)
}

func atomicWriteJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return atomicWrite(path, raw, 0o600)
}

func atomicWrite(path string, raw []byte, mode os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func readLockMetadata(path string) LockMetadata {
	raw, err := os.ReadFile(path)
	if err != nil {
		return LockMetadata{}
	}
	var metadata LockMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return LockMetadata{}
	}
	return metadata
}
