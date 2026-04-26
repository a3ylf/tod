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
	if got := stdout.String(); got != "Copied task: write docs\n" {
		t.Fatalf("stdout = %q, want copied message", got)
	}
}
