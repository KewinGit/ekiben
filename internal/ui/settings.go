package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsTab int

const (
	tabGroups settingsTab = iota
	tabFields
	tabGeneral
)

const settingsTabCount = 3

// canonicalFields defines all available container-card fields (default order).
var canonicalFields = []string{"status", "health", "cpu", "mem", "net", "port", "exposed", "uptime", "image", "pids", "restarts", "errors"}

// refreshIntervalOpts and sortOpts and themeOpts are the cycle options for General tab.
var refreshIntervalOpts = []string{"1s", "2s", "5s"}
var sortOpts = []string{"name", "cpu", "mem", "status"}
var themeOpts = []string{"dark", "light", "mono"}

func (m *Model) enterSettings() {
	m.settingsTab = tabGroups
	m.settingsSel = 0
	m.settingsGroups = append([]string{}, currentGroupNames(m)...)
	// fields working list: enabled (in saved order) first, then the rest
	on := map[string]bool{}
	m.settingsFields = m.settingsFields[:0]
	for _, f := range m.cfg.CardFields {
		if !on[f] {
			on[f] = true
			m.settingsFields = append(m.settingsFields, f)
		}
	}
	for _, f := range canonicalFields {
		if !on[f] {
			m.settingsFields = append(m.settingsFields, f)
		}
	}
	m.settingsFieldOn = map[string]bool{}
	for _, f := range m.cfg.CardFields {
		m.settingsFieldOn[f] = true
	}
}

func (m *Model) moveField(delta int) {
	i := m.settingsSel
	j := i + delta
	if i < 0 || i >= len(m.settingsFields) || j < 0 || j >= len(m.settingsFields) {
		return
	}
	m.settingsFields[i], m.settingsFields[j] = m.settingsFields[j], m.settingsFields[i]
	m.settingsSel = j
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
	if i < 0 || i >= len(m.settingsGroups) || j < 0 || j >= len(m.settingsGroups) {
		return
	}
	m.settingsGroups[i], m.settingsGroups[j] = m.settingsGroups[j], m.settingsGroups[i]
	m.settingsSel = j
}

// settingsRowCount returns the number of selectable rows for the active tab.
func (m *Model) settingsRowCount() int {
	switch m.settingsTab {
	case tabGroups:
		return len(m.settingsGroups)
	case tabFields:
		return len(m.settingsFields)
	case tabGeneral:
		return 5
	}
	return 0
}

// cycle advances cur through opts by dir (+1 or -1), wrapping around.
func cycle(opts []string, cur string, dir int) string {
	for i, o := range opts {
		if o == cur {
			n := (i + dir + len(opts)) % len(opts)
			return opts[n]
		}
	}
	// cur not found — return first option
	return opts[0]
}

// updateSettings handles keys while in settings mode.
func (m *Model) updateSettings(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	switch key {
	case "tab":
		m.settingsTab = (m.settingsTab + 1) % settingsTabCount
		m.settingsSel = 0
	case "up", "k":
		if m.settingsSel > 0 {
			m.settingsSel--
		}
	case "down", "j":
		if m.settingsSel < m.settingsRowCount()-1 {
			m.settingsSel++
		}
	case "J":
		if m.settingsTab == tabGroups {
			m.moveGroup(1)
		} else if m.settingsTab == tabFields {
			m.moveField(1)
		}
	case "K":
		if m.settingsTab == tabGroups {
			m.moveGroup(-1)
		} else if m.settingsTab == tabFields {
			m.moveField(-1)
		}
	case " ":
		switch m.settingsTab {
		case tabFields:
			if m.settingsSel >= 0 && m.settingsSel < len(m.settingsFields) {
				f := m.settingsFields[m.settingsSel]
				m.settingsFieldOn[f] = !m.settingsFieldOn[f]
			}
		case tabGeneral:
			switch m.settingsSel {
			case 1: // confirm destructive
				m.cfg.ConfirmDestructive = !m.cfg.ConfirmDestructive
			case 3: // show stopped
				m.cfg.ShowStopped = !m.cfg.ShowStopped
			}
		}
	case "left":
		if m.settingsTab == tabGeneral {
			switch m.settingsSel {
			case 0:
				m.cfg.RefreshInterval = cycle(refreshIntervalOpts, m.cfg.RefreshInterval, -1)
			case 2:
				m.cfg.SortWithinGroup = cycle(sortOpts, m.cfg.SortWithinGroup, -1)
			case 4:
				m.cfg.Theme = cycle(themeOpts, m.cfg.Theme, -1)
				m.theme = ThemeByName(m.cfg.Theme)
			}
		}
	case "right":
		if m.settingsTab == tabGeneral {
			switch m.settingsSel {
			case 0:
				m.cfg.RefreshInterval = cycle(refreshIntervalOpts, m.cfg.RefreshInterval, +1)
			case 2:
				m.cfg.SortWithinGroup = cycle(sortOpts, m.cfg.SortWithinGroup, +1)
			case 4:
				m.cfg.Theme = cycle(themeOpts, m.cfg.Theme, +1)
				m.theme = ThemeByName(m.cfg.Theme)
			}
		}
	case "enter":
		m.saveSettings()
		m.mode = viewGrid
		return m.enrichmentCmds() // fetch data for any newly-enabled fields
	}
	return nil
}

