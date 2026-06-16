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
