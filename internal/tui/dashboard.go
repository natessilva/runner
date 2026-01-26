package tui

import (
	"fmt"

	"strava-fitness/internal/service"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

// DashboardModel is the dashboard screen model
type DashboardModel struct {
	queryService *service.QueryService
	data         *service.DashboardData
	loading      bool
	err          error
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel(qs *service.QueryService) DashboardModel {
	return DashboardModel{
		queryService: qs,
		loading:      true,
	}
}

// Init initializes the dashboard
func (m DashboardModel) Init() tea.Cmd {
	return m.loadData
}

func (m DashboardModel) loadData() tea.Msg {
	data, err := m.queryService.GetDashboardData()
	if err != nil {
		return dashboardDataMsg{err: err}
	}
	return dashboardDataMsg{data: data}
}

type dashboardDataMsg struct {
	data *service.DashboardData
	err  error
}

// Update handles messages
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardDataMsg:
		m.loading = false
		m.err = msg.err
		m.data = msg.data
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadData
		}
	}
	return m, nil
}

// View renders the dashboard
func (m DashboardModel) View() string {
	if m.loading {
		return "\n  Loading dashboard..."
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err))
	}

	if m.data == nil {
		return "\n  No data available. Press 's' to sync with Strava."
	}

	// Build the dashboard layout
	var sections []string

	// Top row: Current Fitness and This Week side by side
	fitnessCard := m.renderFitnessCard()
	weekCard := m.renderWeekCard()
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, fitnessCard, "  ", weekCard)
	sections = append(sections, topRow)

	// Chart
	if len(m.data.EFHistory) > 2 {
		chart := m.renderChart()
		sections = append(sections, chart)
	}

	// Recent activities
	activities := m.renderRecentActivities()
	sections = append(sections, activities)

	// Help
	help := statusStyle.Render("Press 'r' to refresh, 's' to sync, '2' for activities list")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m DashboardModel) renderFitnessCard() string {
	title := cardTitleStyle.Render("Current Fitness")

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	lines := []string{
		RenderMetric("Efficiency Factor", fmt.Sprintf("%.2f", m.data.CurrentEF), m.data.EFTrend),
		RenderMetric("Fitness (CTL)", fmt.Sprintf("%.0f", m.data.CurrentFitness), ""),
		RenderMetric("Fatigue (ATL)", fmt.Sprintf("%.0f", m.data.CurrentFatigue), ""),
		RenderMetric("Form (TSB)", fmt.Sprintf("%.0f", m.data.CurrentForm), ""),
		"",
		mutedStyle.Render(m.data.FormDescription),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return cardStyle.Width(38).Render(lipgloss.JoinVertical(lipgloss.Left, title, content))
}

func (m DashboardModel) renderWeekCard() string {
	title := cardTitleStyle.Render("This Week")

	lines := []string{
		RenderMetric("Runs", fmt.Sprintf("%d", m.data.WeekRunCount), ""),
		RenderMetric("Distance", fmt.Sprintf("%.1f mi", m.data.WeekDistance), ""),
		RenderMetric("Time", formatDuration(m.data.WeekTime), ""),
		RenderMetric("Avg EF", fmt.Sprintf("%.2f", m.data.WeekAvgEF), ""),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return cardStyle.Width(30).Render(lipgloss.JoinVertical(lipgloss.Left, title, content))
}

func (m DashboardModel) renderChart() string {
	title := cardTitleStyle.Render("Efficiency Factor - Recent Trend")

	// Create the chart
	graph := asciigraph.Plot(m.data.EFHistory,
		asciigraph.Height(8),
		asciigraph.Width(60),
		asciigraph.Precision(2),
	)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, graph))
}

func (m DashboardModel) renderRecentActivities() string {
	title := cardTitleStyle.Render("Recent Activities")

	if len(m.data.RecentActivities) == 0 {
		return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, "No activities yet"))
	}

	// Header
	header := tableHeaderStyle.Render(fmt.Sprintf("%-10s  %-20s  %8s  %6s  %7s  %6s",
		"Date", "Name", "Distance", "EF", "Decouple", "TRIMP"))

	var rows []string
	rows = append(rows, header)

	for i, am := range m.data.RecentActivities {
		if i >= 5 {
			break
		}

		a := am.Activity
		met := am.Metrics

		ef := "-"
		if met.EfficiencyFactor != nil {
			ef = fmt.Sprintf("%.2f", *met.EfficiencyFactor)
		}

		dec := "-"
		if met.AerobicDecoupling != nil {
			dec = fmt.Sprintf("%.1f%%", *met.AerobicDecoupling)
		}

		trimp := "-"
		if met.TRIMP != nil {
			trimp = fmt.Sprintf("%.0f", *met.TRIMP)
		}

		row := tableRowStyle.Render(fmt.Sprintf("%-10s  %-20s  %7.1fmi  %6s  %7s  %6s",
			a.StartDateLocal.Format("Jan 02"),
			truncateName(a.Name, 20),
			a.Distance/1609.34,
			ef,
			dec,
			trimp,
		))
		rows = append(rows, row)
	}

	table := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, table))
}

func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func truncateName(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

