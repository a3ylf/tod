# todos

A personal Todoist-style TUI written in Go with Bubble Tea.

## Run

```sh
go run ./cmd/todos
```

Build a local binary:

```sh
go build -o tod ./cmd/tod
```

Install the global `tod` command:

```sh
go install ./cmd/tod
```

Tasks are stored as JSON at:

```text
$XDG_DATA_HOME/todos/tasks.json
```

If `XDG_DATA_HOME` is not set, the app uses:

```text
~/.local/share/todos/tasks.json
```

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
`Copied task: <task>` or `Copied tasks: <tasks>`. Clipboard copy uses
`clip.exe` on WSL/Windows when available, then falls back to OSC52 for terminals
that support it.

Use ctrl-up/ctrl-down in the task list to select a contiguous range. `y`, `w`,
and `W` use all selected tasks joined with newlines. Editing is disabled while
multiple tasks are selected.

## Planning

Generate a Codex kickoff prompt for a task:

```sh
tod --plan 12
```

The planner loads the task from local storage and runs `codex exec -s read-only`
as a prompt maker. It asks Codex to return only the kickoff plan, without
editing files or adding extra commentary.
