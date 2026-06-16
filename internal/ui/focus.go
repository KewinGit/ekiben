package ui

import (
	"fmt"
	"strings"

	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// focusLogsWidth is the log viewport width inside the detail's bordered panel
// (panel border 1 + scrollbar 1 on each relevant side).
func (m *Model) focusLogsWidth() int {
	// panel: Width = m.width-2 (total m.width incl. border); text area = m.width-4
	// after padding; minus 1 column for the scrollbar.
	w := m.width - 5
	if w < 1 {
		w = 1
	}
	return w
}

// findImageSize returns the on-disk size of the image named ref ("repo:tag"), if known.
func (m *Model) findImageSize(ref string) (int64, bool) {
	for _, img := range m.images {
		if img.Repo+":"+img.Tag == ref {
			return img.Size, true
		}
	}
	return 0, false
}

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

	accent := lipgloss.NewStyle().Foreground(t.Accent)
	innerW := m.width - 4 // panel border (2) + padding (2)
	if innerW < 10 {
		innerW = 10
	}
	trunc := func(s string) string { return ansi.Truncate(s, innerW, "…") }

	// --- info section (top) ---
	var info strings.Builder
	info.WriteString(lipgloss.NewStyle().Foreground(t.Header).Bold(true).Render(c.Name) +
		dim.Render("  "+c.Project) + "\n")
	info.WriteString(statusLine(CardInput{Container: c, Theme: t}, t, uptimeStr(c)) + "\n")
	imgLine := c.Image
	if sz, ok := m.findImageSize(c.Image); ok {
		imgLine += "  " + HumanBytes(uint64(sz))
	}
	info.WriteString(lbl.Render("image") + " " + trunc(imgLine) + "\n")
	info.WriteString(fmt.Sprintf("%s   %s %6.1f%%\n", lbl.Render("cpu"), Sparkline(histValues(m.history[c.ID]), 100), st.CPUPerc))
	info.WriteString(fmt.Sprintf("%s   %s %s %5.1f%%\n", lbl.Render("mem"), Sparkline(histValues(m.memHistory[c.ID]), 100), HumanBytes(st.MemUsage), memPct))
	info.WriteString(fmt.Sprintf("%s   ↓%s ↑%s\n", lbl.Render("net"), HumanBytes(st.NetRx), HumanBytes(st.NetTx)))
	ports := "—"
	if len(c.Ports) > 0 {
		ports = accent.Render(strings.Join(c.Ports, " "))
	}
	info.WriteString(lbl.Render("ports") + " " + ports + "\n")
	exp := "—"
	if len(c.Exposed) > 0 {
		exp = strings.Join(c.Exposed, " ")
	}
	info.WriteString(lbl.Render("exp  ") + " " + dim.Render(trunc(exp)) + "\n")
	nets := "—"
	if len(c.Networks) > 0 {
		nets = strings.Join(c.Networks, " ")
	}
	info.WriteString(lbl.Render("nets ") + " " + trunc(nets) + "\n")
	vols := "—"
	if len(c.Mounts) > 0 {
		vols = strings.Join(c.Mounts, " ")
	}
	info.WriteString(lbl.Render("vols ") + " " + trunc(vols))

	box := func(s string) string {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).Padding(0, 1).Width(m.width - 2).Render(s)
	}
	infoBox := box(strings.TrimRight(info.String(), "\n"))
	infoH := lipgloss.Height(infoBox)

	// --- logs section (bottom, scrollable, bordered) ---
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
		"↑↓ PgUp/PgDn g/G scroll · wheel scroll · f follow · / search · e shell · esc back\n" +
			"s stop · r restart · p pause · a start · u unpause · d delete")
	helpH := lipgloss.Height(help)

	logsBoxH := m.height - infoH - helpH - 2
	if logsBoxH < 5 {
		logsBoxH = 5
	}
	var logsInner string
	if m.logsReady {
		vpH := logsBoxH - 3 // box border (2) + logs header (1)
		if vpH < 1 {
			vpH = 1
		}
		m.logsVP.Width = m.focusLogsWidth()
		m.logsVP.Height = vpH
		bar := scrollbar(vpH, m.logsTotalLines, m.logsVP.YOffset, vpH, t)
		body := lipgloss.JoinHorizontal(lipgloss.Top, m.logsVP.View(), bar)
		logsInner = logsHead + "\n" + body
	} else {
		logsInner = logsHead + "\n" + dim.Render("(loading…)")
	}
	logsBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).Width(m.width-2).Padding(0, 1).Render(logsInner)

	return infoBox + "\n" + logsBox + "\n" + help
}
