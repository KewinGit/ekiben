package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// humanDuration renders a duration compactly: 45s, 50m, 3h, 2d.
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// uptimeStr returns the compact uptime for a running container, or "".
func uptimeStr(c docker.Container) string {
	if !c.Running() || c.CreatedAt.IsZero() {
		return ""
	}
	return humanDuration(time.Since(c.CreatedAt))
}

type CardInput struct {
	Container docker.Container
	Stats     docker.Stats
	History   []float64 // cpu history for sparkline
	Fields    []string
	Width     int
	Selected  bool
	Theme     Theme

	// optional enrichment (shown only when the field is enabled)
	Restarts      int
	RestartsKnown bool
	LogErrs       int
	LogWarns      int
	LogKnown      bool
}

// RenderCard returns a bordered, fixed-width card for one container.
func RenderCard(in CardInput) string {
	t := in.Theme
	innerW := in.Width - 2 // borders
	if innerW < MinCardWidth-2 {
		innerW = MinCardWidth - 2
	}

	lbl := lipgloss.NewStyle().Foreground(t.Label)
	var lines []string

	for _, f := range in.Fields {
		switch f {
		case "status":
			up := ""
			if contains(in.Fields, "uptime") {
				up = uptimeStr(in.Container)
			}
			lines = append(lines, statusLine(in, t, up))
		case "health":
			// folded into the status line for compactness; skip separate line
		case "cpu":
			lines = append(lines, fmt.Sprintf("%s %5.1f%%", lbl.Render("cpu"), in.Stats.CPUPerc))
		case "mem":
			memPct := 0.0
			if in.Stats.MemLimit > 0 {
				memPct = float64(in.Stats.MemUsage) / float64(in.Stats.MemLimit) * 100
			}
			lines = append(lines, fmt.Sprintf("%s %s %4.1f%%", lbl.Render("mem"), HumanBytes(in.Stats.MemUsage), memPct))
		case "net":
			lines = append(lines, fmt.Sprintf("%s ↓%s ↑%s", lbl.Render("net"),
				HumanBytes(in.Stats.NetRx), HumanBytes(in.Stats.NetTx)))
		case "port":
			ports := "—"
			if len(in.Container.Ports) > 0 {
				ports = lipgloss.NewStyle().Foreground(t.Accent).Render(strings.Join(in.Container.Ports, " "))
			}
			lines = append(lines, fmt.Sprintf("%s %s", lbl.Render("port"), ports))
		case "exposed":
			exp := "—"
			if len(in.Container.Exposed) > 0 {
				exp = strings.Join(in.Container.Exposed, " ")
			}
			lines = append(lines, lbl.Render("exp")+" "+lipgloss.NewStyle().Foreground(t.Dim).Render(exp))
		case "image":
			head := lbl.Render("img") + " "
			avail := innerW - 4 // "img " prefix
			if avail < 1 {
				avail = 1
			}
			name := in.Container.Image
			first, rest := name, ""
			if len(name) > avail {
				first, rest = name[:avail], name[avail:]
			}
			lines = append(lines, head+first)
			// continuation line, indented under the value (not under "img")
			lines = append(lines, "    "+ansi.Truncate(rest, avail, "…"))
		case "pids":
			lines = append(lines, fmt.Sprintf("%s %d", lbl.Render("pids"), in.Stats.PIDs))
		case "restarts":
			v := "—"
			if in.RestartsKnown {
				v = fmt.Sprintf("%d", in.Restarts)
				if in.Restarts >= 5 {
					v += lipgloss.NewStyle().Foreground(t.Problem).Render(" ⟳")
				}
			}
			lines = append(lines, lbl.Render("restr")+" "+v)
		case "errors":
			v := lipgloss.NewStyle().Foreground(t.Dim).Render("—")
			if in.LogKnown {
				v = fmt.Sprintf("%s %s",
					lipgloss.NewStyle().Foreground(t.Problem).Render(fmt.Sprintf("%d err", in.LogErrs)),
					lipgloss.NewStyle().Foreground(t.Warn).Render(fmt.Sprintf("%d warn", in.LogWarns)))
			}
			lines = append(lines, lbl.Render("log")+" "+v)
		case "uptime":
			// shown on the status line; render standalone only if status is hidden
			if !contains(in.Fields, "status") {
				if up := uptimeStr(in.Container); up != "" {
					lines = append(lines, fmt.Sprintf("%s %s", lbl.Render("up"), up))
				}
			}
		}
	}

	// Truncate every line (and the title) to the inner width so nothing wraps:
	// wrapping would make a card taller than its siblings and break the grid.
	title := ansi.Truncate(cardTitle(in, t), innerW, "")
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], innerW, "")
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	borderColor := t.Border
	border := lipgloss.NormalBorder()
	if in.Container.Problem() {
		borderColor = t.Problem
	}
	if in.Selected {
		borderColor = t.Selected
		border = lipgloss.DoubleBorder()
	}

	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Width(innerW).
		Height(len(lines) + 1)

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, title, body))
}

func cardTitle(in CardInput, t Theme) string {
	// The double border already marks the selection — no extra arrow needed.
	return fmt.Sprintf("%s %s", dotFor(in.Container, t), in.Container.Name)
}

// statusLine renders the status line. When uptime != "" it is inserted between
// "up" and the health word, i.e. "up · <uptime> · healthy".
func statusLine(in CardInput, t Theme, uptime string) string {
	c := in.Container
	switch c.Status {
	case docker.StatusExited:
		return lipgloss.NewStyle().Foreground(t.Problem).Render(fmt.Sprintf("exited (%d)", c.ExitCode))
	case docker.StatusRestarting:
		return lipgloss.NewStyle().Foreground(t.Problem).Render("restarting")
	case docker.StatusPaused:
		return lipgloss.NewStyle().Foreground(t.Warn).Render("paused")
	}
	s := "up"
	if uptime != "" {
		s += lipgloss.NewStyle().Foreground(t.Dim).Render(" · " + uptime)
	}
	switch c.Health {
	case docker.HealthHealthy:
		s += " · " + lipgloss.NewStyle().Foreground(t.Healthy).Render("healthy")
	case docker.HealthUnhealthy:
		s += " · " + lipgloss.NewStyle().Foreground(t.Problem).Render("unhealthy")
	case docker.HealthStarting:
		s += " · " + lipgloss.NewStyle().Foreground(t.Warn).Render("starting")
	}
	return s
}

func dotFor(c docker.Container, t Theme) string {
	color := t.Healthy
	switch {
	case c.Status == docker.StatusExited || c.Status == docker.StatusDead:
		color = t.Problem
	case c.Health == docker.HealthUnhealthy:
		color = t.Warn
	case c.Status == docker.StatusPaused:
		color = t.Warn
	}
	return lipgloss.NewStyle().Foreground(color).Render("●")
}
