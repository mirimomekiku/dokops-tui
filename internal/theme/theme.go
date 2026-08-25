package theme

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Color Palette
var (
	ColorPrimary    = lipgloss.Color("#7aa2f7") // Soft Neon Blue
	ColorSecondary  = lipgloss.Color("#bb9af7") // Lavender Purple
	ColorSuccess    = lipgloss.Color("#9ece6a") // Neon Green
	ColorWarning    = lipgloss.Color("#e0af68") // Warm Amber
	ColorDanger     = lipgloss.Color("#f7768e") // Coral Red
	ColorInfo       = lipgloss.Color("#7dcfff") // Cyan
	ColorDark       = lipgloss.Color("#1a1b26") // Deep Background
	ColorSurface    = lipgloss.Color("#24283b") // Surface / Card
	ColorSurfaceAlt = lipgloss.Color("#2f3549") // Secondary Surface
	ColorText       = lipgloss.Color("#c0caf5") // Primary Text
	ColorMuted      = lipgloss.Color("#565f89") // Muted Text
	ColorHighlight  = lipgloss.Color("#ff9e64") // Orange Accent
	ColorBorder     = lipgloss.Color("#3b4261") // Subtle Border
)

// Reusable Lipgloss Styles
var (
	AppContainer = lipgloss.NewStyle().
			Padding(0)

	HeaderStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorText).
			Bold(true).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#7aa2f7")).
			Padding(0, 1)

	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#3d59a1")).
			Padding(0, 1)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Background(ColorSurface).
				Padding(0, 1)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Background(ColorDark).
			Padding(0, 1)

	FocusedCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Background(ColorDark).
				Padding(0, 1)

	CardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	FooterStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorMuted).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight)

	DescStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	BadgeSuccess = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorSuccess).
			Bold(true).
			Padding(0, 1)

	BadgeDanger = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorDanger).
			Bold(true).
			Padding(0, 1)

	BadgeWarning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorWarning).
			Bold(true).
			Padding(0, 1)

	BadgeInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorInfo).
			Bold(true).
			Padding(0, 1)

	BadgeSecondary = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorSecondary).
			Bold(true).
			Padding(0, 1)

	BadgePrimary = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorPrimary).
			Bold(true).
			Padding(0, 1)
)

// RenderProgressBar returns a stylized progress bar
func RenderProgressBar(width int, percent float64, filledColor, emptyColor lipgloss.Color) string {
	if width <= 0 {
		width = 10
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filledLen := int((percent / 100.0) * float64(width))
	if filledLen > width {
		filledLen = width
	}
	emptyLen := width - filledLen

	filledStyle := lipgloss.NewStyle().Foreground(filledColor)
	emptyStyle := lipgloss.NewStyle().Foreground(emptyColor)

	var colorToUse lipgloss.Color
	switch {
	case percent > 85:
		colorToUse = ColorDanger
	case percent > 65:
		colorToUse = ColorWarning
	default:
		colorToUse = filledColor
	}
	filledStyle = filledStyle.Foreground(colorToUse)

	return filledStyle.Render(strings.Repeat("█", filledLen)) + emptyStyle.Render(strings.Repeat("░", emptyLen))
}

// RenderGauge renders a label, bar, and percentage
func RenderGauge(label string, labelWidth int, width int, percent float64, details string) string {
	lblStyle := lipgloss.NewStyle().Width(labelWidth).Foreground(ColorText)
	valStyle := lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	bar := RenderProgressBar(width, percent, ColorSuccess, ColorSurfaceAlt)
	pctStr := valStyle.Render(fmt.Sprintf("%5.1f%%", percent))

	res := fmt.Sprintf("%s %s %s", lblStyle.Render(label), bar, pctStr)
	if details != "" {
		res += " " + detailStyle.Render(details)
	}
	return res
}

// FormatBytes formats bytes into human-readable representation
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatIntBytes formats int64 bytes
func FormatIntBytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}
	return FormatBytes(uint64(bytes))
}

// FormatDuration formats duration nicely
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2f µs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2f ms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}
