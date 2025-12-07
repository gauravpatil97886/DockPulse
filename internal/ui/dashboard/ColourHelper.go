package dashboard

import "github.com/gdamore/tcell/v2"

// Color constants for consistent UI theming
var (
	// Primary colors
	ColorRed     = tcell.NewRGBColor(255, 0, 0)
	ColorGreen   = tcell.NewRGBColor(0, 255, 0)
	ColorBlue    = tcell.NewRGBColor(0, 0, 255)
	ColorYellow  = tcell.NewRGBColor(255, 255, 0)
	ColorCyan    = tcell.NewRGBColor(0, 255, 255)
	ColorMagenta = tcell.NewRGBColor(255, 0, 255)
	ColorWhite   = tcell.NewRGBColor(255, 255, 255)
	ColorBlack   = tcell.NewRGBColor(0, 0, 0)
	ColorGray    = tcell.NewRGBColor(128, 128, 128)

	// Extended colors
	ColorOrange = tcell.NewRGBColor(255, 165, 0)
	ColorPurple = tcell.NewRGBColor(128, 0, 128)
	ColorPink   = tcell.NewRGBColor(255, 192, 203)
	ColorBrown  = tcell.NewRGBColor(165, 42, 42)
	ColorTeal   = tcell.NewRGBColor(0, 128, 128)
	ColorLime   = tcell.NewRGBColor(0, 255, 0)

	// UI specific colors
	ColorDodgerBlue    = tcell.NewRGBColor(30, 144, 255)
	ColorMediumPurple  = tcell.NewRGBColor(147, 112, 219)
	ColorDarkSlateGray = tcell.NewRGBColor(47, 79, 79)

	// Status colors
	ColorSuccess = tcell.NewRGBColor(0, 255, 0)   // Green
	ColorError   = tcell.NewRGBColor(255, 0, 0)   // Red
	ColorWarning = tcell.NewRGBColor(255, 165, 0) // Orange
	ColorInfo    = tcell.NewRGBColor(0, 255, 255) // Cyan
	ColorRunning = tcell.NewRGBColor(0, 255, 0)   // Green
	ColorStopped = tcell.NewRGBColor(255, 0, 0)   // Red
)
