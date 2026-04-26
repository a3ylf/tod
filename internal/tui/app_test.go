package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"todos/internal/todo"
)

func TestScrollStart(t *testing.T) {
	tests := []struct {
		selected int
		visible  int
		total    int
		want     int
	}{
		{selected: 0, visible: 5, total: 20, want: 0},
		{selected: 4, visible: 5, total: 20, want: 0},
		{selected: 5, visible: 5, total: 20, want: 1},
		{selected: 19, visible: 5, total: 20, want: 15},
		{selected: 2, visible: 5, total: 3, want: 0},
	}
	for _, test := range tests {
		got := scrollStart(test.selected, test.visible, test.total)
		if got != test.want {
			t.Fatalf("scrollStart(%d, %d, %d) = %d, want %d", test.selected, test.visible, test.total, got, test.want)
		}
	}
}

func TestPaneNavigation(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 3,
			Tasks: []todo.Task{
				{ID: 1, Title: "one", Project: "Inbox", Priority: 4},
				{ID: 2, Title: "two", Project: "Work", Priority: 4},
			},
		},
		view:  "Inbox",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(key("down"))
	m = updated.(model)
	if m.selected != 0 {
		t.Fatalf("task down in one-item Inbox selected %d, want 0", m.selected)
	}
	updated, _ = m.updateNormal(key("right"))
	m = updated.(model)
	if m.focus != paneTasks {
		t.Fatalf("right focus = %v, want tasks", m.focus)
	}
	updated, _ = m.updateNormal(key("left"))
	m = updated.(model)
	if m.focus != paneSidebar {
		t.Fatalf("left focus = %v, want sidebar", m.focus)
	}
	updated, _ = m.updateNormal(key("down"))
	m = updated.(model)
	if m.view != "#Work" {
		t.Fatalf("sidebar down view = %q, want #Work", m.view)
	}
	updated, _ = m.updateNormal(key("right"))
	m = updated.(model)
	if m.focus != paneTasks {
		t.Fatalf("right focus = %v, want tasks", m.focus)
	}
}

func TestInitialModelStartsOnViews(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	m := initialModel(todo.Store{Tasks: []todo.Task{{ID: 1, Title: "due", Project: "Inbox", Due: today, Priority: 4}}}, "/tmp/tasks.json")
	if m.view != "Today" || m.focus != paneSidebar {
		t.Fatalf("initial state = (%q, %v), want Today with views focus", m.view, m.focus)
	}
}

func TestViewsStartWithTodayHideInboxAndZeroCounts(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	m := model{
		store: todo.Store{Tasks: []todo.Task{
			{ID: 1, Title: "due", Project: "Inbox", Due: today, Priority: 4},
			{ID: 2, Title: "plain", Project: "Inbox", Priority: 4},
		}},
	}
	views := m.views()
	if len(views) == 0 || views[0] != "Today" {
		t.Fatalf("views = %v, want Today first", views)
	}
	for _, view := range views {
		if view == "Inbox" || view == "Upcoming" || view == "Completed" {
			t.Fatalf("views = %v, want Inbox and zero-count views hidden", views)
		}
	}
}

func TestViewSeparatesHeaderFromContent(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 1, Title: "one", Project: "Inbox", Priority: 4}},
		},
		view:   "Inbox",
		focus:  paneTasks,
		width:  80,
		height: 20,
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) < 2 || !strings.Contains(lines[1], "---") {
		t.Fatalf("second line = %q, want header separator", lines[1])
	}
}

func TestEditModeTargetsSelectedTask(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 1, Title: "one", Project: "Work", Due: "2026-04-25", Priority: 3, Labels: []string{"focus"}}},
		},
		view:  "All",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(key("e"))
	m = updated.(model)
	if !m.editing || m.editTaskID != 1 || !m.input.active || m.input.kind != "edit" {
		t.Fatalf("edit state = (%t, %d, %t, %q), want active text edit", m.editing, m.editTaskID, m.input.active, m.input.kind)
	}
	if m.input.value != "one p3 2026-04-25 #Work @focus" {
		t.Fatalf("edit value = %q, want full task text", m.input.value)
	}
}

