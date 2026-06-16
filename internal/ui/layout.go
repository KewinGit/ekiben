package ui

const (
	MinCardWidth = 20
	CardGap      = 1
)

// Columns returns how many cards of at least MinCardWidth (plus gaps) fit in width.
func Columns(width int) int {
	if width < MinCardWidth {
		return 1
	}
	n := (width + CardGap) / (MinCardWidth + CardGap)
	if n < 1 {
		return 1
	}
	return n
}

// CardWidth returns the actual width each card gets so the row fills `width`
// (cards grow beyond MinCardWidth to consume leftover space).
func CardWidth(width, columns int) int {
	if columns < 1 {
		columns = 1
	}
	totalGap := CardGap * (columns - 1)
	w := (width - totalGap) / columns
	if w < MinCardWidth {
		w = MinCardWidth
	}
	return w
}
