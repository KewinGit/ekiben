package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStopAsksConfirmThenActs(t *testing.T) {
	m := newTestModel() // confirm_destructive defaults true

	// Press 's' — should open confirm modal, not act yet.
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	after := m2.(*Model)
	if !after.confirm || after.confirmFor != "stop" || after.confirmID != "1" {
		t.Fatalf("expected pending stop confirm, got confirm=%v for=%q id=%q",
			after.confirm, after.confirmFor, after.confirmID)
	}
	if cmd != nil {
		t.Fatal("no cmd expected after confirm dialog opened")
	}

	// Press 'y' — confirm clears, a doActionCmd is returned.
	m3, actionCmd := after.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	confirmed := m3.(*Model)
	if confirmed.confirm {
		t.Fatal("confirm should be cleared after 'y'")
	}
	if actionCmd == nil {
		t.Fatal("expected a cmd from confirmed action")
	}

	// Execute the cmd and feed actionResultMsg back.
	msg := actionCmd().(actionResultMsg)
	if msg.err != nil {
		t.Fatalf("unexpected action error: %v", msg.err)
	}
	m4, _ := confirmed.Update(msg)
	final := m4.(*Model)
	if final.lastErr != nil {
		t.Fatalf("lastErr should be nil after successful action, got %v", final.lastErr)
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
