package plan

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"todos/internal/todo"
)

func Prompt(task todo.Task) string {
	var b strings.Builder
	b.WriteString("You are a prompt maker. Do not edit files or run commands.\n")
	b.WriteString("Return only the kickoff plan. Do not include a preface, explanation, sign-off, or code fence.\n")
	b.WriteString("The first character of your response must be '#'.\n\n")
	b.WriteString("# Kickoff Plan\n\n")
	b.WriteString("Include exactly these sections:\n")
	b.WriteString("## Goal\n")
	b.WriteString("## Implementation Plan\n")
	b.WriteString("## Files To Inspect First\n")
	b.WriteString("## Verification\n")
	b.WriteString("## Assumptions And Questions\n\n")
	b.WriteString("Task:\n")
	b.WriteString(fmt.Sprintf("ID: %d\n", task.ID))
	b.WriteString(fmt.Sprintf("Title: %s\n", task.Title))
	b.WriteString(fmt.Sprintf("Project: %s\n", task.Project))
	if task.Due != "" {
		b.WriteString(fmt.Sprintf("Due: %s\n", task.Due))
	}
	b.WriteString(fmt.Sprintf("Priority: p%d\n", task.Priority))
	if len(task.Labels) > 0 {
		b.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(task.Labels, ", ")))
	}
	return b.String()
}

func RunCodex(ctx context.Context, task todo.Task, workspace string) error {
	args := []string{"exec", "-s", "read-only"}
	if workspace != "" {
		args = append(args, "-C", workspace)
	}
	args = append(args, Prompt(task))
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
