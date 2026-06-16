package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFocusShowsContainerAndEscReturns(t *testing.T) {
	m := newTestModel()
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter focus
	if m.mode != viewFocus {
		t.Fatalf("did not enter focus")
	}
	out := m.View()
	if !strings.Contains(out, "a") { // selected container name
		t.Fatalf("focus view missing container name:\n%s", out)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != viewGrid {
		t.Fatalf("esc should return to grid")
	}
}
