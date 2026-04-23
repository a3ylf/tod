package todo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func DefaultPath() (string, error) {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "todos", "tasks.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "todos", "tasks.json"), nil
}

func Load(path string) (Store, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Store{}, err
		}
	}
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

func Save(path string, store Store) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
