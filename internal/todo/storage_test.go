package todo

import (
	"path/filepath"
	"testing"
)

func TestTestPathUsesSeparateFileBesideDefaultPath(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	defaultPath, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath returned error: %v", err)
	}
	testPath, err := TestPath()
	if err != nil {
		t.Fatalf("TestPath returned error: %v", err)
	}

	if got, want := testPath, filepath.Join(filepath.Dir(defaultPath), "tasks-test.json"); got != want {
		t.Fatalf("TestPath = %q, want %q", got, want)
	}
	if testPath == defaultPath {
		t.Fatalf("TestPath = %q, want a path separate from DefaultPath", testPath)
	}
}
