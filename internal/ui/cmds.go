package ui

import tea "github.com/charmbracelet/bubbletea"

// these are replaced with real implementations in Task 16
func (m *Model) pollCmd() tea.Cmd    { return nil }
func (m *Model) refreshCmd() tea.Cmd { return nil }

// confirm helpers (real bodies in Task 17)
func (m *Model) handleConfirmKey(tea.KeyMsg) (tea.Model, tea.Cmd) { m.confirm = false; return m, nil }
func (m *Model) confirmBar() string                               { return "" }

// other-view stubs (real bodies in Tasks 18-20)
func (m *Model) viewFocus() string    { return "focus" }
func (m *Model) viewLogs() string     { return "logs" }
func (m *Model) viewSettings() string { return "settings" }
