package tui

import (
	"fmt"

	"strava-fitness/internal/service"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ActivitiesModel is the activities list screen model
type ActivitiesModel struct {
	queryService *service.QueryService
	activities   []service.ActivityWithMetrics
	cursor       int
	offset       int
	total        int
	pageSize     int
	loading      bool
	err          error
}

// NewActivitiesModel creates a new activities model
func NewActivitiesModel(qs *service.QueryService) ActivitiesModel {
	return ActivitiesModel{
		queryService: qs,
		pageSize:     15,
		loading:      true,
	}
}

// Init initializes the activities screen
func (m ActivitiesModel) Init() tea.Cmd {
	return m.loadPage
}

type activitiesLoadedMsg struct {
	activities []service.ActivityWithMetrics
	total      int
	err        error
}

func (m ActivitiesModel) loadPage() tea.Msg {
	activities, err := m.queryService.GetActivitiesList(m.pageSize, m.offset)
	if err != nil {
		return activitiesLoadedMsg{err: err}
	}

	total, err := m.queryService.GetTotalActivityCount()
	if err != nil {
		return activitiesLoadedMsg{err: err}
	}

	return activitiesLoadedMsg{activities: activities, total: total}
}

// Update handles messages
func (m ActivitiesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case activitiesLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.activities = msg.activities
		m.total = msg.total

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if m.offset > 0 {
				// Go to previous page
				m.offset -= m.pageSize
				m.cursor = m.pageSize - 1
				m.loading = true
				return m, m.loadPage
			}
		case "down", "j":
			if m.cursor < len(m.activities)-1 {
				m.cursor++
			} else if m.offset+len(m.activities) < m.total {
				// Go to next page
				m.offset += m.pageSize
				m.cursor = 0
				m.loading = true
				return m, m.loadPage
			}
		case "pgup":
			if m.offset > 0 {
				m.offset -= m.pageSize
				if m.offset < 0 {
					m.offset = 0
				}
				m.cursor = 0
				m.loading = true
				return m, m.loadPage
			}
		case "pgdown":
			if m.offset+m.pageSize < m.total {
				m.offset += m.pageSize
				m.cursor = 0
				m.loading = true
				return m, m.loadPage
			}
		case "r":
			m.loading = true
			return m, m.loadPage
		case "enter":
			if len(m.activities) > 0 && m.cursor < len(m.activities) {
				activityID := m.activities[m.cursor].Activity.ID
				return m, func() tea.Msg {
					return OpenActivityDetailMsg{ActivityID: activityID}
				}
			}
		}
	}
	return m, nil
}

// View renders the activities list
func (m ActivitiesModel) View() string {
	if m.loading {
		return "\n  Loading activities..."
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err))
	}

	if len(m.activities) == 0 {
		return "\n  No activities found. Press 's' to sync with Strava."
	}

	var sections []string

	// Title with pagination info
	startNum := m.offset + 1
	endNum := m.offset + len(m.activities)
	title := cardTitleStyle.Render(fmt.Sprintf("Activities (%d-%d of %d)", startNum, endNum, m.total))
	sections = append(sections, title)

	// Header
	header := tableHeaderStyle.Render(fmt.Sprintf("   %-10s  %-25s  %8s  %6s  %7s  %7s  %6s",
		"Date", "Name", "Distance", "Pace", "EF", "Decouple", "TRIMP"))
	sections = append(sections, header)

	// Rows
	for i, am := range m.activities {
		a := am.Activity
		met := am.Metrics

		// Calculate pace
		pace := "-"
		if a.MovingTime > 0 && a.Distance > 0 {
			paceSecsPerMile := float64(a.MovingTime) / (a.Distance / 1609.34)
			paceMin := int(paceSecsPerMile) / 60
			paceSec := int(paceSecsPerMile) % 60
			pace = fmt.Sprintf("%d:%02d", paceMin, paceSec)
		}

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

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		row := fmt.Sprintf("%s%-10s  %-25s  %7.1fmi  %6s  %6s  %7s  %6s",
			cursor,
			a.StartDateLocal.Format("Jan 02"),
			truncateName(a.Name, 25),
			a.Distance/1609.34,
			pace,
			ef,
			dec,
			trimp,
		)

		if i == m.cursor {
			sections = append(sections, tableSelectedStyle.Render(row))
		} else {
			sections = append(sections, tableRowStyle.Render(row))
		}
	}

	// Help
	help := statusStyle.Render("\n  enter: view details  j/k: navigate  pgup/pgdn: page  r: refresh")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
