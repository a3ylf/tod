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
tab            switch focused side
n              add task
e or enter     open edit mode for the selected task
w              close and print selected task id + title
W              close and run plan mode for selected task
x or space     toggle complete
d              set due date
p              cycle priority p4 -> p3 -> p2 -> p1
P              move to project
L              edit labels
/              search
c              clear search
D              delete, press twice
q              save and quit
```

Due dates accept `today`, `tomorrow`, `+3d`, `yyyy-mm-dd`, or `clear`.

Task text can include components inline. For example:

```text
play the game of life tomorrow p3 #Work @home
```

creates a task named `play the game of life` due tomorrow with priority `p3`,
project `Work`, and label `home`.

In edit mode, use left/right to pick a field, enter to change it, `p` to cycle
priority, `x` to toggle completion, and esc or `e` to close the editor.

## Planning

Press `w` to close the TUI and print the selected task:

```text
12	play the game of life
```

Press `W` to close the TUI and generate the Codex kickoff plan for the selected
task directly.

Generate a Codex kickoff prompt for a task:

```sh
tod --plan 12
```

The planner loads the task from local storage and runs `codex exec -s read-only`
as a prompt maker. It asks Codex to return only the kickoff plan, without
editing files or adding extra commentary.
