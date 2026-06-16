package ui

import "testing"

func TestSparklineRange(t *testing.T) {
	got := Sparkline([]float64{0, 50, 100}, 100)
	want := "▁▄█"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSparklineEmpty(t *testing.T) {
	if got := Sparkline(nil, 100); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestSparklineAutoMax(t *testing.T) {
	// max<=0 -> use data max; here data max is 8 so 8 maps to full block
	got := Sparkline([]float64{0, 8}, 0)
	if []rune(got)[1] != '█' {
		t.Fatalf("auto-max top should be full block, got %q", got)
	}
}