func TestTextEditReplacesTaskFields(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 1, Title: "one", Project: "Work", Due: "2026-04-25", Priority: 3, Labels: []string{"focus"}}},
		},
		view:       "All",
		focus:      paneTasks,
		editing:    true,
		editTaskID: 1,
		input:      inputState{active: true, kind: "edit", title: "Edit", value: "two p1 tomorrow #Home @deep"},
	}
	updated, _ := m.updateInput(key("enter"))
	m = updated.(model)
	task, _ := m.store.Task(1)
	if m.editing || m.input.active {
		t.Fatalf("edit still active after commit")
	}
	if task.Title != "two" || task.Project != "Home" || task.Priority != 1 || task.Due == "" || !reflect.DeepEqual(task.Labels, []string{"deep"}) {
		t.Fatalf("task = %+v, want text edit applied", task)
	}

	m.editing = true
	m.editTaskID = 1
	m.input = inputState{active: true, kind: "edit", title: "Edit", value: "plain"}
	updated, _ = m.updateInput(key("enter"))
	m = updated.(model)
	task, _ = m.store.Task(1)
	if task.Title != "plain" || task.Project != "Inbox" || task.Priority != 4 || task.Due != "" || len(task.Labels) != 0 {
		t.Fatalf("task = %+v, want removed inline metadata cleared", task)
	}
}

func TestInputEditingUsesCursor(t *testing.T) {
	m := model{input: inputState{active: true, kind: "edit", title: "Edit", value: "abcd", cursor: 2}}
	updated, _ := m.updateInput(key("X"))
	m = updated.(model)
	if m.input.value != "abXcd" || m.input.cursor != 3 {
		t.Fatalf("input = (%q, %d), want inserted at cursor", m.input.value, m.input.cursor)
	}

	updated, _ = m.updateInput(key("left"))
	m = updated.(model)
	updated, _ = m.updateInput(key("backspace"))
	m = updated.(model)
	if m.input.value != "aXcd" || m.input.cursor != 1 {
		t.Fatalf("input after cursor backspace = (%q, %d), want aXcd at 1", m.input.value, m.input.cursor)
	}
}

func TestInputSpaceInsertsAndMovesCursor(t *testing.T) {
	m := model{input: inputState{active: true, kind: "new", title: "New task", value: "ab", cursor: 1, selectStart: -1}}
	updated, _ := m.updateInput(key("space"))
	m = updated.(model)
	if m.input.value != "a b" || m.input.cursor != 2 {
		t.Fatalf("input after space = (%q, %d), want a b at 2", m.input.value, m.input.cursor)
	}
	if view := m.inputView(80); !strings.Contains(view, "a b") {
		t.Fatalf("inputView = %q, want visible inserted space", view)
	}
}

