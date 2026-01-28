# Architecture

Runner is a terminal-based aerobic fitness analyzer that syncs with Strava.

## System Design

```
┌─────────────────────────────────────────────────────────────────┐
│                         TUI Layer                               │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Root Model                            │    │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌─────────┐  │    │
│  │  │ Dashboard │ │  Stats    │ │ Activities│ │  Sync   │  │    │
│  │  └───────────┘ └───────────┘ └───────────┘ └─────────┘  │    │
│  └─────────────────────────────────────────────────────────┘    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                      Service Layer                              │
│  ┌─────────────────┐  ┌─────────────────┐                       │
│  │  SyncService    │  │  QueryService   │                       │
│  │  - Orchestrates │  │  - Reads data   │                       │
│  │    API sync     │  │  - Aggregates   │                       │
│  │  - Computes     │  │  - Trends       │                       │
│  │    metrics      │  │                 │                       │
│  └─────────────────┘  └─────────────────┘                       │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                       Data Layer                                │
│  ┌─────────────────┐  ┌─────────────────┐  ┌────────────────┐   │
│  │  Strava Client  │  │   SQLite Store  │  │  Rate Limiter  │   │
│  │  - OAuth        │  │  - Activities   │  │  - 100/15min   │   │
│  │  - Activities   │  │  - Streams      │  │  - 1000/day    │   │
│  │  - Streams      │  │  - Metrics      │  │                │   │
│  └─────────────────┘  └─────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/
├── analysis/      # Fitness metric calculations
├── auth/          # Strava OAuth flow
├── config/        # Configuration loading
├── service/       # Business logic (sync, queries)
├── store/         # SQLite persistence
├── strava/        # API client
└── tui/           # Bubble Tea UI
```

## Database Schema

All data stored in `~/.runner/data.db` (SQLite).

### Core Tables

- **auth** - OAuth tokens (singleton row)
- **activities** - Activity summaries from Strava
- **streams** - Second-by-second data (time, HR, pace, cadence, etc.)
- **activity_metrics** - Computed metrics per activity (EF, decoupling, TRIMP)
- **fitness_trends** - Daily aggregated fitness metrics (CTL, ATL, TSB)
- **sync_state** - Sync cursor tracking

## Fitness Metrics

### Efficiency Factor (EF)

Measures pace-to-heart-rate efficiency. Higher values indicate better aerobic fitness.

```
EF = (average speed in m/s) / (average HR) × 100,000
```

Typical range: 1.0 - 2.0. Track over time to see aerobic gains.

### Aerobic Decoupling

Compares efficiency in first half vs second half of a run. Well-trained aerobic athletes show <5% decoupling on long steady runs.

```
Decoupling = ((EF_first_half / EF_second_half) - 1) × 100
```

### Training Load (TRIMP / CTL / ATL / TSB)

**TRIMP** (Training Impulse) quantifies workout stress using the Banister model:

```
TRIMP = duration(min) × HR_ratio × e^(1.92 × HR_ratio)
where HR_ratio = (avg_HR - resting_HR) / (max_HR - resting_HR)
```

**CTL** (Chronic Training Load) - 42-day exponential moving average of TRIMP. Represents fitness.

**ATL** (Acute Training Load) - 7-day exponential moving average of TRIMP. Represents fatigue.

**TSB** (Training Stress Balance) - CTL minus ATL. Positive = fresh, negative = fatigued.

## Strava Integration

### Rate Limits

- 100 requests per 15 minutes
- 1,000 requests per day

The app tracks usage and waits when approaching limits.

### Sync Strategy

1. Fetch activity summaries (paginated, newest first)
2. For each activity with HR data, fetch detailed streams
3. Compute metrics from stream data
4. Update fitness trend aggregates

## Tech Stack

| Component | Library |
|-----------|---------|
| TUI Framework | [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| TUI Styling | [Lip Gloss](https://github.com/charmbracelet/lipgloss) |
| Charts | [asciigraph](https://github.com/guptarohit/asciigraph) |
| Database | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go) |
| OAuth | [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) |
