package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModel is the help screen model
type HelpModel struct{}

// NewHelpModel creates a new help model
func NewHelpModel() HelpModel {
	return HelpModel{}
}

// Init initializes the help screen
func (m HelpModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m HelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders the help screen
func (m HelpModel) View() string {
	var sections []string

	title := cardTitleStyle.Render("Keyboard Shortcuts")
	sections = append(sections, title)

	// Navigation section
	navSection := m.renderSection("Navigation", []keyHelp{
		{"1", "Dashboard"},
		{"2", "Activities list"},
		{"3 or s", "Sync screen"},
		{"?", "Help (this screen)"},
		{"q", "Quit"},
		{"esc", "Back / close help"},
	})
	sections = append(sections, navSection)

	// Dashboard keys
	dashSection := m.renderSection("Dashboard", []keyHelp{
		{"r", "Refresh data"},
	})
	sections = append(sections, dashSection)

	// Activities keys
	actSection := m.renderSection("Activities List", []keyHelp{
		{"j / down", "Move cursor down"},
		{"k / up", "Move cursor up"},
		{"pgdn", "Next page"},
		{"pgup", "Previous page"},
		{"r", "Refresh list"},
	})
	sections = append(sections, actSection)

	// Sync keys
	syncSection := m.renderSection("Sync Screen", []keyHelp{
		{"s / enter", "Start sync"},
	})
	sections = append(sections, syncSection)

	// Metrics explanation
	metricsSection := m.renderMetricsHelp()
	sections = append(sections, metricsSection)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

type keyHelp struct {
	key  string
	desc string
}

func (m HelpModel) renderSection(title string, keys []keyHelp) string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render(title))

	for _, k := range keys {
		lines = append(lines, "  "+RenderKeyHelp(k.key, k.desc))
	}

	return strings.Join(lines, "\n")
}

func (m HelpModel) renderMetricsHelp() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render("Metrics Explained"))
	lines = append(lines, "")

	metrics := []struct {
		name string
		desc string
	}{
		{"EF (Efficiency Factor)", "Speed per heartbeat. Higher = more efficient aerobic system."},
		{"Decoupling", "HR drift vs pace over time. <5% = good aerobic base."},
		{"TRIMP", "Training impulse - combines duration and intensity."},
		{"CTL (Fitness)", "Chronic training load - 42 day avg of TRIMP."},
		{"ATL (Fatigue)", "Acute training load - 7 day avg of TRIMP."},
		{"TSB (Form)", "Training stress balance = CTL - ATL. Positive = fresh."},
	}

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	for _, metric := range metrics {
		lines = append(lines, "  "+helpKeyStyle.Render(metric.name))
		lines = append(lines, "  "+mutedStyle.Render(metric.desc))
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
