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
	cursor       int
	offset       int
	pageSize     int
	total        int
}

// NewStatsModel creates a new stats model
func NewStatsModel(qs *service.QueryService) StatsModel {
	return StatsModel{
		queryService: qs,
		periodType:   "weekly",
		loading:      true,
		pageSize:     15,
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
	// Load all historical data
	numPeriods := 104 // 2 years of weeks
	if m.periodType == "monthly" {
		numPeriods = 36 // 3 years of months
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
		// Filter to only periods with data
		var periodsWithData []service.PeriodStats
		for _, s := range msg.stats {
			if s.RunCount > 0 {
				periodsWithData = append(periodsWithData, s)
			}
		}
		m.stats = periodsWithData
		m.total = len(periodsWithData)
		m.cursor = 0
		m.offset = 0

	case tea.KeyMsg:
		switch msg.String() {
		case "w":
			if m.periodType != "weekly" {
				m.periodType = "weekly"
				m.loading = true
				m.cursor = 0
				m.offset = 0
				return m, m.loadStats
			}
		case "m":
			if m.periodType != "monthly" {
				m.periodType = "monthly"
				m.loading = true
				m.cursor = 0
				m.offset = 0
				return m, m.loadStats
			}
		case "r":
			m.loading = true
			return m, m.loadStats
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if m.offset > 0 {
				m.offset -= m.pageSize
				m.cursor = m.pageSize - 1
			}
		case "down", "j":
			visibleCount := m.getVisibleCount()
			if m.cursor < visibleCount-1 {
				m.cursor++
			} else if m.offset+visibleCount < m.total {
				m.offset += m.pageSize
				m.cursor = 0
			}
		case "pgup":
			if m.offset > 0 {
				m.offset -= m.pageSize
				if m.offset < 0 {
					m.offset = 0
				}
				m.cursor = 0
			}
		case "pgdown":
			if m.offset+m.pageSize < m.total {
				m.offset += m.pageSize
				m.cursor = 0
			}
		}
	}
	return m, nil
}

func (m StatsModel) getVisibleCount() int {
	remaining := m.total - m.offset
	if remaining > m.pageSize {
		return m.pageSize
	}
	return remaining
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

	// Title with pagination info
	periodLabel := "Weekly"
	if m.periodType == "monthly" {
		periodLabel = "Monthly"
	}

	if m.total == 0 {
		title := cardTitleStyle.Render(fmt.Sprintf("Period Stats (%s)", periodLabel))
		sections = append(sections, title)
		sections = append(sections, "\n  No data available. Sync some activities first.")
		return lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	startNum := m.offset + 1
	endNum := m.offset + m.getVisibleCount()
	if endNum > m.total {
		endNum = m.total
	}

	title := cardTitleStyle.Render(fmt.Sprintf("Period Stats (%s) - %d-%d of %d", periodLabel, startNum, endNum, m.total))
	sections = append(sections, title)

	// Header
	header := tableHeaderStyle.Render(fmt.Sprintf("   %-12s  %5s  %8s  %7s  %7s",
		"Period", "Runs", "Miles", "Avg HR", "Avg SPM"))
	sections = append(sections, header)

	// Reverse the data so most recent is first
	reversed := make([]service.PeriodStats, len(m.stats))
	for i, s := range m.stats {
		reversed[len(m.stats)-1-i] = s
	}

	// Rows for current page
	endIdx := m.offset + m.pageSize
	if endIdx > len(reversed) {
		endIdx = len(reversed)
	}

	for i := m.offset; i < endIdx; i++ {
		s := reversed[i]

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

		cursor := "  "
		if i-m.offset == m.cursor {
			cursor = "> "
		}

		row := fmt.Sprintf("%s%-12s  %5d  %8s  %7s  %7s",
			cursor,
			s.PeriodLabel,
			s.RunCount,
			milesStr,
			hrStr,
			spmStr,
		)

		if i-m.offset == m.cursor {
			sections = append(sections, tableSelectedStyle.Render(row))
		} else {
			sections = append(sections, tableRowStyle.Render(row))
		}
	}

	// Help
	help := statusStyle.Render("\n  w/m: weekly/monthly  j/k: navigate  pgup/pgdn: page  r: refresh")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