func TestUndoRestoresPreviousInputEdit(t *testing.T) {
	m := model{input: inputState{active: true, kind: "edit", title: "Edit", value: "abcd", cursor: 2}}
	updated, _ := m.updateInput(key("X"))
	m = updated.(model)
	updated, _ = m.updateInput(key("Y"))
	m = updated.(model)
	if m.input.value != "abXYcd" || m.input.cursor != 4 {
		t.Fatalf("input = (%q, %d), want two edits", m.input.value, m.input.cursor)
	}
	updated, _ = m.updateInput(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = updated.(model)
	if m.input.value != "abXcd" || m.input.cursor != 3 {
		t.Fatalf("input after first undo = (%q, %d), want first edit state", m.input.value, m.input.cursor)
	}
	updated, _ = m.updateInput(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = updated.(model)
	if m.input.value != "abcd" || m.input.cursor != 2 {
		t.Fatalf("input after second undo = (%q, %d), want original edit state", m.input.value, m.input.cursor)
	}
}

func TestAltDeleteDeletesWordForwardInInput(t *testing.T) {
	m := model{input: inputState{active: true, kind: "edit", title: "Edit", value: "one two three", cursor: 4}}
	updated, _ := m.updateInput(tea.KeyMsg{Type: tea.KeyDelete, Alt: true})
	m = updated.(model)
	if m.input.value != "one three" || m.input.cursor != 4 {
		t.Fatalf("input = (%q, %d), want forward word deleted", m.input.value, m.input.cursor)
	}
}

func TestDeleteDeletesCharacterForwardInInput(t *testing.T) {
	m := model{input: inputState{active: true, kind: "edit", title: "Edit", value: "one two", cursor: 4}}
	updated, _ := m.updateInput(key("delete"))
	m = updated.(model)
	if m.input.value != "one wo" || m.input.cursor != 4 {
		t.Fatalf("input = (%q, %d), want forward character deleted", m.input.value, m.input.cursor)
	}
}

func TestMouseSelectionCanBeDeletedInEditInput(t *testing.T) {
	m := model{height: 12, input: inputState{active: true, kind: "edit", title: "Edit", value: "abcdef", cursor: 6, selectStart: -1}}
	inputTop, _, _ := m.inputLayout()
	updated, _ := m.Update(tea.MouseMsg(tea.MouseEvent{X: 7, Y: inputTop, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	m = updated.(model)
	updated, _ = m.Update(tea.MouseMsg(tea.MouseEvent{X: 10, Y: inputTop, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}))
	m = updated.(model)
	if _, _, ok := m.inputSelectionBounds(); !ok {
		t.Fatalf("expected mouse selection, got input %+v", m.input)
	}
	updated, _ = m.updateInput(key("delete"))
	m = updated.(model)
	if m.input.value != "aef" || m.input.cursor != 1 {
		t.Fatalf("input after deleting selection = (%q, %d), want aef at 1", m.input.value, m.input.cursor)
	}
}

func TestNewAndEditInputColorTokens(t *testing.T) {
	styles := inputTokenStyles("task p3 2026-04-25 #Work @focus")
	for _, index := range []int{5, 8, 19, 25} {
		if _, ok := styles[index]; !ok {
			t.Fatalf("token styles missing index %d: %#v", index, styles)
		}
	}
	projectStyle, _ := inputTokenStyle("#Work")
	labelStyle, _ := inputTokenStyle("@focus")
	dateStyle, _ := inputTokenStyle("2026-04-25")
	priorityInputStyle, _ := inputTokenStyle("p3")
	if projectStyle.GetForeground() == labelStyle.GetForeground() || projectStyle.GetForeground() == dateStyle.GetForeground() || labelStyle.GetForeground() == dateStyle.GetForeground() {
		t.Fatalf("project, label, and date input colors should be distinct")
	}
	if projectStyle.GetForeground() != projectInputStyle.GetForeground() {
		t.Fatalf("project input foreground = %v, want task project foreground %v", projectStyle.GetForeground(), projectInputStyle.GetForeground())
	}
	if labelStyle.GetForeground() != labelInputStyle.GetForeground() {
		t.Fatalf("label input foreground = %v, want task label foreground %v", labelStyle.GetForeground(), labelInputStyle.GetForeground())
	}
	if priorityInputStyle.GetForeground() != priorityStyle(3).GetForeground() {
		t.Fatalf("priority input foreground = %v, want task priority foreground %v", priorityInputStyle.GetForeground(), priorityStyle(3).GetForeground())
	}
	for _, kind := range []string{"new", "edit"} {
		m := model{input: inputState{active: true, kind: kind, title: "Input", value: "task p3 #Work", cursor: len("task p3 #Work"), selectStart: -1}}
		if !m.inputTokenColorEnabled() {
			t.Fatalf("inputTokenColorEnabled false for %s", kind)
		}
	}
}

func TestUndoRestoresPreviousStore(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 1, Title: "one", Project: "Inbox", Priority: 4}},
		},
		view:  "All",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(key("p"))
	m = updated.(model)
	if m.store.Tasks[0].Priority != 3 {
		t.Fatalf("priority = %d, want changed to 3", m.store.Tasks[0].Priority)
	}
	updated, _ = m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = updated.(model)
	if m.store.Tasks[0].Priority != 4 {
		t.Fatalf("priority after undo = %d, want 4", m.store.Tasks[0].Priority)
	}
}

func TestUpDownExitTextEditInput(t *testing.T) {
	for _, keyName := range []string{"up", "down"} {
		m := model{
			editing:    true,
			editTaskID: 1,
			input:      inputState{active: true, kind: "edit", title: "Edit", value: "one", cursor: 3},
		}
		updated, _ := m.updateInput(key(keyName))
		m = updated.(model)
		if m.input.active || m.editing || m.editTaskID != 0 {
			t.Fatalf("%s left state input=%t editing=%t editTaskID=%d, want exited", keyName, m.input.active, m.editing, m.editTaskID)
		}
	}
}

func TestInputMouseMovesCursor(t *testing.T) {
	m := model{height: 12, input: inputState{active: true, kind: "edit", title: "Edit", value: "abcdef", cursor: 6}}
	inputTop, _, _ := m.inputLayout()
	updated, _ := m.Update(tea.MouseMsg(tea.MouseEvent{X: 8, Y: inputTop, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	m = updated.(model)
	if m.input.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", m.input.cursor)
	}
}

func TestInputMouseMovesCursorOnWrappedLine(t *testing.T) {
	m := model{width: 18, height: 12, input: inputState{active: true, kind: "edit", title: "Edit", value: "one two three", cursor: 13}}
	inputTop, _, _ := m.inputLayout()
	updated, _ := m.Update(tea.MouseMsg(tea.MouseEvent{X: 7, Y: inputTop + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	m = updated.(model)
	if m.input.cursor <= len("one two ") {
		t.Fatalf("cursor = %d, want cursor on wrapped line", m.input.cursor)
	}
}

func TestUpDownExitEditMode(t *testing.T) {
	for _, keyName := range []string{"up", "down"} {
		m := model{
			store: todo.Store{
				NextID: 2,
				Tasks:  []todo.Task{{ID: 1, Title: "one", Project: "Inbox", Priority: 4}},
			},
			view:       "Inbox",
			focus:      paneTasks,
			editing:    true,
			editTaskID: 1,
		}
		updated, _ := m.updateEdit(key(keyName))
		m = updated.(model)
		if m.editing || m.editTaskID != 0 {
			t.Fatalf("%s left edit state = (%t, %d), want (false, 0)", keyName, m.editing, m.editTaskID)
		}
	}
}

func TestSearchUpdatesLiveAndSlashEnterClears(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 3,
			Tasks: []todo.Task{
				{ID: 1, Title: "one", Project: "Inbox", Priority: 4},
				{ID: 2, Title: "two", Project: "Work", Priority: 4},
			},
		},
		view:     "All",
		focus:    paneTasks,
		selected: 1,
	}

	updated, _ := m.updateNormal(key("/"))
	m = updated.(model)
	updated, _ = m.updateInput(key("#"))
	m = updated.(model)
	if m.search != "#" {
		t.Fatalf("live search = %q, want #", m.search)
	}
	if m.selected != 0 {
		t.Fatalf("selected after live search = %d, want clamped to 0", m.selected)
	}

	updated, _ = m.updateInput(key("enter"))
	m = updated.(model)
	updated, _ = m.updateNormal(key("/"))
	m = updated.(model)
	updated, _ = m.updateInput(key("enter"))
	m = updated.(model)
	if m.search != "" {
		t.Fatalf("search after slash enter = %q, want empty", m.search)
	}
}

func TestWExportsSelectedTask(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 7, Title: "write prompt", Project: "Inbox", Priority: 4}},
		},
		view:  "Inbox",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(key("w"))
	m = updated.(model)
	if m.exportID != 7 {
		t.Fatalf("exportID = %d, want 7", m.exportID)
	}
	if m.copyOnExit {
		t.Fatal("copyOnExit = true, want false")
	}
}

func TestMultiSelectionExportsSelectedTasks(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 4,
			Tasks: []todo.Task{
				{ID: 1, Title: "one", Project: "Inbox", Priority: 4},
				{ID: 2, Title: "two", Project: "Inbox", Priority: 4},
				{ID: 3, Title: "three", Project: "Inbox", Priority: 4},
			},
		},
		view:  "All",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlDown})
	m = updated.(model)
	updated, _ = m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlDown})
	m = updated.(model)
	if got := taskTitles(m.selectedTasks()); got != "one\ntwo\nthree" {
		t.Fatalf("selected task titles = %q, want three selected tasks", got)
	}

	updated, _ = m.updateNormal(key("w"))
	m = updated.(model)
	if m.exportID != 1 || m.exportTitle != "one\ntwo\nthree" {
		t.Fatalf("export = (%d, %q), want multi export", m.exportID, m.exportTitle)
	}
}

