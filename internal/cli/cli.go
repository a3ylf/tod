package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"todos/internal/plan"
	"todos/internal/todo"
	"todos/internal/tui"
)

// Run executes the todos command line interface.
func Run(name string, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	planID := flags.Int("plan", 0, "ask Codex to generate a kickoff prompt for a task id")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *planID != 0 {
		return runPlan(*planID)
	}
	if flags.NArg() > 0 {
		id, err := strconv.Atoi(flags.Arg(0))
		if err == nil {
			return runPlan(id)
		}
		return fmt.Errorf("unknown argument: %s", flags.Arg(0))
	}
	exported, err := tui.Run()
	if err != nil {
		return err
	}
	if exported != nil {
		return handleExportedTask(stdout, exported)
	}
	return nil
}

func handleExportedTask(stdout io.Writer, exported *tui.ExportedTask) error {
	if exported.RunPlan {
		return runPlan(exported.ID)
	}
	fmt.Fprintln(stdout, exported.Title)
	return nil
}

func runPlan(id int) error {
	if id <= 0 {
		return errors.New("task id must be positive")
	}
	path, err := todo.DefaultPath()
	if err != nil {
		return err
	}
	store, err := todo.Load(path)
	if err != nil {
		return err
	}
	task, ok := store.Task(id)
	if !ok {
		return fmt.Errorf("task %d not found", id)
	}
	workspace, _ := os.Getwd()
	return plan.RunCodex(context.Background(), *task, workspace)
}
