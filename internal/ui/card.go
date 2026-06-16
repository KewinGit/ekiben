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
			sl := statusLine(in, t)
			if contains(in.Fields, "uptime") {
				if up := uptimeStr(in.Container); up != "" {
					sl += lipgloss.NewStyle().Foreground(t.Dim).Render(" · " + up)
				}
			}
			lines = append(lines, sl)
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
			lines = append(lines, fmt.Sprintf("%s %s", lbl.Render("img"), in.Container.Image))
		case "pids":
			lines = append(lines, fmt.Sprintf("%s %d", lbl.Render("pids"), in.Stats.PIDs))
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
	dot := dotFor(in.Container, t)
	marker := ""
	if in.Selected {
		marker = lipgloss.NewStyle().Foreground(t.Selected).Render(" ►")
	}
	return fmt.Sprintf("%s %s%s", dot, in.Container.Name, marker)
}

func statusLine(in CardInput, t Theme) string {
	c := in.Container
	switch c.Status {
	case docker.StatusExited:
		return lipgloss.NewStyle().Foreground(t.Problem).Render(fmt.Sprintf("exited (%d)", c.ExitCode))
	case docker.StatusRestarting:
		return lipgloss.NewStyle().Foreground(t.Problem).Render("restarting")
	case docker.StatusPaused:
		return lipgloss.NewStyle().Foreground(t.Warn).Render("paused")
	}
	switch c.Health {
	case docker.HealthHealthy:
		return "up · " + lipgloss.NewStyle().Foreground(t.Healthy).Render("healthy")
	case docker.HealthUnhealthy:
		return "up · " + lipgloss.NewStyle().Foreground(t.Problem).Render("unhealthy")
	case docker.HealthStarting:
		return "up · " + lipgloss.NewStyle().Foreground(t.Warn).Render("starting")
	default:
		return "up"
	}
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
