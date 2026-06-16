package ui

import (
	"fmt"
	"strings"

	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) selectedContainer() (docker.Container, bool) {
	id := m.SelectedID()
	for _, g := range m.groups {
		for _, c := range g.Containers {
			if c.ID == id {
				return c, true
			}
		}
	}
	return docker.Container{}, false
}

func (m *Model) viewFocus() string {
	c, ok := m.selectedContainer()
	if !ok {
		return "no container selected — [esc] back"
	}
	st := m.stats[c.ID]
	t := m.theme
	title := lipgloss.NewStyle().Foreground(t.Header).Bold(true).
		Render(fmt.Sprintf("%s — %s", c.Name, c.Project))

	hist := []float64{}
	if rb := m.history[c.ID]; rb != nil {
		hist = rb.Values()
	}

	var b strings.Builder
	b.WriteString(title + "\n\n")
	b.WriteString(statusLine(CardInput{Container: c, Theme: t}, t) + "  " +
		lipgloss.NewStyle().Foreground(t.Dim).Render(c.Image) + "\n")
	b.WriteString(fmt.Sprintf("cpu  %s  %.1f%%\n", Sparkline(hist, 100), st.CPUPerc))
	b.WriteString(fmt.Sprintf("mem  %s / %s\n", HumanBytes(st.MemUsage), HumanBytes(st.MemLimit)))
	b.WriteString(fmt.Sprintf("net  ↓%s ↑%s\n", HumanBytes(st.NetRx), HumanBytes(st.NetTx)))
	if len(c.Ports) > 0 {
		b.WriteString("port " + strings.Join(c.Ports, " ") + "\n")
	}

	// --- live log tail ---
	b.WriteString("\n")
	logLabel := lipgloss.NewStyle().Foreground(t.Header).Bold(true).Render("logs")
	b.WriteString(logLabel + "\n")
	dimStyle := lipgloss.NewStyle().Foreground(t.Dim)
	viewW := m.width
	if viewW < 1 {
		viewW = 80
	}
	if m.focusLogs == "" {
		b.WriteString(dimStyle.Render("  (loading…)") + "\n")
	} else {
		allLines := strings.Split(m.focusLogs, "\n")
		// take last 10 lines
		start := len(allLines) - 10
		if start < 0 {
			start = 0
		}
		tailLines := allLines[start:]
		for _, l := range tailLines {
			if l == "" {
				continue
			}
			truncated := ansi.Truncate(l, viewW-2, "")
			b.WriteString(dimStyle.Render("  "+truncated) + "\n")
		}
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(t.Dim).Render(
		"[esc] back  [l] logs  [s] stop  [r] restart  [a] start  [u] unpause"))
	return b.String()
}
