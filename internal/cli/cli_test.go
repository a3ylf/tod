package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"todos/internal/todo"
	"todos/internal/tui"
)

func TestRunHelpUsesCommandNameAndSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run("tod", []string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Usage of tod:") {
		t.Fatalf("stderr = %q, want usage for tod", got)
	}
	if got := stderr.String(); !strings.Contains(got, "-db") {
		t.Fatalf("stderr = %q, want --db flag in help", got)
	}
	if got := stderr.String(); !strings.Contains(got, "TODOS_DB_PATH") {
		t.Fatalf("stderr = %q, want TODOS_DB_PATH in help", got)
	}
}

func TestConfigureDatabasePathPersistsNormalizedPath(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)
	databasePath := filepath.Join(t.TempDir(), "shared", "tasks.json")
	var stdout bytes.Buffer
	got, err := configureDatabasePath(strings.NewReader("\""+databasePath+"\"\n"), &stdout)
	if err != nil {
		t.Fatalf("configureDatabasePath returned error: %v", err)
	}
	if got != databasePath {
		t.Fatalf("configured path = %q, want %q", got, databasePath)
	}
	resolved, err := todo.ResolveDatabasePath("")
	if err != nil {
		t.Fatalf("ResolveDatabasePath: %v", err)
	}
	if resolved != databasePath {
		t.Fatalf("resolved path = %q, want %q", resolved, databasePath)
	}
	if !strings.Contains(stdout.String(), "Saved database configuration") {
		t.Fatalf("setup output = %q, want save confirmation", stdout.String())
	}
}

func TestConfigureDatabasePathRejectsEmptyEOF(t *testing.T) {
	var stdout bytes.Buffer
	_, err := configureDatabasePath(strings.NewReader(""), &stdout)
	if !errors.Is(err, todo.ErrDatabasePathNotConfigured) && !strings.Contains(err.Error(), "database path is required") {
		t.Fatalf("configureDatabasePath error = %v, want required path error", err)
	}
}

func TestHandleExportedTaskPrintsTitleOnly(t *testing.T) {
	var stdout bytes.Buffer
	err := handleExportedTask(&stdout, &tui.ExportedTask{ID: 42, Title: "write docs"})
	if err != nil {
		t.Fatalf("handleExportedTask returned error: %v", err)
	}
	if got := stdout.String(); got != "write docs\n" {
		t.Fatalf("stdout = %q, want title only", got)
	}
}

func TestHandleCopiedExportedTaskPrintsCopiedMessage(t *testing.T) {
	var stdout bytes.Buffer
	err := handleExportedTask(&stdout, &tui.ExportedTask{ID: 42, Title: "write docs", Copied: true})
	if err != nil {
		t.Fatalf("handleExportedTask returned error: %v", err)
	}
	if got := stdout.String(); got != "Copied task:\nwrite docs\n" {
		t.Fatalf("stdout = %q, want copied message", got)
	}
}

func TestHandleCopiedExportedTasksPrintsPluralMessage(t *testing.T) {
	var stdout bytes.Buffer
	err := handleExportedTask(&stdout, &tui.ExportedTask{Title: "one\ntwo", Copied: true, Count: 2})
	if err != nil {
		t.Fatalf("handleExportedTask returned error: %v", err)
	}
	if got := stdout.String(); got != "Copied tasks:\none\ntwo\n" {
		t.Fatalf("stdout = %q, want copied tasks message", got)
	}
}
