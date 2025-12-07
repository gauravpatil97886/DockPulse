package dashboard

import "strings"

// DrawGraph renders a horizontal ASCII graph.
// value = 0–100 percentage.
func DrawGraph(value float64, width int) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	filled := int((value / 100.0) * float64(width))
	empty := width - filled

	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}
