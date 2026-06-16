package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Border   lipgloss.Color // normal card border
	Selected lipgloss.Color // selected card border (cyan)
	Problem  lipgloss.Color // problem border (red)
	Healthy  lipgloss.Color // green
	Warn     lipgloss.Color // yellow
	Orange   lipgloss.Color // orange (warnings to watch)
	Dim      lipgloss.Color // dim/no-check
	Accent   lipgloss.Color // ports/blue
	Header   lipgloss.Color
	Label    lipgloss.Color
}

func ThemeByName(name string) Theme {
	switch name {
	case "light":
		return Theme{
			Border: "#cccccc", Selected: "#0aa", Problem: "#c00",
			Healthy: "#070", Warn: "#a60", Orange: "#c4520a", Dim: "#888",
			Accent: "#06c", Header: "#00a", Label: "#555",
		}
	case "mono":
		return Theme{
			Border: "#666", Selected: "#fff", Problem: "#fff",
			Healthy: "#ccc", Warn: "#ccc", Orange: "#ccc", Dim: "#777",
			Accent: "#ccc", Header: "#fff", Label: "#999",
		}
	default: // dark
		return Theme{
			Border: "#30363d", Selected: "#2dd4bf", Problem: "#f85149",
			Healthy: "#3fb950", Warn: "#d29922", Orange: "#db6d28", Dim: "#6e7681",
			Accent: "#58a6ff", Header: "#58a6ff", Label: "#8b949e",
		}
	}
}
