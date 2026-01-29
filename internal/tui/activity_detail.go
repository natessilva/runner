package tui

import (
	"fmt"
	"strings"

	"runner/internal/service"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

// ActivityDetailModel is the activity detail screen model
type ActivityDetailModel struct {
	queryService *service.QueryService
	units        Units
	activityID   int64
	detail       *service.ActivityDetail
	activityPRs  []service.PersonalRecordDisplay
	viewport     viewport.Model
	loading      bool
	err          error
	width        int
	height       int
	ready        bool
}

// NewActivityDetailModel creates a new activity detail model
func NewActivityDetailModel(qs *service.QueryService, units Units, activityID int64, width, height int) ActivityDetailModel {
	m := ActivityDetailModel{
		queryService: qs,
		units:        units,
		activityID:   activityID,
		loading:      true,
		width:        width,
		height:       height,
	}

	if width > 0 && height > 0 {
		m.viewport = viewport.New(width, height-6) // Reserve space for header/footer
		m.ready = true
	}

	return m
}

// Init initializes the activity detail screen
func (m ActivityDetailModel) Init() tea.Cmd {
	return m.loadDetail
}

type activityDetailLoadedMsg struct {
	detail *service.ActivityDetail
	prs    []service.PersonalRecordDisplay
	err    error
}

func (m ActivityDetailModel) loadDetail() tea.Msg {
	detail, err := m.queryService.GetActivityDetailByID(m.activityID)
	if err != nil {
		return activityDetailLoadedMsg{detail: nil, prs: nil, err: err}
	}

	// Also load PRs for this activity (non-fatal if this fails)
	prs, err := m.queryService.GetActivityPRs(m.activityID)
	if err != nil {
		// PRs are supplementary - still show activity detail even if PRs fail to load
		return activityDetailLoadedMsg{detail: detail, prs: nil, err: nil}
	}
	return activityDetailLoadedMsg{detail: detail, prs: prs, err: nil}
}

// Update handles messages
func (m ActivityDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case activityDetailLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.detail = msg.detail
		m.activityPRs = msg.prs
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
		if m.detail != nil {
			m.viewport.SetContent(m.renderContent())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadDetail
		}
	}

	// Handle viewport scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the activity detail screen
func (m ActivityDetailModel) View() string {
	if m.loading {
		return "\n  Loading activity details..."
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err))
	}

	if !m.ready {
		return "\n  Initializing..."
	}

	// Footer with help
	footer := statusStyle.Render("  esc: back to list  j/k or arrows: scroll  r: refresh")

	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), footer)
}

