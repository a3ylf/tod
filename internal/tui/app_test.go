package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	if m.view != "Today" {
		t.Fatalf("sidebar down view = %q, want Today", m.view)
	}
	updated, _ = m.updateNormal(key("right"))
	m = updated.(model)
	if m.focus != paneTasks {
		t.Fatalf("right focus = %v, want tasks", m.focus)
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

func TestCtrlDeleteSequenceDeletesWordForward(t *testing.T) {
	m := model{input: inputState{active: true, kind: "edit", title: "Edit", value: "one two three", cursor: 4}}
	updated, _ := m.Update(csiMsg("?CSI[51 59 53 126]?"))
	m = updated.(model)
	if m.input.value != "one three" || m.input.cursor != 4 {
		t.Fatalf("input = (%q, %d), want forward word deleted", m.input.value, m.input.cursor)
	}
}

func TestCtrlDDeletesWordForwardInInput(t *testing.T) {
	m := model{input: inputState{active: true, kind: "edit", title: "Edit", value: "one two three", cursor: 4}}
	updated, _ := m.updateInput(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(model)
	if m.input.value != "one three" || m.input.cursor != 4 {
		t.Fatalf("input = (%q, %d), want forward word deleted", m.input.value, m.input.cursor)
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
	if m.exportPlan {
		t.Fatal("exportPlan = true, want false")
	}
}

func TestShiftWExportsSelectedTaskForPlan(t *testing.T) {
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
	if !m.exportPlan {
		t.Fatal("exportPlan = false, want true")
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
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

type csiMsg string

func (m csiMsg) String() string {
	return string(m)
}
