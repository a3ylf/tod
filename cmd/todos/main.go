package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"todos/internal/plan"
	"todos/internal/todo"
	"todos/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	planID := flag.Int("plan", 0, "ask Codex to generate a kickoff prompt for a task id")
	flag.Parse()
	if *planID != 0 {
		return runPlan(*planID)
	}
	if flag.NArg() > 0 {
		id, err := strconv.Atoi(flag.Arg(0))
		if err == nil {
			return runPlan(id)
		}
		return fmt.Errorf("unknown argument: %s", flag.Arg(0))
	}
	exported, err := tui.Run()
	if err != nil {
		return err
	}
	if exported != nil {
		if exported.RunPlan {
			return runPlan(exported.ID)
		}
		fmt.Printf("%d\t%s\n", exported.ID, exported.Title)
	}
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
