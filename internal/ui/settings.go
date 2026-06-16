package ui

import (
	"fmt"
	"strings"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/version"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsTab int

const (
	tabGroups settingsTab = iota
	tabFields
	tabGeneral
	tabInfo
)

const settingsTabCount = 4

// canonicalFields defines the canonical order for card fields.
var canonicalFields = []string{"status", "health", "cpu", "mem", "net", "port", "uptime", "image", "pids"}

// refreshIntervalOpts and sortOpts and themeOpts are the cycle options for General tab.
var refreshIntervalOpts = []string{"1s", "2s", "5s"}
var sortOpts = []string{"name", "cpu", "mem", "status"}
var themeOpts = []string{"dark", "light", "mono"}

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
		return 9
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

// toggleCardField adds or removes field f from m.cfg.CardFields,
// then rebuilds CardFields in canonical order (no duplicates).
func (m *Model) toggleCardField(f string) {
	enabled := map[string]bool{}
	for _, x := range m.cfg.CardFields {
		enabled[x] = true
	}
	if enabled[f] {
		delete(enabled, f)
	} else {
		enabled[f] = true
	}
	// rebuild in canonical order
	out := make([]string, 0, len(enabled))
	for _, cf := range canonicalFields {
		if enabled[cf] {
			out = append(out, cf)
		}
	}
	m.cfg.CardFields = out
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
		}
	case "K":
		if m.settingsTab == tabGroups {
			m.moveGroup(-1)
		}
	case " ":
		switch m.settingsTab {
		case tabFields:
			if m.settingsSel >= 0 && m.settingsSel < len(canonicalFields) {
				m.toggleCardField(canonicalFields[m.settingsSel])
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
	tabs := []string{"Groups", "Card fields", "General", "Info"}
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
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[↑↓] select  [shift+J / shift+K] move  [tab] next  [enter] save"))
	case tabFields:
		for i, f := range canonicalFields {
			cursor := "  "
			if i == m.settingsSel {
				cursor = lipgloss.NewStyle().Foreground(t.Selected).Render("► ")
			}
			on := "[ ]"
			if contains(m.cfg.CardFields, f) {
				on = lipgloss.NewStyle().Foreground(t.Healthy).Render("[x]")
			}
			body.WriteString(cursor + on + " " + f + "\n")
		}
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[space] toggle  [tab] next  [enter] save"))
	case tabGeneral:
		rows := []struct {
			label string
			value string
		}{
			{"refresh interval   ", m.cfg.RefreshInterval},
			{"confirm destructive", boolStr(m.cfg.ConfirmDestructive)},
			{"sort within group  ", m.cfg.SortWithinGroup},
			{"show stopped       ", boolStr(m.cfg.ShowStopped)},
			{"theme              ", m.cfg.Theme},
		}
		for i, row := range rows {
			cursor := "  "
			if i == m.settingsSel {
				cursor = lipgloss.NewStyle().Foreground(t.Selected).Render("► ")
			}
			body.WriteString(cursor + row.label + " " + row.value + "\n")
		}
		body.WriteString(lipgloss.NewStyle().Foreground(t.Dim).Render("\n[←/→] cycle  [space] toggle  [enter] save"))
	case tabInfo:
		total := 0
		for _, g := range m.groups {
			total += len(g.Containers)
		}
		lbl := lipgloss.NewStyle().Foreground(t.Label)
		body.WriteString(lbl.Render("ekiben   ") + version.String() + "\n")
		body.WriteString(lbl.Render("config   ") + config.Path() + "\n")
		body.WriteString(lbl.Render("groups   ") + fmt.Sprintf("%d", len(m.groups)) + "\n")
		body.WriteString(lbl.Render("contain. ") + fmt.Sprintf("%d", total) + "\n\n")
		keys := lipgloss.NewStyle().Foreground(t.Dim)
		body.WriteString(keys.Render("keys: ↑↓←→ navigate · click select · wheel scroll\n"))
		body.WriteString(keys.Render("      enter focus · l logs · s/r/p/a/u/d actions\n"))
		body.WriteString(keys.Render("      space collapse · c settings · q quit"))
	}

	content := head.String() + "\n\n" + body.String()
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).
		Render(content)
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
