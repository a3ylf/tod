package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"todos/internal/todo"
	"todos/internal/tui"
)

// Run executes the todos command line interface.
func Run(name string, args []string, stdout, stderr io.Writer) error {
	return run(name, args, os.Stdin, stdout, stderr)
}

func run(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "path to the shared database file")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage of %s:\n", name)
		flags.PrintDefaults()
		fmt.Fprintln(stderr, "\nDatabase path precedence: --db, TODOS_DB_PATH, config file, first-run prompt.")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown argument: %s", flags.Arg(0))
	}
	path, err := todo.ResolveDatabasePath(*dbPath)
	if errors.Is(err, todo.ErrDatabasePathNotConfigured) {
		path, err = configureDatabasePath(stdin, stdout)
	}
	if err != nil {
		return err
	}
	exported, err := tui.Run(path)
	if err != nil {
		return err
	}
	if exported != nil {
		return handleExportedTask(stdout, exported)
	}
	return nil
}

func configureDatabasePath(stdin io.Reader, stdout io.Writer) (string, error) {
	configPath, err := todo.ConfigPath()
	if err != nil {
		return "", err
	}
	fmt.Fprintln(stdout, "No database path is configured yet.")
	fmt.Fprintln(stdout, "Enter an absolute path shared by Windows and WSL (for example, /mnt/c/Users/<name>/AppData/Roaming/todos/tasks.json).")
	fmt.Fprint(stdout, "Database path: ")

	line, readErr := bufio.NewReader(stdin).ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("read database path: %w", readErr)
	}
	path, err := todo.NormalizeDatabasePath(strings.TrimSpace(line))
	if err != nil {
		if errors.Is(err, todo.ErrDatabasePathNotConfigured) {
			return "", fmt.Errorf("database path is required")
		}
		return "", err
	}
	if err := todo.EnsureDatabaseDirectory(path); err != nil {
		return "", err
	}
	if err := todo.SaveConfig(configPath, todo.Config{DBPath: path}); err != nil {
		return "", fmt.Errorf("save database configuration: %w", err)
	}
	fmt.Fprintln(stdout, "Saved database configuration to "+configPath)
	return path, nil
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
