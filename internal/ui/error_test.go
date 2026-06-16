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

// TestActionFailureSetsLastErr verifies that gap #4: a failed action
// propagates to m.lastErr via doAction.
func TestActionFailureSetsLastErr(t *testing.T) {
	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
	}
	fake := docker.NewFake(cs)
	actionErr := errors.New("permission denied")
	fake.ActionErr = actionErr

	m := newTestModelWith(fake)

	// Call doAction directly — this is the unit under test.
	err := m.doAction("stop", "1")

	if err != actionErr {
		t.Fatalf("doAction returned %v, want %v", err, actionErr)
	}
	if m.lastErr == nil {
		t.Fatal("lastErr should be set after a failed action")
	}
	if m.lastErr != actionErr {
		t.Fatalf("lastErr = %v, want %v", m.lastErr, actionErr)
	}
}

// TestActionFailureViaConfirmSetsLastErr verifies gap #4 through the confirm
// modal path.
func TestActionFailureViaConfirmSetsLastErr(t *testing.T) {
	cs := []docker.Container{
		{ID: "1", Name: "a", Project: "p", Status: docker.StatusUp},
	}
	fake := docker.NewFake(cs)
	actionErr := errors.New("container locked")
	fake.ActionErr = actionErr

	m := newTestModelWith(fake)

	// Trigger confirm dialog for stop.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !m.confirm {
		t.Fatal("expected confirm modal after 's'")
	}

	// Confirm the action.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	updated := m2.(*Model)

	if updated.lastErr == nil {
		t.Fatal("lastErr should be set after confirmed failed action")
	}
	if updated.lastErr != actionErr {
		t.Fatalf("lastErr = %v, want %v", updated.lastErr, actionErr)
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
