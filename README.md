# todos

A personal Todoist-style TUI written in Go with Bubble Tea.

## Run

```sh
go run ./cmd/todos
```

Use the separate testing task database when trying changes:

```sh
go run ./cmd/todos --test
```

Build a local binary:

```sh
go build -o tod ./cmd/tod
```

Build the Windows binary from Linux or WSL:

```sh
GOOS=windows GOARCH=amd64 go build -o tod.exe ./cmd/tod
```

Install the global `tod` command:

```sh
go install ./cmd/tod
```

## Database setup

Tasks are stored as JSON at a path configured for each environment. The path
is selected in this order:

1. `--db PATH`
2. `TODOS_DB_PATH`
3. the per-user config file
4. an interactive first-run prompt

The config file is stored at:

```text
Linux/WSL: ~/.config/todos/config.json
Windows:   %APPDATA%\todos\config.json
```

On first launch, enter a path that both environments can access. For example,
the same Windows file can be configured as:

```text
Windows: C:\Users\alex\AppData\Roaming\todos\tasks.json
WSL:     /mnt/c/Users/alex/AppData/Roaming/todos/tasks.json
```

You can also provide the path for one run:

```sh
tod --db /mnt/c/Users/alex/AppData/Roaming/todos/tasks.json
```

The database is exclusively locked while the TUI is open. A second instance
using the same file exits with a database-in-use message. If the process is
terminated unexpectedly, remove the matching `.lock` file manually after
confirming that no `tod` instance is still running.

The `--test` mode uses `tasks-test.json` in the same directory.

## Keys

```text
left/right     switch between sidebar and tasks
up/down        move within the focused side
ctrl+up/down   select multiple tasks in the task list
tab            switch focused side
n              add task
e or enter     edit the selected task as text
y              copy selected task text to clipboard
w              close and print selected task text
W              copy selected task text to clipboard and quit
x or space     toggle complete
d              set due date
p              cycle priority p4 -> p3 -> p2 -> p1
P              move to project
L              edit labels
/              search
c              clear search
D              delete, press twice
ctrl+z or u    undo last task change
q              save and quit
```

Due dates accept `today`, `tomorrow`, `+3d`, `yyyy-mm-dd`, or `clear`.

Task text can include components inline. For example:

```text
play the game of life tomorrow p3 #Work @home
```

creates a task named `play the game of life` due tomorrow with priority `p3`,
project `Work`, and label `home`.

Edit mode opens the whole task as one text buffer. Use inline tokens like
`p3`, `2026-04-25`, `#Work`, and `@home`; removing a token clears that field.
Enter applies the edit, esc cancels, and up/down exits edit mode. While editing,
left/right move by character, alt-left/alt-right move by word, delete removes one
character forward, alt-delete or alt-d deletes one word forward, ctrl-w deletes
one word backward, ctrl-u clears before the cursor, ctrl-k clears after the
cursor, and ctrl-z or `u` walks back through text edits.

## Exporting

Press `w` to close the TUI and print the selected task:

```text
play the game of life
```

Press `y` to copy the selected task text to the terminal clipboard without
leaving the TUI. Press `W` to copy the selected task text, quit, and print
`Copied task:` or `Copied tasks:` followed by the copied text on the next line.
Clipboard copy uses
`clip.exe` on WSL/Windows when available, then falls back to OSC52 for terminals
that support it.

Use ctrl-up/ctrl-down in the task list to select a contiguous range. `y`, `w`,
and `W` use all selected tasks joined with newlines. Editing is disabled while
multiple tasks are selected.
