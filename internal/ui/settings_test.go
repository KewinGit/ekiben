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
	// canonical index 2 = "cpu"
	m.settingsSel = 2

	// "cpu" should be present in default config
	if !contains(m.cfg.CardFields, "cpu") {
		t.Fatalf("precondition: cpu should be in default CardFields, got %v", m.cfg.CardFields)
	}

	// press space → cpu should be removed
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if contains(m.cfg.CardFields, "cpu") {
		t.Fatalf("after first space, cpu should be removed; CardFields=%v", m.cfg.CardFields)
	}

	// press space again → cpu should be re-added
	m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !contains(m.cfg.CardFields, "cpu") {
		t.Fatalf("after second space, cpu should be present; CardFields=%v", m.cfg.CardFields)
	}

	// verify canonical order: cpu must come after health (index 1) and before mem (index 3)
	cpuPos, healthPos, memPos := -1, -1, -1
	for i, f := range m.cfg.CardFields {
		switch f {
		case "cpu":
			cpuPos = i
		case "health":
			healthPos = i
		case "mem":
			memPos = i
		}
	}
	if healthPos >= 0 && cpuPos >= 0 && healthPos >= cpuPos {
		t.Fatalf("canonical order violated: health(%d) should be before cpu(%d)", healthPos, cpuPos)
	}
	if cpuPos >= 0 && memPos >= 0 && cpuPos >= memPos {
		t.Fatalf("canonical order violated: cpu(%d) should be before mem(%d)", cpuPos, memPos)
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
