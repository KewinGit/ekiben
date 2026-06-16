package ui

import (
	"errors"
	"testing"

	"github.com/KewinGit/ekiben/internal/docker"
	tea "github.com/charmbracelet/bubbletea"
)

// TestErrorBannerShown verifies that an errMsg sets lastErr and that
// viewGrid renders the error banner.
func TestErrorBannerShown(t *testing.T) {
	m := newTestModel()
	sentinel := errors.New("cannot connect to Docker")

	m2, _ := m.Update(errMsg{err: sentinel})
	updated := m2.(*Model)

	if updated.lastErr == nil {
		t.Fatal("lastErr should be set after errMsg")
	}
	if updated.lastErr != sentinel {
		t.Fatalf("lastErr = %v, want %v", updated.lastErr, sentinel)
	}

	view := updated.viewGrid()
	if !containsString(view, "Docker error:") {
		t.Errorf("viewGrid should contain error banner, got:\n%s", view)
	}
	if !containsString(view, sentinel.Error()) {
		t.Errorf("viewGrid should contain error message %q, got:\n%s", sentinel.Error(), view)
	}
}

// TestErrorClearedOnSuccess verifies that lastErr is cleared when
// applyContainers is called (i.e. a successful refresh).
func TestErrorClearedOnSuccess(t *testing.T) {
	m := newTestModel()
	m.lastErr = errors.New("some transient error")

	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
	}
	m.applyContainers(cs)

	if m.lastErr != nil {
		t.Fatalf("lastErr should be nil after successful applyContainers, got %v", m.lastErr)
	}
}

// TestActionFailureSetsLastErr verifies that a failed async action
// propagates to m.lastErr via doActionCmd + actionResultMsg roundtrip.
func TestActionFailureSetsLastErr(t *testing.T) {
	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
	}
	fake := docker.NewFake(cs)
	actionErr := errors.New("permission denied")
	fake.ActionErr = actionErr

	m := newTestModelWith(fake)

	// Execute the cmd to get an actionResultMsg, then feed it to Update.
	cmd := m.doActionCmd("stop", "1")
	msg := cmd().(actionResultMsg)
	if msg.err != actionErr {
		t.Fatalf("actionResultMsg.err = %v, want %v", msg.err, actionErr)
	}

	m2, _ := m.Update(msg)
	updated := m2.(*Model)
	if updated.lastErr == nil {
		t.Fatal("lastErr should be set after a failed action")
	}
	if updated.lastErr != actionErr {
		t.Fatalf("lastErr = %v, want %v", updated.lastErr, actionErr)
	}
}

// TestActionFailureViaConfirmSetsLastErr verifies the confirm modal path:
// confirm gates execution and a failing client surfaces lastErr.
func TestActionFailureViaConfirmSetsLastErr(t *testing.T) {
	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
	}
	fake := docker.NewFake(cs)
	actionErr := errors.New("container locked")
	fake.ActionErr = actionErr

	m := newTestModelWith(fake)

	// Trigger confirm modal for stop.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	updated := m2.(*Model)
	if updated.modal.kind != modalConfirm {
		t.Fatal("expected confirm modal after 's'")
	}

	// Confirm the action — returns a cmd (doActionCmd), not yet an error.
	m3, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	afterConfirm := m3.(*Model)
	if afterConfirm.modal.kind != modalNone {
		t.Fatal("modal should be cleared after answer")
	}
	if cmd == nil {
		t.Fatal("expected a cmd from confirmed action")
	}

	// Execute the cmd and feed the actionResultMsg back to Update.
	msg := cmd().(actionResultMsg)
	m4, _ := afterConfirm.Update(msg)
	final := m4.(*Model)

	if final.lastErr == nil {
		t.Fatal("lastErr should be set after confirmed failed action")
	}
	if final.lastErr != actionErr {
		t.Fatalf("lastErr = %v, want %v", final.lastErr, actionErr)
	}
}

// newTestModelWith creates a test model using the provided fake client.
func newTestModelWith(fake *docker.Fake) *Model {
	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
	}
	return newTestModelFromFake(fake, cs)
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