func TestMultiSelectionBoxWrapsSelectedRange(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 4,
			Tasks: []todo.Task{
				{ID: 1, Title: "one", Project: "Inbox", Priority: 4},
				{ID: 2, Title: "two", Project: "Inbox", Priority: 4},
				{ID: 3, Title: "three", Project: "Inbox", Priority: 4},
			},
		},
		view:  "All",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlDown})
	m = updated.(model)
	lines := m.taskListLines(m.filteredTasks(), 48, 8)
	if len(lines) < 4 {
		t.Fatalf("lines = %v, want range box", lines)
	}
	if !strings.Contains(lines[0], "+") || !strings.Contains(lines[3], "+") {
		t.Fatalf("lines = %v, want range box top and bottom", lines)
	}
	if !strings.Contains(lines[1], "one") || !strings.Contains(lines[2], "two") {
		t.Fatalf("lines = %v, want selected tasks inside box", lines)
	}
}

func TestPlainMoveClearsMultiSelection(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 3,
			Tasks: []todo.Task{
				{ID: 1, Title: "one", Project: "Inbox", Priority: 4},
				{ID: 2, Title: "two", Project: "Inbox", Priority: 4},
			},
		},
		view:  "All",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlDown})
	m = updated.(model)
	if len(m.selectedTasks()) != 2 {
		t.Fatalf("selected tasks = %d, want 2", len(m.selectedTasks()))
	}
	updated, _ = m.updateNormal(key("up"))
	m = updated.(model)
	if len(m.selectedTasks()) != 1 {
		t.Fatalf("selected tasks after plain move = %d, want 1", len(m.selectedTasks()))
	}
}

