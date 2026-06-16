package ui

import "fmt"

// HumanBytes renders a byte count compactly: 512B, 1.5K, 128M, 2.0G.
func HumanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	val := float64(b) / float64(div)
	suffix := []string{"K", "M", "G", "T", "P"}[exp]
	if val >= 100 {
		return fmt.Sprintf("%.0f%s", val, suffix)
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}
