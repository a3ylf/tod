package cli

import (
	"bytes"
	"strings"
	"testing"

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
	if got := stderr.String(); !strings.Contains(got, "-test") {
		t.Fatalf("stderr = %q, want test database flag", got)
	}
}

func TestRunRecognizesTestFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run("tod", []string{"--test", "extra"}, &stdout, &stderr)
	if err == nil || err.Error() != "unknown argument: extra" {
		t.Fatalf("Run error = %v, want unknown argument after recognized --test flag", err)
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