func TestShiftWCopiesSelectedTaskAndQuits(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 7, Title: "write prompt", Project: "Inbox", Priority: 4}},
		},
		view:  "Inbox",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(key("W"))
	m = updated.(model)
	if m.exportID != 7 {
		t.Fatalf("exportID = %d, want 7", m.exportID)
	}
	if !m.copyOnExit {
		t.Fatal("copyOnExit = false, want true")
	}
}

func TestEditDoesNotOpenForMultipleTasks(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 3,
			Tasks: []todo.Task{
				{ID: 1, Title: "one", Project: "Inbox", Priority: 4},
				{ID: 2, Title: "two", Project: "Inbox", Priority: 4},
			},
		},
		view:  "All",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlDown})
	m = updated.(model)
	updated, _ = m.updateNormal(key("e"))
	m = updated.(model)
	if m.editing || m.input.active {
		t.Fatalf("edit state = (%t, %t), want edit disabled for multi-select", m.editing, m.input.active)
	}
}

func TestYCopiesSelectedTaskWithoutQuitting(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 7, Title: "write prompt", Project: "Inbox", Priority: 4}},
		},
		view:  "Inbox",
		focus: paneTasks,
	}
	updated, cmd := m.updateNormal(key("y"))
	m = updated.(model)
	if m.exportID != 0 {
		t.Fatalf("exportID = %d, want 0", m.exportID)
	}
	if cmd == nil {
		t.Fatal("copy command is nil")
	}
}

func TestTaskRowHidesInboxProject(t *testing.T) {
	m := model{selected: 0, focus: paneTasks}
	row := m.taskRow(0, todo.Task{ID: 1, Title: "one", Project: "Inbox", Priority: 4}, 80)
	if strings.Contains(row, "#Inbox") {
		t.Fatalf("task row %q contains #Inbox", row)
	}

	row = m.taskRow(0, todo.Task{ID: 1, Title: "one", Project: "Work", Priority: 4}, 80)
	if !strings.Contains(row, "#Work") {
		t.Fatalf("task row %q does not contain #Work", row)
	}
}

