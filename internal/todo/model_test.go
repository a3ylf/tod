package todo

import (
	"reflect"
	"testing"
	"time"
)

func TestNormalizeDue(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tests := map[string]string{
		"today":      "2026-04-23",
		"tomorrow":   "2026-04-24",
		"+3d":        "2026-04-26",
		"2026-05-01": "2026-05-01",
		"clear":      "",
	}
	for input, want := range tests {
		got, err := NormalizeDue(input, now)
		if err != nil {
			t.Fatalf("NormalizeDue(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeDue(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFilterViewsAndSorting(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tasks := []Task{
		{ID: 1, Title: "later", Project: "Home", Due: "2026-04-25", Priority: 4},
		{ID: 2, Title: "urgent", Project: "Inbox", Due: "2026-04-23", Priority: 1},
		{ID: 3, Title: "done", Project: "Inbox", Due: "2026-04-23", Priority: 1, Completed: true},
		{ID: 4, Title: "overdue", Project: "Work", Due: "2026-04-22", Priority: 2},
	}
	got := Filter(tasks, "Today", "", now)
	wantIDs := []int{4, 2}
	if ids(got) != nil && !reflect.DeepEqual(ids(got), wantIDs) {
		t.Fatalf("Today ids = %v, want %v", ids(got), wantIDs)
	}
	got = Filter(tasks, "Completed", "", now)
	if !reflect.DeepEqual(ids(got), []int{3}) {
		t.Fatalf("Completed ids = %v, want [3]", ids(got))
	}
	got = Filter(tasks, "#Home", "", now)
	if !reflect.DeepEqual(ids(got), []int{1}) {
		t.Fatalf("#Home ids = %v, want [1]", ids(got))
	}
}

func TestCleanLabels(t *testing.T) {
	got := CleanLabels("@home, errand home")
	want := []string{"errand", "home"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CleanLabels = %v, want %v", got, want)
	}
}

func TestParseTaskTextComponents(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	got := ParseTaskText("play the game of life tomorrow p3 #Work @home @fun", now)
	if got.Title != "play the game of life" {
		t.Fatalf("Title = %q, want play the game of life", got.Title)
	}
	if got.Project != "Work" || !got.HasProject {
		t.Fatalf("Project = (%q, %t), want (Work, true)", got.Project, got.HasProject)
	}
	if got.Due != "2026-04-24" || !got.HasDue {
		t.Fatalf("Due = (%q, %t), want (2026-04-24, true)", got.Due, got.HasDue)
	}
	if got.Priority != 3 || !got.HasPriority {
		t.Fatalf("Priority = (%d, %t), want (3, true)", got.Priority, got.HasPriority)
	}
	if !reflect.DeepEqual(got.Labels, []string{"fun", "home"}) {
		t.Fatalf("Labels = %v, want [fun home]", got.Labels)
	}
}

func TestStoreAddParsesTaskText(t *testing.T) {
	var store = NewStore()
	id := store.Add("make it work today p1 #Work @focus", "Inbox")
	if id == 0 {
		t.Fatal("Add returned 0")
	}
	task := store.Tasks[0]
	if task.Title != "make it work" || task.Project != "Work" || task.Priority != 1 || task.Due == "" {
		t.Fatalf("task = %+v, want parsed title/project/priority/due", task)
	}
	if !reflect.DeepEqual(task.Labels, []string{"focus"}) {
		t.Fatalf("Labels = %v, want [focus]", task.Labels)
	}
}

func TestApplyTaskTextPreservesUnspecifiedComponents(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	task := Task{Title: "old", Project: "Inbox", Due: "2026-04-23", Priority: 1, Labels: []string{"old"}}
	if !ApplyTaskText(&task, "new title p4", now) {
		t.Fatal("ApplyTaskText returned false")
	}
	if task.Title != "new title" || task.Priority != 4 {
		t.Fatalf("task = %+v, want updated title and priority", task)
	}
	if task.Project != "Inbox" || task.Due != "2026-04-23" || !reflect.DeepEqual(task.Labels, []string{"old"}) {
		t.Fatalf("task = %+v, want unspecified metadata preserved", task)
	}
}

func ids(tasks []Task) []int {
	out := make([]int, len(tasks))
	for i, task := range tasks {
		out[i] = task.ID
	}
	return out
}
