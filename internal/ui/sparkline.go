package ui

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// Sparkline maps values to block characters scaled to `max` (or the data max
// when max<=0). One rune per value.
func Sparkline(values []float64, max float64) string {
	if len(values) == 0 {
		return ""
	}
	if max <= 0 {
		for _, v := range values {
			if v > max {
				max = v
			}
		}
	}
	if max <= 0 {
		return string([]rune{sparkBlocks[0]})[:0] + repeatRune(sparkBlocks[0], len(values))
	}
	out := make([]rune, len(values))
	last := float64(len(sparkBlocks) - 1)
	for i, v := range values {
		if v < 0 {
			v = 0
		}
		idx := int((v / max) * last)
		if idx < 0 {
			idx = 0
		}
		if idx > len(sparkBlocks)-1 {
			idx = len(sparkBlocks) - 1
		}
		out[i] = sparkBlocks[idx]
	}
	return string(out)
}

func repeatRune(r rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = r
	}
	return string(b)
}
