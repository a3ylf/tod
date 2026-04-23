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
j/k or arrows  move through tasks
tab or l       next view
h             previous view
n             add task
e or enter     edit title
x or space     toggle complete
d             set due date
p             cycle priority p4 -> p3 -> p2 -> p1
P             move to project
L             edit labels
/             search
c             clear search
D             delete, press twice
q             save and quit
```

Due dates accept `today`, `tomorrow`, `+3d`, `yyyy-mm-dd`, or `clear`.
