package tui

import (
	"fmt"

	"strava-fitness/internal/service"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatsModel is the period stats screen model
type StatsModel struct {
	queryService *service.QueryService
	stats        []service.PeriodStats
	periodType   string // "weekly" or "monthly"
	loading      bool
	err          error
}

// NewStatsModel creates a new stats model
func NewStatsModel(qs *service.QueryService) StatsModel {
	return StatsModel{
		queryService: qs,
		periodType:   "weekly",
		loading:      true,
	}
}

// Init initializes the stats screen
func (m StatsModel) Init() tea.Cmd {
	return m.loadStats
}

type statsLoadedMsg struct {
	stats []service.PeriodStats
	err   error
}

func (m StatsModel) loadStats() tea.Msg {
	numPeriods := 12
	if m.periodType == "monthly" {
		numPeriods = 6
	}

	stats, err := m.queryService.GetPeriodStats(m.periodType, numPeriods)
	return statsLoadedMsg{stats: stats, err: err}
}

// Update handles messages
func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.stats = msg.stats

	case tea.KeyMsg:
		switch msg.String() {
		case "w":
			if m.periodType != "weekly" {
				m.periodType = "weekly"
				m.loading = true
				return m, m.loadStats
			}
		case "m":
			if m.periodType != "monthly" {
				m.periodType = "monthly"
				m.loading = true
				return m, m.loadStats
			}
		case "r":
			m.loading = true
			return m, m.loadStats
		}
	}
	return m, nil
}

// View renders the stats screen
func (m StatsModel) View() string {
	if m.loading {
		return "\n  Loading stats..."
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err))
	}

	var sections []string

	// Title
	periodLabel := "Weekly"
	if m.periodType == "monthly" {
		periodLabel = "Monthly"
	}
	title := cardTitleStyle.Render(fmt.Sprintf("Period Stats (%s)", periodLabel))
	sections = append(sections, title)

	// Header
	header := tableHeaderStyle.Render(fmt.Sprintf("%-12s  %5s  %8s  %7s  %7s",
		"Period", "Runs", "Miles", "Avg HR", "Avg SPM"))
	sections = append(sections, header)

	// Rows - show most recent first
	for i := len(m.stats) - 1; i >= 0; i-- {
		s := m.stats[i]

		hrStr := "-"
		if s.AvgHR > 0 {
			hrStr = fmt.Sprintf("%.0f", s.AvgHR)
		}

		spmStr := "-"
		if s.AvgSPM > 0 {
			spmStr = fmt.Sprintf("%.0f", s.AvgSPM)
		}

		milesStr := "-"
		if s.TotalMiles > 0 {
			milesStr = fmt.Sprintf("%.1f", s.TotalMiles)
		}

		row := tableRowStyle.Render(fmt.Sprintf("%-12s  %5d  %8s  %7s  %7s",
			s.PeriodLabel,
			s.RunCount,
			milesStr,
			hrStr,
			spmStr,
		))
		sections = append(sections, row)
	}

	// Help
	help := statusStyle.Render("\n  'w' weekly, 'm' monthly, 'r' refresh, '1' dashboard")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
