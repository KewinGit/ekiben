package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStopAsksConfirmThenActs(t *testing.T) {
	m := newTestModel() // confirm_destructive defaults true
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !m.confirm || m.confirmFor != "stop" || m.confirmID != "1" {
		t.Fatalf("expected pending stop confirm, got confirm=%v for=%q id=%q", m.confirm, m.confirmFor, m.confirmID)
	}
	// answer yes
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.confirm {
		t.Fatalf("confirm should be cleared after answer")
	}
}

func TestCollapseToggle(t *testing.T) {
	m := newTestModel()
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.collapsed["p"] {
		t.Fatalf("group should be collapsed")
	}
	if len(m.order) != 0 {
		t.Fatalf("collapsed group should remove its cards from nav order")
	}
}
