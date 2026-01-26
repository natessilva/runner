package tui

import (
	"strava-fitness/internal/service"
	"strava-fitness/internal/store"
	"strava-fitness/internal/strava"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen identifiers
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenActivities
	ScreenStats
	ScreenSync
	ScreenHelp
)

// App is the root Bubble Tea model
type App struct {
	screen     Screen
	prevScreen Screen

	// Screen models
	dashboard  DashboardModel
	activities ActivitiesModel
	stats      StatsModel
	syncScreen SyncModel
	help       HelpModel

	// Services
	db          *store.DB
	queryService *service.QueryService
	syncService  *service.SyncService
	stravaClient *strava.Client

	// Window dimensions
	width  int
	height int

	// Status message
	status string
}

// NewApp creates a new App with all dependencies
func NewApp(db *store.DB, stravaClient *strava.Client, syncService *service.SyncService, queryService *service.QueryService) *App {
	return &App{
		screen:       ScreenDashboard,
		db:           db,
		queryService: queryService,
		syncService:  syncService,
		stravaClient: stravaClient,
		dashboard:    NewDashboardModel(queryService, 0, 0),
		activities:   NewActivitiesModel(queryService),
		stats:        NewStatsModel(queryService),
		syncScreen:   NewSyncModel(syncService),
		help:         NewHelpModel(),
	}
}

// Init initializes the app
func (a *App) Init() tea.Cmd {
	return a.dashboard.Init()
}

// Update handles messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keybindings (unless in sync mode)
		if a.screen != ScreenSync || !a.syncScreen.syncing {
			switch msg.String() {
			case "q", "ctrl+c":
				return a, tea.Quit
			case "1":
				a.screen = ScreenDashboard
				a.dashboard = NewDashboardModel(a.queryService, a.width, a.height)
				return a, a.dashboard.Init()
			case "2":
				a.screen = ScreenActivities
				return a, a.activities.Init()
			case "3":
				a.screen = ScreenStats
				return a, a.stats.Init()
			case "4", "s":
				if a.screen != ScreenSync {
					a.screen = ScreenSync
					return a, a.syncScreen.Init()
				}
				// Let 's' fall through to sync screen when already there
			case "?":
				a.prevScreen = a.screen
				a.screen = ScreenHelp
				return a, nil
			case "esc":
				if a.screen == ScreenHelp {
					a.screen = a.prevScreen
					return a, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case SyncCompleteMsg:
		// Refresh dashboard after sync
		a.screen = ScreenDashboard
		a.dashboard = NewDashboardModel(a.queryService, a.width, a.height)
		return a, a.dashboard.Init()
	}

	// Delegate to current screen
	var cmd tea.Cmd
	switch a.screen {
	case ScreenDashboard:
		var m tea.Model
		m, cmd = a.dashboard.Update(msg)
		a.dashboard = m.(DashboardModel)
	case ScreenActivities:
		var m tea.Model
		m, cmd = a.activities.Update(msg)
		a.activities = m.(ActivitiesModel)
	case ScreenStats:
		var m tea.Model
		m, cmd = a.stats.Update(msg)
		a.stats = m.(StatsModel)
	case ScreenSync:
		var m tea.Model
		m, cmd = a.syncScreen.Update(msg)
		a.syncScreen = m.(SyncModel)
	case ScreenHelp:
		var m tea.Model
		m, cmd = a.help.Update(msg)
		a.help = m.(HelpModel)
	}

	return a, cmd
}

// View renders the app
func (a *App) View() string {
	header := a.renderHeader()
	nav := a.renderNav()

	var content string
	switch a.screen {
	case ScreenDashboard:
		content = a.dashboard.View()
	case ScreenActivities:
		content = a.activities.View()
	case ScreenStats:
		content = a.stats.View()
	case ScreenSync:
		content = a.syncScreen.View()
	case ScreenHelp:
		content = a.help.View()
	}

	footer := a.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, nav, content, footer)
}

func (a *App) renderHeader() string {
	return headerStyle.Render("Strava Aerobic Fitness Analyzer")
}

func (a *App) renderNav() string {
	items := []struct {
		key    string
		label  string
		screen Screen
	}{
		{"1", "Dashboard", ScreenDashboard},
		{"2", "Activities", ScreenActivities},
		{"3", "Stats", ScreenStats},
		{"4", "Sync", ScreenSync},
		{"?", "Help", ScreenHelp},
	}

	var nav string
	for i, item := range items {
		if i > 0 {
			nav += "  "
		}

		label := "[" + item.key + "] " + item.label
		if a.screen == item.screen {
			nav += navActiveStyle.Render(label)
		} else {
			nav += navInactiveStyle.Render(label)
		}
	}

	nav += "  " + navInactiveStyle.Render("[q] Quit")

	return navStyle.Render(nav)
}

func (a *App) renderFooter() string {
	if a.status != "" {
		return statusStyle.Render(a.status)
	}
	return ""
}

// SyncCompleteMsg is sent when sync finishes
type SyncCompleteMsg struct{}
