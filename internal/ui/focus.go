package ui

import (
	"fmt"
	"strings"

	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/charmbracelet/lipgloss"
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

func histValues(rb *model.RingBuffer) []float64 {
	if rb == nil {
		return nil
	}
	return rb.Values()
}

func (m *Model) viewFocus() string {
	c, ok := m.selectedContainer()
	if !ok {
		return "no container selected — [esc] back"
	}
	st := m.stats[c.ID]
	t := m.theme
	lbl := lipgloss.NewStyle().Foreground(t.Label)
	dim := lipgloss.NewStyle().Foreground(t.Dim)

	memPct := 0.0
	if st.MemLimit > 0 {
		memPct = float64(st.MemUsage) / float64(st.MemLimit) * 100
	}

	// --- info section (top) ---
	var info strings.Builder
	info.WriteString(lipgloss.NewStyle().Foreground(t.Header).Bold(true).Render(c.Name) +
		dim.Render("  "+c.Project) + "\n")
	statusln := statusLine(CardInput{Container: c, Theme: t}, t)
	if up := uptimeStr(c); up != "" {
		statusln += dim.Render(" · up " + up)
	}
	info.WriteString(statusln + dim.Render("  "+c.Image) + "\n")
	info.WriteString(fmt.Sprintf("%s %s %6.1f%%\n", lbl.Render("cpu"), Sparkline(histValues(m.history[c.ID]), 100), st.CPUPerc))
	info.WriteString(fmt.Sprintf("%s %s %s %5.1f%%\n", lbl.Render("mem"), Sparkline(histValues(m.memHistory[c.ID]), 100), HumanBytes(st.MemUsage), memPct))
	info.WriteString(fmt.Sprintf("%s ↓%s ↑%s", lbl.Render("net"), HumanBytes(st.NetRx), HumanBytes(st.NetTx)))
	if len(c.Ports) > 0 {
		info.WriteString("   " + lbl.Render("port") + " " +
			lipgloss.NewStyle().Foreground(t.Accent).Render(strings.Join(c.Ports, " ")))
	}
	infoStr := info.String()
	infoH := lipgloss.Height(infoStr)

	// --- logs section (bottom, scrollable) ---
	status := ""
	if m.logsFollow {
		status += "  follow ON"
	}
	if m.logsSearching {
		status += fmt.Sprintf("  search: %s_", m.logsQuery)
	} else if m.logsQuery != "" {
		status += fmt.Sprintf("  filter: %s", m.logsQuery)
	}
	logsHead := lipgloss.NewStyle().Foreground(t.Header).Bold(true).Render("logs") + dim.Render(status)
	help := dim.Render(
		"↑↓ PgUp/PgDn g/G scroll · wheel scroll · f follow · / search · esc back\n" +
			"s stop · r restart · p pause · a start · u unpause · d delete")
	helpH := lipgloss.Height(help)

	logsH := m.height - infoH - helpH - 2 // logs header + separator
	if logsH < 3 {
		logsH = 3
	}
	var logsView string
	if m.logsReady {
		m.logsVP.Width = m.width - 1 // leave a column for the scrollbar
		m.logsVP.Height = logsH
		bar := scrollbar(logsH, m.logsTotalLines, m.logsVP.YOffset, logsH, t)
		logsView = lipgloss.JoinHorizontal(lipgloss.Top, m.logsVP.View(), bar)
	} else {
		logsView = dim.Render("(loading…)")
	}

	return infoStr + "\n" + logsHead + "\n" + logsView + "\n" + help
}
