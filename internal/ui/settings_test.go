package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSettingsReorderMovesGroup(t *testing.T) {
	m := newTestModel()
	// add a second group so order matters
	m.cfg.GroupOrder = []string{"p", "q"}
	m.settingsGroups = []string{"p", "q"}
	m.settingsTab = tabGroups
	m.settingsSel = 0
	m.moveGroup(1) // move "p" down
	if m.settingsGroups[0] != "q" || m.settingsGroups[1] != "p" {
		t.Fatalf("reorder failed: %v", m.settingsGroups)
	}
	_ = tea.KeyMsg{}
}

// TestSettingsToggleField: press space on a field that's enabled → it's removed;
// press space again → it's re-added and CardFields is in canonical order.
func TestSettingsToggleField(t *testing.T) {
	m := newTestModel()
	m.mode = viewSettings
	m.enterSettings()
	m.settingsTab = tabFields
	// settingsFields starts as the saved order (status,health,cpu,...); index 2 = cpu
	if m.settingsFields[2] != "cpu" {
		t.Fatalf("expected cpu at index 2, got %v", m.settingsFields)
	}
	m.settingsSel = 2

	// space → cpu disabled (in the working set, not yet saved)
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.settingsFieldOn["cpu"] {
		t.Fatalf("after first space cpu should be disabled")
	}
	// space → cpu enabled again
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !m.settingsFieldOn["cpu"] {
		t.Fatalf("after second space cpu should be enabled")
	}
	// enter saves: cfg.CardFields holds enabled fields in order
	m.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	if !contains(m.cfg.CardFields, "cpu") {
		t.Fatalf("after save cpu should be present; got %v", m.cfg.CardFields)
	}
}

func TestSettingsReorderField(t *testing.T) {
	m := newTestModel()
	m.enterSettings()
	m.settingsTab = tabFields
	m.settingsSel = 0 // "status"
	first := m.settingsFields[0]
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")}) // move down
	if m.settingsFields[1] != first || m.settingsSel != 1 {
		t.Fatalf("field reorder failed: %v sel=%d", m.settingsFields, m.settingsSel)
	}
}

// TestSettingsGeneralCycleAndToggle: general tab row 0 right → RefreshInterval advances;
// row 1 space → ConfirmDestructive flips.
func TestSettingsGeneralCycleAndToggle(t *testing.T) {
	m := newTestModel()
	m.mode = viewSettings
	m.enterSettings()
	m.settingsTab = tabGeneral

	initialInterval := m.cfg.RefreshInterval

	// row 0 = refresh interval, press right
	m.settingsSel = 0
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.RefreshInterval == initialInterval {
		t.Fatalf("RefreshInterval should have changed from %q after pressing right", initialInterval)
	}

	// row 1 = confirm destructive, press space
	initialConfirm := m.cfg.ConfirmDestructive
	m.settingsSel = 1
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.cfg.ConfirmDestructive == initialConfirm {
		t.Fatalf("ConfirmDestructive should have flipped from %v", initialConfirm)
	}
}