func (m ActivityDetailModel) renderContent() string {
	if m.detail == nil {
		return "No data"
	}

	var sections []string

	// Activity header
	sections = append(sections, m.renderHeader())

	// Summary metrics
	sections = append(sections, m.renderSummary())

	// Mile splits
	if len(m.detail.Splits) > 0 {
		sections = append(sections, m.renderSplits())
	}

	// HR zones
	if len(m.detail.HRZones) > 0 {
		sections = append(sections, m.renderHRZones())
	}

	// Pace chart
	if len(m.detail.PaceData) > 5 {
		sections = append(sections, m.renderPaceChart())
	}

	// HR chart
	if len(m.detail.HRData) > 5 {
		sections = append(sections, m.renderHRChart())
	}

	// PRs achieved during this activity
	if len(m.activityPRs) > 0 {
		sections = append(sections, m.renderActivityPRs())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m ActivityDetailModel) renderHeader() string {
	a := m.detail.Activity.Activity
	title := cardTitleStyle.Render(a.Name)

	// Date and basic stats
	date := a.StartDateLocal.Format("Monday, January 2, 2006 at 3:04 PM")
	duration := formatDuration(a.MovingTime)
	pace := m.units.FormatPaceWithUnit(a.MovingTime, a.Distance)

	subtitle := lipgloss.NewStyle().Foreground(mutedColor).Render(date)

	stats := fmt.Sprintf("%s  •  %s  •  %s", m.units.FormatDistance(a.Distance), duration, pace)
	statsLine := lipgloss.NewStyle().Foreground(textColor).Bold(true).Render(stats)

	return lipgloss.JoinVertical(lipgloss.Left, "", title, subtitle, statsLine, "")
}

func (m ActivityDetailModel) renderSummary() string {
	var lines []string

	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Summary"))

	met := m.detail.Activity.Metrics

	// EF
	efStr := "-"
	if met.EfficiencyFactor != nil {
		efStr = fmt.Sprintf("%.2f", *met.EfficiencyFactor)
	}
	lines = append(lines, fmt.Sprintf("  Efficiency Factor:    %s", efStr))

	// Decoupling
	decStr := "-"
	if met.AerobicDecoupling != nil {
		decStr = fmt.Sprintf("%.1f%%", *met.AerobicDecoupling)
	}
	lines = append(lines, fmt.Sprintf("  Aerobic Decoupling:   %s", decStr))

	// TRIMP
	trimpStr := "-"
	if met.TRIMP != nil {
		trimpStr = fmt.Sprintf("%.0f", *met.TRIMP)
	}
	lines = append(lines, fmt.Sprintf("  Training Impulse:     %s", trimpStr))

	// Avg HR
	if m.detail.AvgHR > 0 {
		lines = append(lines, fmt.Sprintf("  Average HR:           %.0f bpm", m.detail.AvgHR))
	}

	// Max HR
	if m.detail.MaxHR > 0 {
		lines = append(lines, fmt.Sprintf("  Max HR:               %d bpm", m.detail.MaxHR))
	}

	// Avg Cadence
	if m.detail.AvgCadence > 0 {
		lines = append(lines, fmt.Sprintf("  Average Cadence:      %.0f spm", m.detail.AvgCadence))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m ActivityDetailModel) renderSplits() string {
	var lines []string

	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Mile Splits"))

	// Header
	// Splits are calculated per mile
	header := fmt.Sprintf("  %-6s  %8s  %6s  %6s", "Mile", "Pace", "HR", "Cadence")
	lines = append(lines, lipgloss.NewStyle().Foreground(primaryColor).Render(header))
	// Note: Pace shown here is always per-mile as calculated by service

	// Find fastest split for highlighting
	fastestPace := 9999
	for _, s := range m.detail.Splits {
		if s.Duration > 0 && s.Duration < fastestPace {
			fastestPace = s.Duration
		}
	}

	for _, s := range m.detail.Splits {
		hrStr := "-"
		if s.AvgHR > 0 {
			hrStr = fmt.Sprintf("%.0f", s.AvgHR)
		}

		cadStr := "-"
		if s.AvgCad > 0 {
			cadStr = fmt.Sprintf("%.0f", s.AvgCad)
		}

		row := fmt.Sprintf("  %-6d  %8s  %6s  %6s", s.Mile, s.Pace, hrStr, cadStr)

		// Highlight fastest split
		if s.Duration == fastestPace {
			lines = append(lines, lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render(row))
		} else {
			lines = append(lines, row)
		}
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m ActivityDetailModel) renderHRZones() string {
	var lines []string

	var title string
	if m.detail.ThresholdHR > 0 {
		title = fmt.Sprintf("HR Zone Distribution (LTHR %d, max HR %d)", m.detail.ThresholdHR, m.detail.ConfiguredMax)
	} else {
		title = fmt.Sprintf("HR Zone Distribution (based on max HR %d)", m.detail.ConfiguredMax)
	}
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(title))

	zoneColors := []lipgloss.Color{
		lipgloss.Color("#10B981"), // Zone 1 - Green (recovery)
		lipgloss.Color("#3B82F6"), // Zone 2 - Blue (aerobic)
		lipgloss.Color("#F59E0B"), // Zone 3 - Amber (tempo)
		lipgloss.Color("#EF4444"), // Zone 4 - Red (threshold)
		lipgloss.Color("#9333EA"), // Zone 5 - Purple (VO2max)
	}

	maxBarWidth := 30
	for i, z := range m.detail.HRZones {
		barWidth := int(z.Percent / 100 * float64(maxBarWidth))
		if barWidth < 1 && z.Seconds > 0 {
			barWidth = 1
		}

		bar := strings.Repeat("█", barWidth)
		color := zoneColors[i%len(zoneColors)]

		timeStr := formatDuration(z.Seconds)
		label := fmt.Sprintf("  Z%d %-18s", z.Zone, z.Name)
		pct := fmt.Sprintf("%5.1f%%", z.Percent)

		line := label + lipgloss.NewStyle().Foreground(color).Render(bar) + " " + pct + " (" + timeStr + ")"
		lines = append(lines, line)
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m ActivityDetailModel) renderPaceChart() string {
	var lines []string

	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(fmt.Sprintf("Pace Over Time (%s)", m.units.PaceLabel())))

	// Filter out zeros and prepare data
	// PaceData is in min/mi, convert if user prefers min/km
	data := m.units.ConvertPaceData(m.detail.PaceData)
	if len(data) > 60 {
		// Downsample for very long runs
		data = downsample(data, 60)
	}

	// Trim trailing zeros
	data = trimTrailingZeros(data)

	if len(data) > 2 {
		chart := asciigraph.Plot(data,
			asciigraph.Height(8),
			asciigraph.Width(50),
		)
		lines = append(lines, chart)
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m ActivityDetailModel) renderHRChart() string {
	var lines []string

	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("Heart Rate Over Time (bpm)"))

	// Filter and prepare data
	data := m.detail.HRData
	if len(data) > 60 {
		data = downsample(data, 60)
	}

	// Trim trailing zeros
	data = trimTrailingZeros(data)

	if len(data) > 2 {
		chart := asciigraph.Plot(data,
			asciigraph.Height(8),
			asciigraph.Width(50),
		)
		lines = append(lines, chart)
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m ActivityDetailModel) renderActivityPRs() string {
	var lines []string

	divider := strings.Repeat("─", 56)
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(fmt.Sprintf("── PRs Set This Run %s", divider)))

	for _, pr := range m.activityPRs {
		var prType string
		if pr.IsEffort {
			prType = "Best Effort"
		} else if pr.Category == "longest_run" || pr.Category == "highest_elevation" || pr.Category == "fastest_pace" {
			prType = "Achievement"
		} else {
			prType = "Distance PR"
		}

		line := fmt.Sprintf("  %s %s: %s (%s/mi)", prType, pr.CategoryLabel, pr.Time, pr.Pace)
		lines = append(lines, lipgloss.NewStyle().Foreground(primaryColor).Render(line))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func downsample(data []float64, targetLen int) []float64 {
	if len(data) <= targetLen {
		return data
	}

	result := make([]float64, targetLen)
	ratio := float64(len(data)) / float64(targetLen)

	for i := 0; i < targetLen; i++ {
		start := int(float64(i) * ratio)
		end := int(float64(i+1) * ratio)
		if end > len(data) {
			end = len(data)
		}

		sum := 0.0
		count := 0
		for j := start; j < end; j++ {
			if data[j] > 0 {
				sum += data[j]
				count++
			}
		}
		if count > 0 {
			result[i] = sum / float64(count)
		}
	}

	return result
}