func (m *Model) saveSettings() {
	m.cfg.GroupOrder = append([]string{}, m.settingsGroups...)
	// card fields = enabled ones, in the arranged order
	fields := make([]string, 0, len(m.settingsFields))
	for _, f := range m.settingsFields {
		if m.settingsFieldOn[f] {
			fields = append(fields, f)
		}
	}
	m.cfg.CardFields = fields
	m.saveConfig()
	m.applyContainers(m.lastContainers)
}

// settingsContent renders the head + body for a given settings tab.
func (m *Model) settingsContent(tab settingsTab) string {
	t := m.theme
	tabs := []string{"Groups", "Card fields", "General"}
	var head strings.Builder
	for i, name := range tabs {
		style := lipgloss.NewStyle().Foreground(t.Dim)
		if settingsTab(i) == tab {
			style = lipgloss.NewStyle().Foreground(t.Header).Bold(true)
		}
		head.WriteString(style.Render("  " + name + "  "))
	}
	cursor := func(i int) string {
		if i == m.settingsSel {
			return lipgloss.NewStyle().Foreground(t.Selected).Render("► ")
		}
		return "  "
	}
	var body strings.Builder
	switch tab {
	case tabGroups:
		for i, g := range m.settingsGroups {
			body.WriteString(cursor(i) + g + "\n")
		}
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[↑↓] select  [shift+J / shift+K] move  [tab] next  [enter] save"))
	case tabFields:
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("fields shown on each container card (top = first):") + "\n")
		for i, f := range m.settingsFields {
			on := "[ ]"
			if m.settingsFieldOn[f] {
				on = lipgloss.NewStyle().Foreground(t.Healthy).Render("[x]")
			}
			body.WriteString(cursor(i) + on + " " + f + "\n")
		}
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[space] toggle  [shift+J / shift+K] reorder  [tab] next  [enter] save"))
	case tabGeneral:
		rows := []struct{ label, value string }{
			{"refresh interval   ", m.cfg.RefreshInterval},
			{"confirm destructive", boolStr(m.cfg.ConfirmDestructive)},
			{"sort within group  ", m.cfg.SortWithinGroup},
			{"show stopped       ", boolStr(m.cfg.ShowStopped)},
			{"theme              ", m.cfg.Theme},
		}
		for i, row := range rows {
			body.WriteString(cursor(i) + row.label + " " + row.value + "\n")
		}
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[←/→] cycle  [space] toggle  [enter] save"))
	}
	return head.String() + "\n\n" + body.String()
}

func (m *Model) viewSettings() string {
	t := m.theme
	// Fixed panel size = the largest content across all tabs, so switching tabs
	// doesn't resize the box.
	maxW, maxH := 0, 0
	for _, tb := range []settingsTab{tabGroups, tabFields, tabGeneral} {
		c := m.settingsContent(tb)
		if w := lipgloss.Width(c); w > maxW {
			maxW = w
		}
		if h := lipgloss.Height(c); h > maxH {
			maxH = h
		}
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).
		Width(maxW).
		Height(maxH).
		Render(m.settingsContent(m.settingsTab))
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
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
