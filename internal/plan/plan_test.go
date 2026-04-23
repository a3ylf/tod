package plan

import (
	"strings"
	"testing"

	"todos/internal/todo"
)

func TestPromptIncludesTaskContext(t *testing.T) {
	task := todo.Task{
		ID:       12,
		Title:    "play the game of life",
		Project:  "Work",
		Due:      "2026-04-24",
		Priority: 3,
		Labels:   []string{"home"},
	}
	got := Prompt(task)
	for _, want := range []string{
		"Do not edit files or run commands",
		"Return only the kickoff plan",
		"The first character of your response must be '#'",
		"# Kickoff Plan",
		"ID: 12",
		"Title: play the game of life",
		"Project: Work",
		"Due: 2026-04-24",
		"Priority: p3",
		"Labels: home",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Prompt missing %q:\n%s", want, got)
		}
	}
}
