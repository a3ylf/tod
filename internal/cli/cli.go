package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"todos/internal/todo"
	"todos/internal/tui"
)

// Run executes the todos command line interface.
func Run(name string, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	test := flags.Bool("test", false, "use the testing database")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown argument: %s", flags.Arg(0))
	}
	path := ""
	if *test {
		var err error
		path, err = todo.TestPath()
		if err != nil {
			return err
		}
	}
	exported, err := tui.RunWithPath(path)
	if err != nil {
		return err
	}
	if exported != nil {
		return handleExportedTask(stdout, exported)
	}
	return nil
}

func handleExportedTask(stdout io.Writer, exported *tui.ExportedTask) error {
	if exported.Copied {
		label := "task"
		if exported.Count > 1 {
			label = "tasks"
		}
		fmt.Fprintf(stdout, "Copied %s:\n%s\n", label, exported.Title)
		return nil
	}
	fmt.Fprintln(stdout, exported.Title)
	return nil
}
