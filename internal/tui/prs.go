package tui

import (
	"fmt"
	"strings"

	"runner/internal/service"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PRsModel is the personal records screen model
type PRsModel struct {
	queryService *service.QueryService
	units        Units
	data         *service.PRsData
	viewport     viewport.Model
	loading      bool
	err          error
	width        int
	height       int
	ready        bool
}

// NewPRsModel creates a new PRs model
func NewPRsModel(qs *service.QueryService, units Units, width, height int) PRsModel {
	m := PRsModel{
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

// Init initializes the PRs screen
func (m PRsModel) Init() tea.Cmd {
	return m.loadPRs
}

type prsLoadedMsg struct {
	data *service.PRsData
	err  error
}

func (m PRsModel) loadPRs() tea.Msg {
	data, err := m.queryService.GetPersonalRecords()
	return prsLoadedMsg{data: data, err: err}
}

// Update handles messages
func (m PRsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case prsLoadedMsg:
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
			return m, m.loadPRs
		}
	}

	// Handle viewport scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the PRs screen
func (m PRsModel) View() string {
	if m.loading {
		return "\n  Loading personal records..."
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

func (m PRsModel) renderContent() string {
	if m.data == nil {
		return "No personal records yet. Run a sync to analyze your activities."
	}

	var sections []string

	// Title
	sections = append(sections, "")
	sections = append(sections, cardTitleStyle.Render("Personal Records"))
	sections = append(sections, "")

	// Race Distances section
	if len(m.data.RaceDistancePRs) > 0 {
		sections = append(sections, m.renderRaceDistances())
	}

	// Best Efforts section
	if len(m.data.BestEffortPRs) > 0 {
		sections = append(sections, m.renderBestEfforts())
	}

	// Other Achievements section
	if len(m.data.OtherPRs) > 0 {
		sections = append(sections, m.renderOtherAchievements())
	}

	if len(m.data.RaceDistancePRs) == 0 && len(m.data.BestEffortPRs) == 0 && len(m.data.OtherPRs) == 0 {
		sections = append(sections, lipgloss.NewStyle().Foreground(mutedColor).Render("  No personal records found. Run a sync to analyze your activities."))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m PRsModel) renderRaceDistances() string {
	var lines []string

	lines = append(lines, m.sectionHeader("Race Distances"))
	lines = append(lines, m.tableHeader())

	for _, pr := range m.data.RaceDistancePRs {
		lines = append(lines, m.formatPRRow(pr))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m PRsModel) renderBestEfforts() string {
	var lines []string

	lines = append(lines, m.sectionHeader("Best Efforts"))
	lines = append(lines, m.effortTableHeader())

	for _, pr := range m.data.BestEffortPRs {
		lines = append(lines, m.formatEffortRow(pr))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m PRsModel) renderOtherAchievements() string {
	var lines []string

	lines = append(lines, m.sectionHeader("Other Achievements"))

	for _, pr := range m.data.OtherPRs {
		lines = append(lines, m.formatOtherRow(pr))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m PRsModel) sectionHeader(title string) string {
	titleLen := len([]rune(title))
	dividerLen := 60 - titleLen - 4
	if dividerLen < 0 {
		dividerLen = 0
	}
	divider := strings.Repeat("─", dividerLen)
	return lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(fmt.Sprintf("── %s %s", title, divider))
}

func (m PRsModel) tableHeader() string {
	header := fmt.Sprintf("  %-14s  %10s  %10s  %8s  %s", "Distance", "Time", "Pace", "Avg HR", "Date")
	return lipgloss.NewStyle().Foreground(primaryColor).Render(header)
}

func (m PRsModel) effortTableHeader() string {
	header := fmt.Sprintf("  %-14s  %10s  %10s  %s", "Distance", "Time", "Pace", "Source Activity")
	return lipgloss.NewStyle().Foreground(primaryColor).Render(header)
}

func (m PRsModel) formatPRRow(pr service.PersonalRecordDisplay) string {
	return fmt.Sprintf("  %-14s  %10s  %10s  %8s  %s",
		pr.CategoryLabel,
		pr.Time,
		pr.Pace+"/mi",
		pr.AvgHR,
		pr.Date,
	)
}

func (m PRsModel) formatEffortRow(pr service.PersonalRecordDisplay) string {
	activityName := pr.ActivityName
	if len(activityName) > 30 {
		activityName = activityName[:27] + "..."
	}
	return fmt.Sprintf("  %-14s  %10s  %10s  %s",
		pr.CategoryLabel,
		pr.Time,
		pr.Pace+"/mi",
		activityName,
	)
}

func (m PRsModel) formatOtherRow(pr service.PersonalRecordDisplay) string {
	var value string

	switch pr.Category {
	case "longest_run":
		value = m.units.FormatDistance(pr.DistanceMeters)
	case "highest_elevation":
		value = fmt.Sprintf("%.0f m", pr.DistanceMeters)
	case "fastest_pace":
		value = pr.Pace + "/mi"
	default:
		value = pr.Time
	}

	return fmt.Sprintf("  %-18s  %s  (%s)", pr.CategoryLabel, value, pr.Date)
}
