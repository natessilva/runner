package tui

import (
	"fmt"

	"strava-fitness/internal/service"

	"github.com/charmbracelet/bubbles/viewport"
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
	viewport     viewport.Model
	ready        bool
	width        int
	height       int
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel(qs *service.QueryService, width, height int) DashboardModel {
	m := DashboardModel{
		queryService: qs,
		loading:      true,
		width:        width,
		height:       height,
	}

	// Initialize viewport if we have dimensions
	if width > 0 && height > 0 {
		viewportHeight := height - 6
		if viewportHeight < 10 {
			viewportHeight = 10
		}
		m.viewport = viewport.New(width, viewportHeight)
		m.ready = true
	}

	return m
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
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case dashboardDataMsg:
		m.loading = false
		m.err = msg.err
		m.data = msg.data
		if m.ready {
			m.viewport.SetContent(m.renderContent())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for header/nav/footer (approximately 6 lines)
		viewportHeight := msg.Height - 6
		if viewportHeight < 10 {
			viewportHeight = 10
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
			m.viewport.SetContent(m.renderContent())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadData
		}
	}

	// Handle viewport scrolling
	if m.ready {
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return m, cmd
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

	// If viewport not ready yet, show content directly without scrolling
	if !m.ready {
		return m.renderContent() + "\n" + statusStyle.Render("  r to refresh, s to sync")
	}

	// Show scroll indicator
	scrollPct := m.viewport.ScrollPercent() * 100
	scrollInfo := statusStyle.Render(fmt.Sprintf("  scroll: %.0f%% (j/k or arrows to scroll, r to refresh)", scrollPct))

	return m.viewport.View() + "\n" + scrollInfo
}

func (m DashboardModel) renderContent() string {
	if m.loading || m.data == nil {
		return ""
	}

	// Build the dashboard layout
	var sections []string

	// Top row: Current Fitness and This Week side by side
	fitnessCard := m.renderFitnessCard()
	weekCard := m.renderWeekCard()
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, fitnessCard, "  ", weekCard)
	sections = append(sections, topRow)

	// Charts row 1: EF and Weekly Mileage side by side
	var chartsRow1 []string
	if len(m.data.EFHistory) > 2 {
		chartsRow1 = append(chartsRow1, m.renderEFChart())
	}
	if len(m.data.WeeklyMileage) > 0 {
		chartsRow1 = append(chartsRow1, m.renderMileageChart())
	}
	if len(chartsRow1) > 0 {
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, chartsRow1...))
	}

	// Charts row 2: Cadence and HR trends
	var chartsRow2 []string
	if len(m.data.WeeklyAvgCadence) > 0 && hasNonZero(m.data.WeeklyAvgCadence) {
		chartsRow2 = append(chartsRow2, m.renderCadenceChart())
	}
	if len(m.data.WeeklyAvgHR) > 0 && hasNonZero(m.data.WeeklyAvgHR) {
		chartsRow2 = append(chartsRow2, m.renderHRChart())
	}
	if len(chartsRow2) > 0 {
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, chartsRow2...))
	}

	// Recent activities
	activities := m.renderRecentActivities()
	sections = append(sections, activities)

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

func (m DashboardModel) renderEFChart() string {
	title := cardTitleStyle.Render("Efficiency Factor Trend")

	graph := asciigraph.Plot(m.data.EFHistory,
		asciigraph.Height(6),
		asciigraph.Width(35),
		asciigraph.Precision(2),
	)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, graph))
}

func (m DashboardModel) renderMileageChart() string {
	title := cardTitleStyle.Render("Weekly Mileage (12 weeks)")

	data := trimTrailingZeros(m.data.WeeklyMileage)
	graph := asciigraph.Plot(data,
		asciigraph.Height(6),
		asciigraph.Width(35),
		asciigraph.Precision(0),
		asciigraph.Caption("miles/week"),
	)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, graph))
}

func (m DashboardModel) renderCadenceChart() string {
	title := cardTitleStyle.Render("Weekly Avg Cadence (12 weeks)")

	data := trimTrailingZeros(m.data.WeeklyAvgCadence)
	graph := asciigraph.Plot(data,
		asciigraph.Height(6),
		asciigraph.Width(35),
		asciigraph.Precision(0),
		asciigraph.Caption("spm"),
	)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, graph))
}

func (m DashboardModel) renderHRChart() string {
	title := cardTitleStyle.Render("Weekly Avg HR (12 weeks)")

	data := trimTrailingZeros(m.data.WeeklyAvgHR)
	graph := asciigraph.Plot(data,
		asciigraph.Height(6),
		asciigraph.Width(35),
		asciigraph.Precision(0),
		asciigraph.Caption("bpm"),
	)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, graph))
}

func hasNonZero(data []float64) bool {
	for _, v := range data {
		if v > 0 {
			return true
		}
	}
	return false
}

// trimTrailingZeros removes trailing zero values from a slice
func trimTrailingZeros(data []float64) []float64 {
	end := len(data)
	for end > 0 && data[end-1] == 0 {
		end--
	}
	if end == 0 {
		return data // Return original if all zeros
	}
	return data[:end]
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
