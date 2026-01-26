package tui

import (
	"context"
	"fmt"
	"strings"

	"strava-fitness/internal/service"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SyncModel is the sync screen model
type SyncModel struct {
	syncService *service.SyncService
	syncing     bool
	result      *service.SyncResult
	err         error
	done        bool
}

// NewSyncModel creates a new sync model
func NewSyncModel(ss *service.SyncService) SyncModel {
	return SyncModel{
		syncService: ss,
	}
}

// Init initializes the sync screen
func (m SyncModel) Init() tea.Cmd {
	return nil
}

// SyncDoneMsg is sent when sync finishes
type SyncDoneMsg struct {
	Result *service.SyncResult
	Err    error
}

// Update handles messages
func (m SyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SyncDoneMsg:
		m.syncing = false
		m.done = true
		m.result = msg.Result
		m.err = msg.Err
		return m, func() tea.Msg { return SyncCompleteMsg{} }

	case tea.KeyMsg:
		if !m.syncing {
			switch msg.String() {
			case "enter", "s":
				m.syncing = true
				m.done = false
				m.err = nil
				m.result = nil
				return m, m.runSync
			}
		}
	}
	return m, nil
}

func (m SyncModel) runSync() tea.Msg {
	ctx := context.Background()

	// Pass nil for progress channel - we're not showing real-time updates
	// (the channel would block if buffer fills up)
	result, syncErr := m.syncService.SyncAll(ctx, nil)

	return SyncDoneMsg{Result: result, Err: syncErr}
}

// View renders the sync screen
func (m SyncModel) View() string {
	var sections []string

	title := cardTitleStyle.Render("Strava Sync")
	sections = append(sections, title)

	if m.err != nil {
		sections = append(sections, errorStyle.Render(fmt.Sprintf("\n  Error: %v", m.err)))
		sections = append(sections, "\n"+statusStyle.Render("  Press 's' or Enter to retry"))
		return lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	if m.done && !m.syncing {
		sections = append(sections, successStyle.Render("\n  Sync complete!"))
		sections = append(sections, m.renderSummary())
		sections = append(sections, "\n"+statusStyle.Render("  Press '1' to go to dashboard"))
		return lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	if m.syncing {
		sections = append(sections, m.renderProgress())
	} else {
		sections = append(sections, m.renderStartPrompt())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m SyncModel) renderStartPrompt() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, "  This will sync your Strava activities:")
	lines = append(lines, "")
	lines = append(lines, "  1. Fetch new activities from Strava")
	lines = append(lines, "  2. Download detailed stream data")
	lines = append(lines, "  3. Compute fitness metrics")
	lines = append(lines, "")

	// Show rate limit status
	short, daily := m.syncService.RateLimitStatus()
	lines = append(lines, statusStyle.Render(fmt.Sprintf("  API limits: %d/100 (15min), %d/1000 (daily)", short, daily)))
	lines = append(lines, "")
	lines = append(lines, statusStyle.Render("  Press 's' or Enter to start sync"))

	return strings.Join(lines, "\n")
}

func (m SyncModel) renderProgress() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, "  Syncing with Strava...")
	lines = append(lines, "")
	lines = append(lines, "  1. Fetching new activities")
	lines = append(lines, "  2. Downloading stream data")
	lines = append(lines, "  3. Computing fitness metrics")
	lines = append(lines, "")
	lines = append(lines, statusStyle.Render("  This may take a moment..."))

	return strings.Join(lines, "\n")
}

func (m SyncModel) renderSummary() string {
	var lines []string

	if m.result == nil {
		return ""
	}

	r := m.result
	lines = append(lines, "")

	if r.ActivitiesStored > 0 {
		lines = append(lines, successStyle.Render(fmt.Sprintf("  %d activities synced", r.ActivitiesStored)))
	} else {
		lines = append(lines, statusStyle.Render("  No new activities"))
	}

	if r.StreamsFetched > 0 {
		lines = append(lines, successStyle.Render(fmt.Sprintf("  %d streams downloaded", r.StreamsFetched)))
	}

	if r.MetricsComputed > 0 {
		lines = append(lines, successStyle.Render(fmt.Sprintf("  %d metrics computed", r.MetricsComputed)))
	}

	if len(r.Errors) > 0 {
		lines = append(lines, "")
		lines = append(lines, warningStyle.Render(fmt.Sprintf("  %d errors occurred", len(r.Errors))))
	}

	return strings.Join(lines, "\n")
}