func TestSelectedTaskBoxWrapsLongTitle(t *testing.T) {
	m := model{selected: 0, focus: paneTasks}
	task := todo.Task{
		ID:       1,
		Title:    "this is a very long task title that should wrap inside the selected box instead of breaking the interface",
		Project:  "Work",
		Priority: 4,
		Due:      "2026-04-25",
	}
	lines := m.selectedTaskBox(0, task, 42)
	if len(lines) < 5 {
		t.Fatalf("selected box lines = %d, want wrapped content", len(lines))
	}
	for _, line := range lines {
		if got := ansi.StringWidth(line); got > 42 {
			t.Fatalf("line width = %d, want <= 42: %q", got, line)
		}
	}
}

func TestSelectedTaskBoxKeepsMetadataInlineWhenItFits(t *testing.T) {
	m := model{selected: 0, focus: paneTasks}
	task := todo.Task{ID: 1, Title: "MCP learn", Project: "code", Priority: 3, Due: "2026-04-24"}
	lines := m.selectedTaskBox(0, task, 96)
	if len(lines) != 3 {
		t.Fatalf("selected box lines = %d, want single content line", len(lines))
	}
	if !strings.Contains(lines[1], "2026-04-24") || !strings.Contains(lines[1], "#code") {
		t.Fatalf("content line missing metadata: %q", lines[1])
	}
}

func TestSelectedTaskBoxBorderUsesPriorityColor(t *testing.T) {
	m := model{selected: 0, focus: paneTasks}
	task := todo.Task{ID: 1, Title: "urgent", Project: "Inbox", Priority: 1}
	got := m.singleTaskBorderStyle(task).GetForeground()
	want := priorityStyle(1).GetForeground()
	if got != want {
		t.Fatalf("border foreground = %v, want %v", got, want)
	}
}

func TestMultiSelectionUsesDedicatedBorderColor(t *testing.T) {
	m := model{selected: 1, selectFrom: 0, focus: paneTasks}
	got := m.multiTaskBorderStyle().GetForeground()
	want := selectedBorderStyle.GetForeground()
	if got != want {
		t.Fatalf("multi border foreground = %v, want %v", got, want)
	}
}

func TestDueBadgeRelativeColors(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name  string
		due   string
		label string
		style lipgloss.Style
	}{
		{name: "past", due: "2026-04-25", label: "2026-04-25", style: warnStyle},
		{name: "today", due: "2026-04-26", label: "today", style: todayDateStyle},
		{name: "tomorrow", due: "2026-04-27", label: "tomorrow", style: dateInputStyle},
		{name: "future", due: "2026-04-28", label: "2026-04-28", style: futureDateStyle},
	}
	for _, tt := range tests {
		task := todo.Task{Due: tt.due}
		if got := dueBadgeAt(task, true, now); got != tt.label {
			t.Fatalf("%s plain badge = %q, want %q", tt.name, got, tt.label)
		}
		if got, want := dueStyleAt(startOfDay(now), now).GetForeground(), todayDateStyle.GetForeground(); got != want {
			t.Fatalf("today foreground = %v, want %v", got, want)
		}
		d, ok := task.DueTimeAt(now)
		if !ok {
			t.Fatalf("%s due did not parse", tt.name)
		}
		if got, want := dueStyleAt(d, now).GetForeground(), tt.style.GetForeground(); got != want {
			t.Fatalf("%s foreground = %v, want %v", tt.name, got, want)
		}
	}
}

func TestInputViewWrapsLongEditText(t *testing.T) {
	m := model{
		input: inputState{
			active: true,
			kind:   "edit",
			title:  "Edit",
			value:  "make it nvim like so slash project and labels can be edited in one place p3 2026-04-25 #Work @focus",
		},
	}
	for _, line := range strings.Split(m.inputView(44), "\n") {
		if got := ansi.StringWidth(line); got > 44 {
			t.Fatalf("line width = %d, want <= 44: %q", got, line)
		}
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
