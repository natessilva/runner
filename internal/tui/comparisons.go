package tui

import (
	"fmt"

	"runner/internal/service"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ComparisonsModel is the trend comparisons screen model
type ComparisonsModel struct {
	queryService *service.QueryService
	comparisons  []service.ComparisonStats
	periodType   string // "weekly" or "monthly"
	loading      bool
	err          error
}

// NewComparisonsModel creates a new comparisons model
func NewComparisonsModel(qs *service.QueryService) ComparisonsModel {
	return ComparisonsModel{
		queryService: qs,
		periodType:   "weekly",
		loading:      true,
	}
}

// Init initializes the comparisons screen
func (m ComparisonsModel) Init() tea.Cmd {
	return m.loadComparisons
}

type comparisonsLoadedMsg struct {
	comparisons []service.ComparisonStats
	err         error
}

func (m ComparisonsModel) loadComparisons() tea.Msg {
	var comparisons []service.ComparisonStats
	var err error

	if m.periodType == "weekly" {
		comparisons, err = m.queryService.GetWeeklyComparisons()
	} else {
		comparisons, err = m.queryService.GetMonthlyComparisons()
	}

	return comparisonsLoadedMsg{comparisons: comparisons, err: err}
}

// Update handles messages
func (m ComparisonsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case comparisonsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.comparisons = msg.comparisons

	case tea.KeyMsg:
		switch msg.String() {
		case "w":
			if m.periodType != "weekly" {
				m.periodType = "weekly"
				m.loading = true
				return m, m.loadComparisons
			}
		case "m":
			if m.periodType != "monthly" {
				m.periodType = "monthly"
				m.loading = true
				return m, m.loadComparisons
			}
		case "r":
			m.loading = true
			return m, m.loadComparisons
		}
	}
	return m, nil
}

// View renders the comparisons screen
func (m ComparisonsModel) View() string {
	if m.loading {
		return "\n  Loading comparisons..."
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err))
	}

	var sections []string

	// Title with mode indicator
	modeIndicator := "[W]eekly | Monthly"
	if m.periodType == "monthly" {
		modeIndicator = "Weekly | [M]onthly"
	}
	title := cardTitleStyle.Render("Trend Comparisons") + "  " + statusStyle.Render(modeIndicator)
	sections = append(sections, title)

	if len(m.comparisons) == 0 {
		sections = append(sections, "\n  No data available. Sync some activities first.")
		return lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	// Render each comparison
	for _, comp := range m.comparisons {
		sections = append(sections, m.renderComparison(comp))
	}

	// Help
	help := statusStyle.Render("\n  w/m: weekly/monthly  r: refresh")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m ComparisonsModel) renderComparison(comp service.ComparisonStats) string {
	// Box title
	boxTitle := fmt.Sprintf("── %s ", comp.Label)
	titleLine := metricLabelStyle.Render(boxTitle)

	// Header row
	header := fmt.Sprintf("                    %-14s  %-14s  %s",
		comp.Current.PeriodLabel,
		comp.Previous.PeriodLabel,
		"Delta")
	headerLine := tableHeaderStyle.Render(header)

	// Data rows
	rows := []string{
		m.renderRow("Runs", fmt.Sprintf("%d", comp.Current.RunCount), fmt.Sprintf("%d", comp.Previous.RunCount), comp.DeltaRuns, false),
		m.renderRow("Miles", formatMiles(comp.Current.TotalMiles), formatMiles(comp.Previous.TotalMiles), comp.DeltaMiles, false),
		m.renderRow("Avg HR", formatHR(comp.Current.AvgHR), formatHR(comp.Previous.AvgHR), comp.DeltaHR, true),
		m.renderRow("Avg Cadence", formatSPM(comp.Current.AvgSPM), formatSPM(comp.Previous.AvgSPM), comp.DeltaSPM, false),
		m.renderRow("Avg EF", formatEF(comp.Current.AvgEF), formatEF(comp.Previous.AvgEF), comp.DeltaEF, false),
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		titleLine,
		headerLine,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)
}

func (m ComparisonsModel) renderRow(label, current, previous string, delta interface{}, invertColor bool) string {
	var deltaStr string
	var trend int // -1 = down, 0 = flat, 1 = up

	switch d := delta.(type) {
	case int:
		if d > 0 {
			deltaStr = fmt.Sprintf("+%d", d)
			trend = 1
		} else if d < 0 {
			deltaStr = fmt.Sprintf("%d", d)
			trend = -1
		} else {
			deltaStr = "0"
			trend = 0
		}
	case float64:
		if d > 0.005 {
			deltaStr = fmt.Sprintf("+%.1f", d)
			trend = 1
		} else if d < -0.005 {
			deltaStr = fmt.Sprintf("%.1f", d)
			trend = -1
		} else {
			deltaStr = "0"
			trend = 0
		}
	}

	// For HR, lower is better, so invert the trend direction
	if invertColor && trend != 0 {
		trend = -trend
	}

	// Add trend arrow and apply style
	var styledDelta string
	switch trend {
	case 1:
		styledDelta = trendUpStyle.Render(deltaStr + " ↑")
	case -1:
		styledDelta = trendDownStyle.Render(deltaStr + " ↓")
	default:
		styledDelta = trendFlatStyle.Render(deltaStr + " →")
	}

	row := fmt.Sprintf("  %-16s  %-14s  %-14s  %s",
		label,
		current,
		previous,
		styledDelta)

	return tableRowStyle.Render(row)
}

func formatMiles(m float64) string {
	if m == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", m)
}

func formatHR(hr float64) string {
	if hr == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f", hr)
}

func formatSPM(spm float64) string {
	if spm == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f", spm)
}

func formatEF(ef float64) string {
	if ef == 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", ef)
}
