# todos

A personal Todoist-style TUI written in Go with Bubble Tea.

## Run

```sh
go run ./cmd/todos
```

Build a local binary:

```sh
go build -o todos ./cmd/todos
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

In edit mode, use up/down to pick a field, enter to change it, `p` to cycle
priority, `x` to toggle completion, and esc or `e` to close the editor.
