package tui

import (
	"fmt"
	"strings"

	"runner/internal/service"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PredictionsModel is the race predictions screen model
type PredictionsModel struct {
	queryService *service.QueryService
	units        Units
	data         *service.PredictionsData
	viewport     viewport.Model
	loading      bool
	err          error
	width        int
	height       int
	ready        bool
}

// NewPredictionsModel creates a new predictions model
func NewPredictionsModel(qs *service.QueryService, units Units, width, height int) PredictionsModel {
	m := PredictionsModel{
		queryService: qs,
		units:        units,
		loading:      true,
		width:        width,
		height:       height,
	}

	if width > 0 && height > 0 {
		m.viewport = viewport.New(width, height-6)
		m.ready = true
	}

	return m
}

// Init initializes the predictions screen
func (m PredictionsModel) Init() tea.Cmd {
	return m.loadPredictions
}

type predictionsLoadedMsg struct {
	data *service.PredictionsData
	err  error
}

func (m PredictionsModel) loadPredictions() tea.Msg {
	data, err := m.queryService.GetRacePredictions()
	return predictionsLoadedMsg{data: data, err: err}
}

// Update handles messages
func (m PredictionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case predictionsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.data = msg.data
		if m.ready {
			m.viewport.SetContent(m.renderContent())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}
		if m.data != nil {
			m.viewport.SetContent(m.renderContent())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadPredictions
		}
	}

	// Handle viewport scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the predictions screen
func (m PredictionsModel) View() string {
	if m.loading {
		return "\n  Loading race predictions..."
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err))
	}

	if !m.ready {
		return "\n  Initializing..."
	}

	footer := statusStyle.Render("  j/k or arrows: scroll  r: refresh")

	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), footer)
}

func (m PredictionsModel) renderContent() string {
	if m.data == nil || !m.data.HasPredictions {
		return m.renderEmptyState()
	}

	var sections []string

	// Title
	sections = append(sections, "")
	sections = append(sections, cardTitleStyle.Render("Race Time Predictions"))
	sections = append(sections, "")

	// VDOT info
	sections = append(sections, m.renderVDOTInfo())

	// Predictions table
	sections = append(sections, m.renderPredictionsTable())

	// About section
	sections = append(sections, m.renderAboutSection())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m PredictionsModel) renderEmptyState() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, cardTitleStyle.Render("Race Time Predictions"))
	lines = append(lines, "")

	emptyStyle := lipgloss.NewStyle().Foreground(mutedColor)
	lines = append(lines, emptyStyle.Render("  No race predictions available yet."))
	lines = append(lines, "")
	lines = append(lines, emptyStyle.Render("  Predictions require at least one personal record from the last year."))
	lines = append(lines, emptyStyle.Render("  Run a sync to analyze your activities and generate predictions."))
	lines = append(lines, "")

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m PredictionsModel) renderVDOTInfo() string {
	var lines []string

	// VDOT value and label
	vdotStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	labelStyle := lipgloss.NewStyle().Foreground(secondaryColor)

	vdotLine := fmt.Sprintf("  VDOT: %s (%s)",
		vdotStyle.Render(fmt.Sprintf("%.1f", m.data.VDOT)),
		labelStyle.Render(m.data.VDOTLabel),
	)
	lines = append(lines, vdotLine)

	// Source PR info
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)
	sourceLine := fmt.Sprintf("  Based on: %s - %s (%s)",
		m.data.SourceCategory,
		m.data.SourceTime,
		m.data.SourceDate,
	)
	lines = append(lines, mutedStyle.Render(sourceLine))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func (m PredictionsModel) renderPredictionsTable() string {
	var lines []string

	// Section header
	divider := strings.Repeat("─", 55)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	lines = append(lines, headerStyle.Render(fmt.Sprintf("── Predicted Times %s", divider[:55-19])))

	// Table header
	tableHeaderStyle := lipgloss.NewStyle().Foreground(primaryColor)
	header := fmt.Sprintf("  %-15s  %12s  %10s  %s", "Distance", "Predicted", "Pace", "Confidence")
	lines = append(lines, tableHeaderStyle.Render(header))

	// Table rows
	for _, pred := range m.data.Predictions {
		lines = append(lines, m.formatPredictionRow(pred))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m PredictionsModel) formatPredictionRow(pred service.PredictionDisplay) string {
	// Color-code confidence
	var confStyle lipgloss.Style
	switch pred.Confidence {
	case "High":
		confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")) // green
	case "Medium":
		confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")) // amber
	case "Low":
		confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")) // red
	default:
		confStyle = lipgloss.NewStyle().Foreground(mutedColor)
	}

	return fmt.Sprintf("  %-15s  %12s  %10s  %s",
		pred.TargetLabel,
		pred.PredictedTime,
		pred.PredictedPace+"/mi",
		confStyle.Render(pred.Confidence),
	)
}

func (m PredictionsModel) renderAboutSection() string {
	var lines []string

	// Section header
	divider := strings.Repeat("─", 55)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	lines = append(lines, headerStyle.Render(fmt.Sprintf("── About These Predictions %s", divider[:55-27])))

	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	lines = append(lines, mutedStyle.Render("  Predictions use Jack Daniels' VDOT methodology."))
	lines = append(lines, mutedStyle.Render("  Confidence reflects: PR recency, distance extrapolation, fitness trends."))
	lines = append(lines, "")

	// Confidence legend
	legendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	lines = append(lines, legendStyle.Render("  Confidence Levels:"))

	highStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	medStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	lowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	lines = append(lines, fmt.Sprintf("    %s - Recent PR, minimal extrapolation", highStyle.Render("High")))
	lines = append(lines, fmt.Sprintf("    %s - Moderate extrapolation or older PR", medStyle.Render("Medium")))
	lines = append(lines, fmt.Sprintf("    %s - Large extrapolation (e.g., 5K to marathon)", lowStyle.Render("Low")))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}
