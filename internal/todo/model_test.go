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

func ids(tasks []Task) []int {
	out := make([]int, len(tasks))
	for i, task := range tasks {
		out[i] = task.ID
	}
	return out
}
