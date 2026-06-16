package ui

import (
	"testing"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() *Model {
	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
		{ID: "2", Name: "b", Project: "p", Status: docker.StatusUp},
		{ID: "3", Name: "c", Project: "p", Status: docker.StatusUp},
	}
	return newTestModelFromFake(docker.NewFake(cs), cs)
}

func newTestModelFromFake(fake *docker.Fake, cs []docker.Container) *Model {
	m := New(fake, config.Default())
	m.applyContainers(cs)
	m.width, m.height = 100, 40
	m.recomputeLayout()
	return m
}

func TestNavigationRight(t *testing.T) {
	m := newTestModel()
	if m.SelectedID() != "1" {
		t.Fatalf("initial selection = %q", m.SelectedID())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.SelectedID() != "2" {
		t.Fatalf("after right = %q want 2", m.SelectedID())
	}
}

func TestNavigationClampsAtEnd(t *testing.T) {
	m := newTestModel()
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // past the end
	if m.SelectedID() != "3" {
		t.Fatalf("selection should clamp to last, got %q", m.SelectedID())
	}
}
