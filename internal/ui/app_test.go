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

// newGridTestModel builds a model with a ragged, multi-group layout and renders
// once so card geometry (m.cardRects) is populated. With width 100 the layout is:
//
//	group alpha (4 cols): [a1 a2 a3 a4] / [a5]
//	group bravo (3 cols): [b1 b2 b3]
//
// The two groups get different column counts (bravo names are longer), so a fixed
// flat stride cannot describe vertical moves correctly.
func newGridTestModel() *Model {
	cs := []docker.Container{
		{ID: "a1", Name: "a1", Project: "alpha", Status: docker.StatusUp},
		{ID: "a2", Name: "a2", Project: "alpha", Status: docker.StatusUp},
		{ID: "a3", Name: "a3", Project: "alpha", Status: docker.StatusUp},
		{ID: "a4", Name: "a4", Project: "alpha", Status: docker.StatusUp},
		{ID: "a5", Name: "a5", Project: "alpha", Status: docker.StatusUp},
		{ID: "b1", Name: "bravo-long-name-1", Project: "bravo", Status: docker.StatusUp},
		{ID: "b2", Name: "bravo-long-name-2", Project: "bravo", Status: docker.StatusUp},
		{ID: "b3", Name: "bravo-long-name-3", Project: "bravo", Status: docker.StatusUp},
	}
	m := newTestModelFromFake(docker.NewFake(cs), cs)
	_ = m.View() // populate cardRects
	return m
}

func selectByID(m *Model, id string) {
	for i, oid := range m.order {
		if oid == id {
			m.selected = i
			return
		}
	}
}

// Down must follow the visual grid: from a3 (alpha row0 col2) the row below in
// the same group is [a5], so it lands on a5 — not a jump into bravo.
func TestNavigationDownNextRowSameGroup(t *testing.T) {
	m := newGridTestModel()
	selectByID(m, "a3")
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedID() != "a5" {
		t.Fatalf("down from a3 = %q, want a5", m.SelectedID())
	}
}

// Down from the last row of alpha crosses the group header into bravo's first
// row, picking the column-closest card (a5 is leftmost -> b1).
func TestNavigationDownIntoNextGroup(t *testing.T) {
	m := newGridTestModel()
	selectByID(m, "a5")
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedID() != "b1" {
		t.Fatalf("down from a5 = %q, want b1", m.SelectedID())
	}
}

// Up from bravo crosses back into alpha's last row [a5].
func TestNavigationUpIntoPrevGroup(t *testing.T) {
	m := newGridTestModel()
	selectByID(m, "b2")
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.SelectedID() != "a5" {
		t.Fatalf("up from b2 = %q, want a5", m.SelectedID())
	}
}

// At the bottom row there is no row below, so down keeps the selection.
func TestNavigationDownAtBottomStays(t *testing.T) {
	m := newGridTestModel()
	selectByID(m, "b3")
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedID() != "b3" {
		t.Fatalf("down from bottom = %q, want b3 (stay)", m.SelectedID())
	}
}

// Up from the top row keeps the selection.
func TestNavigationUpAtTopStays(t *testing.T) {
	m := newGridTestModel()
	selectByID(m, "a2")
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.SelectedID() != "a2" {
		t.Fatalf("up from top = %q, want a2 (stay)", m.SelectedID())
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
