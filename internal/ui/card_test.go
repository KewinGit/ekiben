package ui

import (
	"strings"
	"testing"

	"github.com/KewinGit/ekiben/internal/docker"
)

func TestCardShowsNameStatusAndFields(t *testing.T) {
	c := docker.Container{Name: "postgres", Status: docker.StatusUp, Health: docker.HealthHealthy, Ports: []string{":5432"}}
	st := docker.Stats{CPUPerc: 2.9, MemUsage: 134217728, MemLimit: 1 << 35}
	out := RenderCard(CardInput{
		Container: c, Stats: st, History: []float64{1, 2, 3},
		Fields: []string{"status", "health", "cpu", "mem", "net", "port"},
		Width:  20, Theme: ThemeByName("dark"),
	})
	for _, want := range []string{"postgres", "healthy", "cpu", "mem", "128M", ":5432"} {
		if !strings.Contains(out, want) {
			t.Errorf("card missing %q in:\n%s", want, out)
		}
	}
}

func TestCardOmitsDisabledFields(t *testing.T) {
	c := docker.Container{Name: "x", Status: docker.StatusUp}
	out := RenderCard(CardInput{
		Container: c, Fields: []string{"status"}, Width: 20, Theme: ThemeByName("dark"),
	})
	if strings.Contains(out, "cpu") || strings.Contains(out, "mem") {
		t.Errorf("disabled fields should not render:\n%s", out)
	}
}
