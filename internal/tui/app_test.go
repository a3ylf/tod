package tui

import "testing"

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
