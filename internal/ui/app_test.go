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
	m.cfgPath = "" // never write the real config from tests
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

func TestHomeTabCycles(t *testing.T) {
	m := newTestModel()
	if m.homeTab != homeContainers {
		t.Fatalf("initial homeTab = %d, want homeContainers(%d)", m.homeTab, homeContainers)
	}
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	m.Update(tabKey)
	if m.homeTab != homeImages {
		t.Fatalf("after 1 tab: homeTab = %d, want homeImages(%d)", m.homeTab, homeImages)
	}
	m.Update(tabKey)
	if m.homeTab != homeVolumes {
		t.Fatalf("after 2 tabs: homeTab = %d, want homeVolumes(%d)", m.homeTab, homeVolumes)
	}
	m.Update(tabKey)
	if m.homeTab != homeNetworks {
		t.Fatalf("after 3 tabs: homeTab = %d, want homeNetworks(%d)", m.homeTab, homeNetworks)
	}
	m.Update(tabKey)
	if m.homeTab != homeInfo {
		t.Fatalf("after 4 tabs: homeTab = %d, want homeInfo(%d)", m.homeTab, homeInfo)
	}
	m.Update(tabKey)
	if m.homeTab != homeContainers {
		t.Fatalf("after 5 tabs (wrap): homeTab = %d, want homeContainers(%d)", m.homeTab, homeContainers)
	}
}

func TestHomeTabDirectKeys(t *testing.T) {
	m := newTestModel()
	rune2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	m.Update(rune2)
	if m.homeTab != homeImages {
		t.Fatalf("key '2': homeTab = %d, want homeImages(%d)", m.homeTab, homeImages)
	}
	rune3 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	m.Update(rune3)
	if m.homeTab != homeVolumes {
		t.Fatalf("key '3': homeTab = %d, want homeVolumes(%d)", m.homeTab, homeVolumes)
	}
	rune1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
	m.Update(rune1)
	if m.homeTab != homeContainers {
		t.Fatalf("key '1': homeTab = %d, want homeContainers(%d)", m.homeTab, homeContainers)
	}
	rune4 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}}
	m.Update(rune4)
	if m.homeTab != homeNetworks {
		t.Fatalf("key '4': homeTab = %d, want homeNetworks(%d)", m.homeTab, homeNetworks)
	}
	rune5 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	m.Update(rune5)
	if m.homeTab != homeInfo {
		t.Fatalf("key '5': homeTab = %d, want homeInfo(%d)", m.homeTab, homeInfo)
	}
}
