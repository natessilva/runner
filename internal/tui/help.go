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
		{"3", "Period stats"},
		{"4 or s", "Sync screen"},
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

	// Stats keys
	statsSection := m.renderSection("Period Stats", []keyHelp{
		{"w", "Weekly view"},
		{"m", "Monthly view"},
		{"r", "Refresh"},
	})
	sections = append(sections, statsSection)

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

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	// EF
	lines = append(lines, "")
	lines = append(lines, "  "+helpKeyStyle.Render("EF (Efficiency Factor)")+" "+valueStyle.Render("Range: 1.0-2.0+"))
	lines = append(lines, "  "+mutedStyle.Render("Speed per heartbeat - higher is better."))
	lines = append(lines, "  "+mutedStyle.Render("Track over months: rising EF at same pace = improving fitness."))

	// Decoupling
	lines = append(lines, "")
	lines = append(lines, "  "+helpKeyStyle.Render("Aerobic Decoupling")+" "+valueStyle.Render("Target: <5%"))
	lines = append(lines, "  "+mutedStyle.Render("HR drift during a run - lower is better."))
	lines = append(lines, "  "+mutedStyle.Render("<5% = strong aerobic base, >10% = needs more base building."))

	// TRIMP
	lines = append(lines, "")
	lines = append(lines, "  "+helpKeyStyle.Render("TRIMP (Training Impulse)")+" "+valueStyle.Render("Range: 30-300+"))
	lines = append(lines, "  "+mutedStyle.Render("Training load = duration x intensity. Not good or bad."))
	lines = append(lines, "  "+mutedStyle.Render("Easy 30min ~40, hard 60min ~150. Use to compare workouts."))

	// CTL/ATL/TSB
	lines = append(lines, "")
	lines = append(lines, "  "+helpKeyStyle.Render("CTL (Fitness)"))
	lines = append(lines, "  "+mutedStyle.Render("42-day average of TRIMP. Your chronic training load."))

	lines = append(lines, "")
	lines = append(lines, "  "+helpKeyStyle.Render("ATL (Fatigue)"))
	lines = append(lines, "  "+mutedStyle.Render("7-day average of TRIMP. Your recent training load."))

	lines = append(lines, "")
	lines = append(lines, "  "+helpKeyStyle.Render("TSB (Form)")+" "+valueStyle.Render("= CTL - ATL"))
	lines = append(lines, "  "+mutedStyle.Render("Positive = fresh/recovered, negative = fatigued."))
	lines = append(lines, "  "+mutedStyle.Render("Race ready: +5 to +15. Heavy training: -10 to -30."))

	lines = append(lines, "")

	return strings.Join(lines, "\n")
}
