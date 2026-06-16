package docker

import (
	"math"
	"testing"
)

func TestCPUPercent(t *testing.T) {
	// cpu delta 200, system delta 1000, 4 online CPUs -> 0.2*4*100 = 80%
	got := CPUPercent(1200, 1000, 11000, 10000, 4)
	if math.Abs(got-80.0) > 0.001 {
		t.Fatalf("got %v want 80", got)
	}
}

func TestCPUPercentZeroSystemDelta(t *testing.T) {
	if got := CPUPercent(100, 100, 500, 500, 4); got != 0 {
		t.Fatalf("got %v want 0", got)
	}
}
