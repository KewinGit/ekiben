package ui

import (
	"strings"

	"github.com/KewinGit/ekiben/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsTab int

const (
	tabGroups settingsTab = iota
	tabFields
	tabGeneral
)

func (m *Model) enterSettings() {
	m.settingsTab = tabGroups
	m.settingsSel = 0
	m.settingsGroups = append([]string{}, currentGroupNames(m)...)
}

func currentGroupNames(m *Model) []string {
	out := []string{}
	for _, g := range m.groups {
		out = append(out, g.Name)
	}
	return out
}

func (m *Model) moveGroup(delta int) {
	i := m.settingsSel
	j := i + delta
	if j < 0 || j >= len(m.settingsGroups) {
		return
	}
	m.settingsGroups[i], m.settingsGroups[j] = m.settingsGroups[j], m.settingsGroups[i]
	m.settingsSel = j
}

// updateSettings handles keys while in settings mode.
func (m *Model) updateSettings(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "tab":
		m.settingsTab = (m.settingsTab + 1) % 3
		m.settingsSel = 0
	case "up", "k":
		if m.settingsSel > 0 {
			m.settingsSel--
		}
	case "down", "j":
		m.settingsSel++ // clamped at render time
	case "J":
		if m.settingsTab == tabGroups {
			m.moveGroup(1)
		}
	case "K":
		if m.settingsTab == tabGroups {
			m.moveGroup(-1)
		}
	case "enter":
		m.saveSettings()
		m.mode = viewGrid
	}
	return nil
}

func (m *Model) saveSettings() {
	m.cfg.GroupOrder = append([]string{}, m.settingsGroups...)
	_ = m.cfg.Save(config.Path())
	m.applyContainers(m.lastContainers)
}

func (m *Model) viewSettings() string {
	t := m.theme
	tabs := []string{"Groups", "Card fields", "General"}
	var head strings.Builder
	for i, name := range tabs {
		style := lipgloss.NewStyle().Foreground(t.Dim)
		if settingsTab(i) == m.settingsTab {
			style = lipgloss.NewStyle().Foreground(t.Header).Bold(true)
		}
		head.WriteString(style.Render("  " + name + "  "))
	}
	var body strings.Builder
	switch m.settingsTab {
	case tabGroups:
		for i, g := range m.settingsGroups {
			cursor := "  "
			if i == m.settingsSel {
				cursor = lipgloss.NewStyle().Foreground(t.Selected).Render("► ")
			}
			body.WriteString(cursor + g + "\n")
		}
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[J/K] move  [tab] next  [enter] save"))
	case tabFields:
		for _, f := range []string{"status", "health", "cpu", "mem", "net", "port", "uptime", "image", "pids"} {
			on := "[ ]"
			if contains(m.cfg.CardFields, f) {
				on = lipgloss.NewStyle().Foreground(t.Healthy).Render("[x]")
			}
			body.WriteString(on + " " + f + "\n")
		}
	case tabGeneral:
		body.WriteString("refresh interval   " + m.cfg.RefreshInterval + "\n")
		body.WriteString("confirm destructive " + boolStr(m.cfg.ConfirmDestructive) + "\n")
		body.WriteString("sort within group   " + m.cfg.SortWithinGroup + "\n")
		body.WriteString("show stopped        " + boolStr(m.cfg.ShowStopped) + "\n")
		body.WriteString("theme               " + m.cfg.Theme + "\n")
	}
	return head.String() + "\n\n" + body.String()
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func boolStr(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
