package ui

import (
	"testing"

	"github.com/KewinGit/ekiben/internal/docker"
)

func TestIngestStatsUpdatesHistory(t *testing.T) {
	m := newTestModel()
	m.Update(statsMsg{"1": {CPUPerc: 5}})
	if m.stats["1"].CPUPerc != 5 {
		t.Fatalf("stats not stored")
	}
	if rb := m.history["1"]; rb == nil || len(rb.Values()) != 1 {
		t.Fatalf("history not updated")
	}
	_ = docker.StatusUp
}
