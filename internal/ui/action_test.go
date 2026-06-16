package ui

import (
	"testing"

	"github.com/KewinGit/ekiben/internal/docker"
	tea "github.com/charmbracelet/bubbletea"
)

func TestStopAsksConfirmThenActs(t *testing.T) {
	m := newTestModel() // confirm_destructive defaults true

	// Press 's' — should open a confirm modal, not act yet.
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	after := m2.(*Model)
	if after.modal.kind != modalConfirm || after.modal.steps != 1 {
		t.Fatalf("expected single confirm modal, got kind=%v steps=%d", after.modal.kind, after.modal.steps)
	}
	if cmd != nil {
		t.Fatal("no cmd expected after confirm dialog opened")
	}

	// Press 'y' — modal clears, the action cmd is returned.
	m3, actionCmd := after.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	confirmed := m3.(*Model)
	if confirmed.modal.kind != modalNone {
		t.Fatal("modal should be cleared after 'y'")
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

func TestDeleteRunningContainerNeedsDoubleConfirm(t *testing.T) {
	m := newTestModel() // containers are StatusUp (running)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.modal.kind != modalConfirm || m.modal.steps != 2 || !m.modal.danger {
		t.Fatalf("delete of running container should be a danger double-confirm, got %+v", m.modal)
	}
	// first y -> still pending
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.modal.kind != modalConfirm || m.modal.stage != 1 {
		t.Fatalf("after one y the modal should still be pending, got %+v", m.modal)
	}
	// second y -> confirmed (modal cleared, cmd returned)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.modal.kind != modalNone || cmd == nil {
		t.Fatalf("second y should confirm and return a cmd")
	}
}

func TestImageDeleteBlockedWhenInUse(t *testing.T) {
	cs := []docker.Container{{ID: "1", Name: "a", Image: "img:t", Status: docker.StatusUp}}
	m := newTestModelFromFake(docker.NewFake(cs), cs)
	m.images = []docker.Image{{ID: "sha", Repo: "img", Tag: "t"}}
	m.homeTab = homeImages
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.modal.kind != modalBlocked {
		t.Fatalf("deleting an in-use image should be blocked, got %+v", m.modal)
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
