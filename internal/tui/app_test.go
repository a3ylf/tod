package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestEditModeTargetsSelectedTask(t *testing.T) {
	m := model{
		store: todo.Store{
			NextID: 2,
			Tasks:  []todo.Task{{ID: 1, Title: "one", Project: "Inbox", Priority: 4}},
		},
		view:  "Inbox",
		focus: paneTasks,
	}
	updated, _ := m.updateNormal(key("e"))
	m = updated.(model)
	if !m.editing || m.editTaskID != 1 {
		t.Fatalf("edit state = (%t, %d), want (true, 1)", m.editing, m.editTaskID)
	}
	updated, _ = m.updateEdit(key("down"))
	m = updated.(model)
	if m.editField != 1 {
		t.Fatalf("edit field = %d, want 1", m.editField)
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
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
