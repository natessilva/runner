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
	progress    service.SyncProgress
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

// SyncProgressMsg is sent during sync to update progress
type SyncProgressMsg struct {
	Progress service.SyncProgress
}

// SyncDoneMsg is sent when sync finishes
type SyncDoneMsg struct {
	Result *service.SyncResult
	Err    error
}

// Update handles messages
func (m SyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SyncProgressMsg:
		m.progress = msg.Progress

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
	progressChan := make(chan service.SyncProgress, 100)
	ctx := context.Background()

	var result *service.SyncResult
	var syncErr error

	// Start sync in goroutine - note: we can't send progress updates in real-time
	// in this simple model, so we just run to completion
	result, syncErr = m.syncService.SyncAll(ctx, progressChan)

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
	p := m.progress

	lines = append(lines, "")
	lines = append(lines, "  Syncing...")
	lines = append(lines, "")

	// Phase indicator
	phases := []string{"activities", "streams", "metrics"}
	phaseNames := map[string]string{
		"activities": "Fetching Activities",
		"streams":    "Downloading Streams",
		"metrics":    "Computing Metrics",
	}

	for _, phase := range phases {
		prefix := "  "
		style := statusStyle
		if phase == p.Phase {
			prefix = "> "
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		}
		lines = append(lines, style.Render(prefix+phaseNames[phase]))
	}

	lines = append(lines, "")

	// Current phase progress
	if p.Total > 0 {
		pct := float64(p.Completed) / float64(p.Total)
		bar := RenderProgressBar(pct, 40)
		lines = append(lines, fmt.Sprintf("  %s  %d/%d", bar, p.Completed, p.Total))
	}

	if p.CurrentActivity != "" {
		lines = append(lines, "")
		lines = append(lines, statusStyle.Render("  "+p.CurrentActivity))
	}

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
