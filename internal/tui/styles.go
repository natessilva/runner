package tui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	bgColor        = lipgloss.Color("#1F2937") // Dark gray
	textColor      = lipgloss.Color("#F9FAFB") // Light gray
)

// Styles
var (
	// App chrome
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			MarginBottom(1)

	// Navigation
	navStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1)

	navActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	navInactiveStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	// Cards and boxes
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(1, 2)

	cardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Metrics
	metricLabelStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Width(20)

	metricValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor)

	// Trends
	trendUpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	trendDownStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	trendFlatStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Table
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				BorderBottom(true).
				BorderForeground(mutedColor).
				Padding(0, 1)

	tableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	tableSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Background(primaryColor).
				Foreground(textColor).
				Padding(0, 1)

	// Status
	statusStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	successStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// Help
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Progress bar
	progressFullStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(mutedColor)
)

// Helper functions

// RenderMetric renders a metric with label, value, and optional trend
func RenderMetric(label, value, trend string) string {
	trendStyle := trendFlatStyle
	if len(trend) > 0 {
		first := []rune(trend)[0]
		switch first {
		case '+', '↑':
			trendStyle = trendUpStyle
		case '-', '↓':
			trendStyle = trendDownStyle
		}
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		metricLabelStyle.Render(label),
		metricValueStyle.Render(value),
		trendStyle.Render(" "+trend),
	)
}

// RenderProgressBar renders an ASCII progress bar
func RenderProgressBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += progressFullStyle.Render("█")
		} else {
			bar += progressEmptyStyle.Render("░")
		}
	}
	return bar
}

// RenderKeyHelp renders a key binding help item
func RenderKeyHelp(key, desc string) string {
	return helpKeyStyle.Render(key) + " " + helpDescStyle.Render(desc)
}
