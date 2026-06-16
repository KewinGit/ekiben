package ui

import "testing"

func TestColumns(t *testing.T) {
	cases := []struct{ width, want int }{
		{10, 1}, // narrower than a card
		{20, 1},
		{41, 2},   // (41+1)/21 = 2
		{63, 3},   // (63+1)/21 = 3
		{84, 4},   // (84+1)/21 = 4
		{210, 10}, // (210+1)/21 = 10
	}
	for _, c := range cases {
		if got := Columns(c.width); got != c.want {
			t.Errorf("Columns(%d)=%d want %d", c.width, got, c.want)
		}
	}
}

func TestCardWidthFillsAvailable(t *testing.T) {
	// 3 columns over width 65 with 1-col gaps: (65 - 2)/3 = 21
	if got := CardWidth(65, 3); got != 21 {
		t.Fatalf("CardWidth(65,3)=%d want 21", got)
	}
}
