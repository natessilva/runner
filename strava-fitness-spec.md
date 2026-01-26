# Strava Aerobic Fitness Analyzer

A terminal-based application for analyzing aerobic fitness trends using Strava running data.

---

## Table of Contents

1. [Overview](#overview)
2. [Goals & Non-Goals](#goals--non-goals)
3. [Technical Stack](#technical-stack)
4. [Architecture](#architecture)
5. [Data Models](#data-models)
6. [Strava API Integration](#strava-api-integration)
7. [Analysis Algorithms](#analysis-algorithms)
8. [TUI Design](#tui-design)
9. [Project Structure](#project-structure)
10. [Implementation Guide](#implementation-guide)
11. [Future Enhancements](#future-enhancements)

---

## Overview

This application fetches running data from Strava's API, stores it locally in SQLite, computes aerobic fitness metrics, and displays trends through an interactive terminal UI built with Bubble Tea.

### Core Value Proposition

- Track aerobic fitness progression over time without relying on Strava's premium features
- Compute sport-science metrics (efficiency factor, aerobic decoupling, training load)
- Visualize trends to inform training decisions
- Single binary, runs anywhere, no cloud dependency

---

## Goals & Non-Goals

### Goals

- Sync all historical run activities with heart rate data
- Compute meaningful aerobic fitness metrics
- Display trends over configurable time ranges
- Provide activity-level drill-down for analysis
- Handle Strava API rate limits gracefully
- Work offline after initial sync

### Non-Goals

- Real-time activity tracking
- Multi-user support
- Mobile or web interface
- Integration with other platforms (Garmin, Wahoo, etc.)
- Workout prescription or coaching
- Social features

---

## Technical Stack

| Component | Library | Version | Purpose |
|-----------|---------|---------|---------|
| Language | Go | 1.22+ | Core runtime |
| TUI Framework | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | v1.x | Terminal UI architecture |
| TUI Styling | [Lip Gloss](https://github.com/charmbracelet/lipgloss) | v1.x | Component styling |
| TUI Components | [Bubbles](https://github.com/charmbracelet/bubbles) | v0.20+ | Tables, spinners, etc. |
| Charts | [asciigraph](https://github.com/guptarohit/asciigraph) | latest | Terminal sparklines |
| Database | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | latest | Pure-Go SQLite |
| OAuth | [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) | latest | Strava authentication |
| HTTP | `net/http` | stdlib | API requests |
| Time | `time` | stdlib | Date calculations |
| JSON | `encoding/json` | stdlib | API parsing |

### Why These Choices

**Bubble Tea**: Elm-architecture provides clean state management. The Charm ecosystem (Lip Gloss, Bubbles) offers cohesive styling and pre-built components.

**modernc.org/sqlite**: Pure Go implementation means no CGO, enabling easy cross-compilation and a truly single binary.

**asciigraph**: Lightweight, no dependencies, produces clean ASCII charts that render well in any terminal.

---

## Architecture

### System Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         TUI Layer                               │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Root Model                            │    │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌─────────┐  │    │
│  │  │ Dashboard │ │  Trends   │ │ Activities│ │  Sync   │  │    │
│  │  │   Model   │ │   Model   │ │   Model   │ │  Model  │  │    │
│  │  └───────────┘ └───────────┘ └───────────┘ └─────────┘  │    │
│  └─────────────────────────────────────────────────────────┘    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                      Service Layer                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌────────────────┐   │
│  │  SyncService    │  │ AnalysisService │  │  QueryService  │   │
│  │  - Orchestrates │  │  - Computes     │  │  - Reads data  │   │
│  │    API sync     │  │    metrics      │  │  - Aggregates  │   │
│  └─────────────────┘  └─────────────────┘  └────────────────┘   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                       Data Layer                                │
│  ┌─────────────────┐  ┌─────────────────┐  ┌────────────────┐   │
│  │  Strava Client  │  │   SQLite Repo   │  │  Rate Limiter  │   │
│  │  - OAuth        │  │  - Activities   │  │  - Token bucket│   │
│  │  - Activities   │  │  - Streams      │  │  - Backoff     │   │
│  │  - Streams      │  │  - Metrics      │  │                │   │
│  └─────────────────┘  └─────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
1. User launches app
         │
         ▼
2. App checks for valid OAuth token
         │
    ┌────┴────┐
    │ Valid?  │
    └────┬────┘
     No  │  Yes
    ┌────┴────────────────┐
    ▼                     ▼
3a. OAuth Flow       3b. Load cached data
    │                     │
    ▼                     ▼
4. Fetch activities  4. Display dashboard
   from Strava            │
    │                     ▼
    ▼               5. Background sync
5. Store in SQLite       for new activities
    │                     │
    ▼                     ▼
6. Fetch streams     6. Update metrics
   (rate limited)         │
    │                     ▼
    ▼               7. Re-render UI
7. Compute metrics
    │
    ▼
8. Display dashboard
```

### Package Dependencies

```
main.go
    └── internal/tui
            ├── internal/service
            │       ├── internal/strava
            │       ├── internal/store
            │       └── internal/analysis
            └── internal/store (for direct queries)
```

---

## Data Models

### SQLite Schema

```sql
-- Database: ~/.strava-fitness/data.db

----------------------------------------------------------------------
-- Authentication
----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS auth (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Singleton row
    athlete_id INTEGER NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at INTEGER NOT NULL,            -- Unix timestamp
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

----------------------------------------------------------------------
-- Activities (summary data from /athlete/activities)
----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS activities (
    id INTEGER PRIMARY KEY,                 -- Strava activity ID
    athlete_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,                     -- "Run", "Ride", etc.
    start_date TEXT NOT NULL,               -- ISO8601 UTC
    start_date_local TEXT NOT NULL,         -- ISO8601 local
    timezone TEXT,
    distance REAL NOT NULL,                 -- meters
    moving_time INTEGER NOT NULL,           -- seconds
    elapsed_time INTEGER NOT NULL,          -- seconds
    total_elevation_gain REAL,              -- meters
    average_speed REAL,                     -- m/s
    max_speed REAL,                         -- m/s
    average_heartrate REAL,                 -- bpm
    max_heartrate REAL,                     -- bpm
    average_cadence REAL,                   -- spm (steps per minute)
    suffer_score INTEGER,                   -- Strava's relative effort
    has_heartrate INTEGER NOT NULL,         -- 0 or 1
    streams_synced INTEGER DEFAULT 0,       -- 0 or 1
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activities_start_date ON activities(start_date);
CREATE INDEX idx_activities_type ON activities(type);
CREATE INDEX idx_activities_has_hr ON activities(has_heartrate);

----------------------------------------------------------------------
-- Streams (second-by-second data from /activities/{id}/streams)
----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS streams (
    activity_id INTEGER NOT NULL,
    time_offset INTEGER NOT NULL,           -- seconds from activity start
    latlng_lat REAL,                        -- latitude
    latlng_lng REAL,                        -- longitude  
    altitude REAL,                          -- meters
    velocity_smooth REAL,                   -- m/s
    heartrate INTEGER,                      -- bpm
    cadence INTEGER,                        -- spm
    grade_smooth REAL,                      -- percent grade
    distance REAL,                          -- cumulative meters
    PRIMARY KEY (activity_id, time_offset),
    FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
);

CREATE INDEX idx_streams_activity ON streams(activity_id);

----------------------------------------------------------------------
-- Computed Metrics (per activity)
----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS activity_metrics (
    activity_id INTEGER PRIMARY KEY,
    
    -- Efficiency metrics
    efficiency_factor REAL,                 -- pace/HR ratio (higher = fitter)
    aerobic_decoupling REAL,                -- % drift first vs second half
    cardiac_drift REAL,                     -- HR increase at steady pace
    
    -- Pace at HR zones (min/km at each zone midpoint)
    pace_at_z1 REAL,                        -- pace at ~60% max HR
    pace_at_z2 REAL,                        -- pace at ~70% max HR
    pace_at_z3 REAL,                        -- pace at ~80% max HR
    
    -- Training load
    trimp REAL,                             -- Training Impulse score
    hrss REAL,                              -- HR Stress Score
    
    -- Quality indicators
    data_quality_score REAL,                -- 0-1, how complete is stream data
    steady_state_pct REAL,                  -- % of run at steady effort
    
    computed_at TEXT DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
);

----------------------------------------------------------------------
-- Daily Fitness Trends
----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS fitness_trends (
    date TEXT PRIMARY KEY,                  -- YYYY-MM-DD
    
    -- Training load metrics (exponential moving averages)
    ctl REAL,                               -- Chronic Training Load (42-day)
    atl REAL,                               -- Acute Training Load (7-day)
    tsb REAL,                               -- Training Stress Balance (CTL-ATL)
    
    -- Rolling efficiency metrics
    efficiency_factor_7d REAL,              -- 7-day avg EF
    efficiency_factor_28d REAL,             -- 28-day avg EF
    efficiency_factor_90d REAL,             -- 90-day avg EF
    
    -- Activity counts
    run_count_7d INTEGER,
    total_distance_7d REAL,                 -- meters
    total_time_7d INTEGER,                  -- seconds
    
    computed_at TEXT DEFAULT CURRENT_TIMESTAMP
);

----------------------------------------------------------------------
-- Sync State
----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Keys used:
-- 'last_activity_sync' - ISO8601 timestamp of last activity list fetch
-- 'oldest_activity_fetched' - ISO8601 timestamp of oldest activity synced
-- 'streams_sync_cursor' - Activity ID where stream sync left off
```

### Go Structs

```go
// internal/store/models.go

package store

import "time"

// Auth represents OAuth tokens for Strava API access
type Auth struct {
    AthleteID    int64     `db:"athlete_id"`
    AccessToken  string    `db:"access_token"`
    RefreshToken string    `db:"refresh_token"`
    ExpiresAt    time.Time `db:"expires_at"`
}

// Activity represents a Strava activity summary
type Activity struct {
    ID                 int64     `db:"id"`
    AthleteID          int64     `db:"athlete_id"`
    Name               string    `db:"name"`
    Type               string    `db:"type"`
    StartDate          time.Time `db:"start_date"`
    StartDateLocal     time.Time `db:"start_date_local"`
    Timezone           string    `db:"timezone"`
    Distance           float64   `db:"distance"`           // meters
    MovingTime         int       `db:"moving_time"`        // seconds
    ElapsedTime        int       `db:"elapsed_time"`       // seconds
    TotalElevationGain float64   `db:"total_elevation_gain"`
    AverageSpeed       float64   `db:"average_speed"`      // m/s
    MaxSpeed           float64   `db:"max_speed"`          // m/s
    AverageHeartrate   *float64  `db:"average_heartrate"`  // nullable
    MaxHeartrate       *float64  `db:"max_heartrate"`      // nullable
    AverageCadence     *float64  `db:"average_cadence"`    // nullable
    SufferScore        *int      `db:"suffer_score"`       // nullable
    HasHeartrate       bool      `db:"has_heartrate"`
    StreamsSynced      bool      `db:"streams_synced"`
}

// StreamPoint represents a single data point from activity streams
type StreamPoint struct {
    ActivityID     int64    `db:"activity_id"`
    TimeOffset     int      `db:"time_offset"`     // seconds
    Lat            *float64 `db:"latlng_lat"`
    Lng            *float64 `db:"latlng_lng"`
    Altitude       *float64 `db:"altitude"`        // meters
    VelocitySmooth *float64 `db:"velocity_smooth"` // m/s
    Heartrate      *int     `db:"heartrate"`       // bpm
    Cadence        *int     `db:"cadence"`         // spm
    GradeSmooth    *float64 `db:"grade_smooth"`    // percent
    Distance       *float64 `db:"distance"`        // cumulative meters
}

// ActivityMetrics represents computed fitness metrics for an activity
type ActivityMetrics struct {
    ActivityID        int64    `db:"activity_id"`
    EfficiencyFactor  *float64 `db:"efficiency_factor"`
    AerobicDecoupling *float64 `db:"aerobic_decoupling"`
    CardiacDrift      *float64 `db:"cardiac_drift"`
    PaceAtZ1          *float64 `db:"pace_at_z1"`
    PaceAtZ2          *float64 `db:"pace_at_z2"`
    PaceAtZ3          *float64 `db:"pace_at_z3"`
    TRIMP             *float64 `db:"trimp"`
    HRSS              *float64 `db:"hrss"`
    DataQualityScore  *float64 `db:"data_quality_score"`
    SteadyStatePct    *float64 `db:"steady_state_pct"`
}

// FitnessTrend represents daily aggregated fitness metrics
type FitnessTrend struct {
    Date               string   `db:"date"` // YYYY-MM-DD
    CTL                *float64 `db:"ctl"`
    ATL                *float64 `db:"atl"`
    TSB                *float64 `db:"tsb"`
    EfficiencyFactor7d *float64 `db:"efficiency_factor_7d"`
    EfficiencyFactor28d *float64 `db:"efficiency_factor_28d"`
    EfficiencyFactor90d *float64 `db:"efficiency_factor_90d"`
    RunCount7d         int      `db:"run_count_7d"`
    TotalDistance7d    float64  `db:"total_distance_7d"`
    TotalTime7d        int      `db:"total_time_7d"`
}
```

---

## Strava API Integration

### OAuth 2.0 Flow

Strava uses OAuth 2.0 with authorization code grant. For a personal CLI app, we'll spin up a temporary localhost server to catch the callback.

#### Setup (One-time)

1. Go to https://www.strava.com/settings/api
2. Create an application:
   - Application Name: "Fitness Analyzer" (or whatever)
   - Category: "Training Analysis"
   - Website: http://localhost
   - Authorization Callback Domain: `localhost`
3. Note your Client ID and Client Secret

#### Auth Flow Implementation

```go
// internal/auth/oauth.go

package auth

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net/http"
    "time"

    "golang.org/x/oauth2"
)

const (
    AuthURL  = "https://www.strava.com/oauth/authorize"
    TokenURL = "https://www.strava.com/oauth/token"
)

// Scopes required for our app
var Scopes = []string{
    "read",
    "activity:read_all",
}

type Config struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string // e.g., "http://localhost:8089/callback"
}

func NewOAuthConfig(cfg Config) *oauth2.Config {
    return &oauth2.Config{
        ClientID:     cfg.ClientID,
        ClientSecret: cfg.ClientSecret,
        Endpoint: oauth2.Endpoint{
            AuthURL:  AuthURL,
            TokenURL: TokenURL,
        },
        RedirectURL: cfg.RedirectURL,
        Scopes:      Scopes,
    }
}

// AuthResult contains the token and athlete info from successful auth
type AuthResult struct {
    Token     *oauth2.Token
    AthleteID int64
}

// Authenticate runs the OAuth flow with a local callback server
func Authenticate(ctx context.Context, cfg *oauth2.Config) (*AuthResult, error) {
    // Generate state for CSRF protection
    state, err := generateState()
    if err != nil {
        return nil, fmt.Errorf("generating state: %w", err)
    }

    // Channel to receive the auth code
    codeChan := make(chan string, 1)
    errChan := make(chan error, 1)

    // Start local server
    server := &http.Server{Addr: ":8089"}
    http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Query().Get("state") != state {
            errChan <- fmt.Errorf("state mismatch")
            return
        }
        code := r.URL.Query().Get("code")
        if code == "" {
            errChan <- fmt.Errorf("no code in callback")
            return
        }
        
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprint(w, "<h1>Success!</h1><p>You can close this window.</p>")
        codeChan <- code
    })

    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            errChan <- err
        }
    }()

    // Generate auth URL and prompt user
    authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
    fmt.Printf("Open this URL in your browser:\n\n%s\n\n", authURL)

    // Wait for callback
    var code string
    select {
    case code = <-codeChan:
    case err := <-errChan:
        server.Shutdown(ctx)
        return nil, err
    case <-time.After(5 * time.Minute):
        server.Shutdown(ctx)
        return nil, fmt.Errorf("authentication timeout")
    }

    server.Shutdown(ctx)

    // Exchange code for token
    token, err := cfg.Exchange(ctx, code)
    if err != nil {
        return nil, fmt.Errorf("exchanging code: %w", err)
    }

    // Extract athlete ID from token extras
    athleteID := extractAthleteID(token)

    return &AuthResult{
        Token:     token,
        AthleteID: athleteID,
    }, nil
}

func generateState() (string, error) {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return hex.EncodeToString(b), nil
}

func extractAthleteID(token *oauth2.Token) int64 {
    // Strava includes athlete info in token response
    if athlete, ok := token.Extra("athlete").(map[string]interface{}); ok {
        if id, ok := athlete["id"].(float64); ok {
            return int64(id)
        }
    }
    return 0
}
```

#### Token Refresh

```go
// internal/auth/refresh.go

package auth

import (
    "context"
    "time"

    "golang.org/x/oauth2"
)

// TokenSource wraps oauth2.TokenSource with persistence
type TokenSource struct {
    config    *oauth2.Config
    token     *oauth2.Token
    onRefresh func(*oauth2.Token) error
}

func NewTokenSource(cfg *oauth2.Config, token *oauth2.Token, onRefresh func(*oauth2.Token) error) *TokenSource {
    return &TokenSource{
        config:    cfg,
        token:     token,
        onRefresh: onRefresh,
    }
}

func (ts *TokenSource) Token() (*oauth2.Token, error) {
    // Check if token needs refresh (with 60s buffer)
    if time.Until(ts.token.Expiry) > 60*time.Second {
        return ts.token, nil
    }

    // Refresh the token
    src := ts.config.TokenSource(context.Background(), ts.token)
    newToken, err := src.Token()
    if err != nil {
        return nil, err
    }

    // Persist the new token
    if ts.onRefresh != nil {
        if err := ts.onRefresh(newToken); err != nil {
            return nil, err
        }
    }

    ts.token = newToken
    return newToken, nil
}
```

### API Client

```go
// internal/strava/client.go

package strava

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "time"

    "golang.org/x/oauth2"
)

const BaseURL = "https://www.strava.com/api/v3"

type Client struct {
    httpClient  *http.Client
    rateLimiter *RateLimiter
}

func NewClient(tokenSource oauth2.TokenSource) *Client {
    return &Client{
        httpClient:  oauth2.NewClient(context.Background(), tokenSource),
        rateLimiter: NewRateLimiter(),
    }
}

// GetActivities fetches activities with pagination
// Returns activities after 'after' timestamp, up to 'perPage' results
func (c *Client) GetActivities(ctx context.Context, after time.Time, page, perPage int) ([]Activity, error) {
    if err := c.rateLimiter.Wait(ctx); err != nil {
        return nil, err
    }

    params := url.Values{}
    params.Set("after", strconv.FormatInt(after.Unix(), 10))
    params.Set("page", strconv.Itoa(page))
    params.Set("per_page", strconv.Itoa(perPage))

    resp, err := c.get(ctx, "/athlete/activities", params)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var activities []Activity
    if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
        return nil, fmt.Errorf("decoding activities: %w", err)
    }

    return activities, nil
}

// GetActivityStreams fetches detailed stream data for an activity
func (c *Client) GetActivityStreams(ctx context.Context, activityID int64) (*Streams, error) {
    if err := c.rateLimiter.Wait(ctx); err != nil {
        return nil, err
    }

    // Request all available stream types
    params := url.Values{}
    params.Set("keys", "time,latlng,altitude,velocity_smooth,heartrate,cadence,grade_smooth,distance")
    params.Set("key_by_type", "true")

    path := fmt.Sprintf("/activities/%d/streams", activityID)
    resp, err := c.get(ctx, path, params)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var streams Streams
    if err := json.NewDecoder(resp.Body).Decode(&streams); err != nil {
        return nil, fmt.Errorf("decoding streams: %w", err)
    }

    return &streams, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values) (*http.Response, error) {
    reqURL := BaseURL + path
    if len(params) > 0 {
        reqURL += "?" + params.Encode()
    }

    req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }

    // Update rate limiter from response headers
    c.rateLimiter.UpdateFromHeaders(resp.Header)

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }

    return resp, nil
}
```

### API Models

```go
// internal/strava/models.go

package strava

import "time"

// Activity represents a Strava activity from the API
type Activity struct {
    ID                 int64     `json:"id"`
    Athlete            Athlete   `json:"athlete"`
    Name               string    `json:"name"`
    Type               string    `json:"type"`
    StartDate          time.Time `json:"start_date"`
    StartDateLocal     time.Time `json:"start_date_local"`
    Timezone           string    `json:"timezone"`
    Distance           float64   `json:"distance"`
    MovingTime         int       `json:"moving_time"`
    ElapsedTime        int       `json:"elapsed_time"`
    TotalElevationGain float64   `json:"total_elevation_gain"`
    AverageSpeed       float64   `json:"average_speed"`
    MaxSpeed           float64   `json:"max_speed"`
    AverageHeartrate   float64   `json:"average_heartrate"`
    MaxHeartrate       float64   `json:"max_heartrate"`
    AverageCadence     float64   `json:"average_cadence"`
    SufferScore        int       `json:"suffer_score"`
    HasHeartrate       bool      `json:"has_heartrate"`
}

type Athlete struct {
    ID int64 `json:"id"`
}

// Streams represents activity stream data from the API
type Streams struct {
    Time           StreamData[int]     `json:"time"`
    LatLng         StreamData[[2]float64] `json:"latlng"`
    Altitude       StreamData[float64] `json:"altitude"`
    VelocitySmooth StreamData[float64] `json:"velocity_smooth"`
    Heartrate      StreamData[int]     `json:"heartrate"`
    Cadence        StreamData[int]     `json:"cadence"`
    GradeSmooth    StreamData[float64] `json:"grade_smooth"`
    Distance       StreamData[float64] `json:"distance"`
}

type StreamData[T any] struct {
    Data         []T    `json:"data"`
    SeriesType   string `json:"series_type"`
    OriginalSize int    `json:"original_size"`
    Resolution   string `json:"resolution"`
}
```

### Rate Limiter

```go
// internal/strava/ratelimit.go

package strava

import (
    "context"
    "net/http"
    "strconv"
    "sync"
    "time"
)

// Strava rate limits:
// - 100 requests per 15 minutes
// - 1000 requests per day

type RateLimiter struct {
    mu sync.Mutex
    
    // 15-minute window
    shortLimit     int
    shortUsage     int
    shortResetsAt  time.Time
    
    // Daily window
    dailyLimit     int
    dailyUsage     int
    dailyResetsAt  time.Time
    
    // Minimum interval between requests
    minInterval    time.Duration
    lastRequest    time.Time
}

func NewRateLimiter() *RateLimiter {
    return &RateLimiter{
        shortLimit:  100,
        dailyLimit:  1000,
        minInterval: 150 * time.Millisecond, // ~6.6 req/s max
    }
}

func (r *RateLimiter) Wait(ctx context.Context) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    now := time.Now()
    
    // Reset windows if expired
    if now.After(r.shortResetsAt) {
        r.shortUsage = 0
        r.shortResetsAt = now.Add(15 * time.Minute)
    }
    if now.After(r.dailyResetsAt) {
        r.dailyUsage = 0
        r.dailyResetsAt = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
    }
    
    // Check limits
    if r.shortUsage >= r.shortLimit {
        waitTime := r.shortResetsAt.Sub(now)
        select {
        case <-time.After(waitTime):
            r.shortUsage = 0
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    
    if r.dailyUsage >= r.dailyLimit {
        waitTime := r.dailyResetsAt.Sub(now)
        select {
        case <-time.After(waitTime):
            r.dailyUsage = 0
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    
    // Enforce minimum interval
    elapsed := now.Sub(r.lastRequest)
    if elapsed < r.minInterval {
        select {
        case <-time.After(r.minInterval - elapsed):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    
    r.shortUsage++
    r.dailyUsage++
    r.lastRequest = time.Now()
    
    return nil
}

func (r *RateLimiter) UpdateFromHeaders(h http.Header) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Strava returns: X-RateLimit-Limit, X-RateLimit-Usage
    // Format: "100,1000" (15-min, daily)
    if usage := h.Get("X-RateLimit-Usage"); usage != "" {
        // Parse and update actual usage from Strava
        // This helps stay accurate even if counts drift
    }
}

// Status returns current rate limit status for display
func (r *RateLimiter) Status() (shortRemaining, dailyRemaining int) {
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.shortLimit - r.shortUsage, r.dailyLimit - r.dailyUsage
}
```

### Sync Strategy

```go
// internal/service/sync.go

package service

import (
    "context"
    "fmt"
    "time"
)

type SyncService struct {
    strava *strava.Client
    store  *store.DB
}

type SyncProgress struct {
    Phase           string  // "activities", "streams", "metrics"
    Total           int
    Completed       int
    CurrentActivity string
    Error           error
}

// SyncAll performs a full sync: activities -> streams -> metrics
func (s *SyncService) SyncAll(ctx context.Context, progress chan<- SyncProgress) error {
    defer close(progress)
    
    // Phase 1: Sync activity summaries
    progress <- SyncProgress{Phase: "activities", Total: 0, Completed: 0}
    
    lastSync, _ := s.store.GetSyncState("last_activity_sync")
    after := time.Time{}
    if lastSync != "" {
        after, _ = time.Parse(time.RFC3339, lastSync)
    }
    
    activitiesAdded := 0
    page := 1
    for {
        activities, err := s.strava.GetActivities(ctx, after, page, 100)
        if err != nil {
            return fmt.Errorf("fetching activities: %w", err)
        }
        
        if len(activities) == 0 {
            break
        }
        
        for _, a := range activities {
            // Only store runs with HR data
            if a.Type == "Run" && a.HasHeartrate {
                if err := s.store.UpsertActivity(a); err != nil {
                    return fmt.Errorf("storing activity %d: %w", a.ID, err)
                }
                activitiesAdded++
            }
        }
        
        progress <- SyncProgress{Phase: "activities", Total: activitiesAdded, Completed: activitiesAdded}
        page++
    }
    
    s.store.SetSyncState("last_activity_sync", time.Now().Format(time.RFC3339))
    
    // Phase 2: Fetch streams for activities that need them
    needStreams, err := s.store.GetActivitiesNeedingStreams(100) // Batch of 100
    if err != nil {
        return err
    }
    
    progress <- SyncProgress{Phase: "streams", Total: len(needStreams), Completed: 0}
    
    for i, activity := range needStreams {
        streams, err := s.strava.GetActivityStreams(ctx, activity.ID)
        if err != nil {
            // Log but continue - some activities may not have streams
            progress <- SyncProgress{
                Phase:     "streams",
                Total:     len(needStreams),
                Completed: i + 1,
                Error:     fmt.Errorf("activity %d: %w", activity.ID, err),
            }
            continue
        }
        
        if err := s.store.SaveStreams(activity.ID, streams); err != nil {
            return fmt.Errorf("saving streams for %d: %w", activity.ID, err)
        }
        
        s.store.MarkStreamsSync(activity.ID)
        
        progress <- SyncProgress{
            Phase:           "streams",
            Total:           len(needStreams),
            Completed:       i + 1,
            CurrentActivity: activity.Name,
        }
    }
    
    // Phase 3: Compute metrics
    needMetrics, err := s.store.GetActivitiesNeedingMetrics()
    if err != nil {
        return err
    }
    
    progress <- SyncProgress{Phase: "metrics", Total: len(needMetrics), Completed: 0}
    
    for i, activity := range needMetrics {
        streams, err := s.store.GetStreams(activity.ID)
        if err != nil {
            continue
        }
        
        metrics := analysis.ComputeActivityMetrics(activity, streams)
        if err := s.store.SaveActivityMetrics(metrics); err != nil {
            return err
        }
        
        progress <- SyncProgress{Phase: "metrics", Total: len(needMetrics), Completed: i + 1}
    }
    
    // Update fitness trends
    if err := s.updateFitnessTrends(ctx); err != nil {
        return err
    }
    
    return nil
}
```

---

## Analysis Algorithms

### Efficiency Factor (EF)

Efficiency Factor measures how fast you can run for a given heart rate. Higher values indicate better aerobic fitness.

```go
// internal/analysis/efficiency.go

package analysis

import "math"

// EfficiencyFactor calculates pace:HR efficiency
// Returns: (speed in m/s) / (average HR) * 100,000
// Higher is better - you're running faster for the same HR
func EfficiencyFactor(streams []StreamPoint) float64 {
    var totalVelocity, totalHR float64
    var count int

    for _, p := range streams {
        if p.VelocitySmooth != nil && p.Heartrate != nil {
            if *p.VelocitySmooth > 0.5 && *p.Heartrate > 80 { // Filter noise
                totalVelocity += *p.VelocitySmooth
                totalHR += float64(*p.Heartrate)
                count++
            }
        }
    }

    if count == 0 {
        return 0
    }

    avgVelocity := totalVelocity / float64(count)  // m/s
    avgHR := totalHR / float64(count)

    // Scale for readability: typical values 1.0 - 2.0
    return (avgVelocity / avgHR) * 100000
}

// NormalizedEfficiencyFactor adjusts for elevation gain
// Uses Strava's GAP-like normalization
func NormalizedEfficiencyFactor(streams []StreamPoint) float64 {
    var totalNGP, totalHR float64
    var count int

    for _, p := range streams {
        if p.VelocitySmooth == nil || p.Heartrate == nil || p.GradeSmooth == nil {
            continue
        }
        if *p.VelocitySmooth < 0.5 || *p.Heartrate < 80 {
            continue
        }

        // Normalize pace for grade
        // Approximate: +10% grade adds ~30s/km equivalent effort
        grade := *p.GradeSmooth / 100.0 // Convert to decimal
        gradeFactor := 1.0 + (grade * 3.0)
        if gradeFactor < 0.5 {
            gradeFactor = 0.5 // Cap adjustment for steep descents
        }

        ngp := *p.VelocitySmooth / gradeFactor
        totalNGP += ngp
        totalHR += float64(*p.Heartrate)
        count++
    }

    if count == 0 {
        return 0
    }

    avgNGP := totalNGP / float64(count)
    avgHR := totalHR / float64(count)

    return (avgNGP / avgHR) * 100000
}
```

### Aerobic Decoupling

Aerobic decoupling measures how much your efficiency drops in the second half of a run compared to the first half. Well-trained aerobic athletes show < 5% decoupling on long steady runs.

```go
// internal/analysis/decoupling.go

package analysis

import "math"

// AerobicDecoupling calculates the pace:HR drift between first and second half
// Returns percentage - positive means second half was less efficient
// < 5% on long runs indicates good aerobic base
func AerobicDecoupling(streams []StreamPoint) float64 {
    if len(streams) < 60 { // Need at least 60 seconds
        return 0
    }

    // Split into halves
    mid := len(streams) / 2
    firstHalf := streams[:mid]
    secondHalf := streams[mid:]

    firstEF := calculateHalfEF(firstHalf)
    secondEF := calculateHalfEF(secondHalf)

    if firstEF == 0 || secondEF == 0 {
        return 0
    }

    // Positive decoupling = second half less efficient (worse)
    // Formula: ((first / second) - 1) * 100
    decoupling := ((firstEF / secondEF) - 1) * 100

    return decoupling
}

func calculateHalfEF(streams []StreamPoint) float64 {
    var totalVelocity, totalHR float64
    var count int

    for _, p := range streams {
        if p.VelocitySmooth != nil && p.Heartrate != nil {
            if *p.VelocitySmooth > 0.5 && *p.Heartrate > 80 {
                totalVelocity += *p.VelocitySmooth
                totalHR += float64(*p.Heartrate)
                count++
            }
        }
    }

    if count == 0 {
        return 0
    }

    return (totalVelocity / float64(count)) / (totalHR / float64(count))
}

// CardiacDrift measures HR increase during steady-state running
// Filters to segments where pace is relatively constant
func CardiacDrift(streams []StreamPoint, activity Activity) float64 {
    // Find steady-state segments (pace within 10% of average)
    avgPace := activity.Distance / float64(activity.MovingTime)
    
    var steadyStreams []StreamPoint
    for _, p := range streams {
        if p.VelocitySmooth == nil {
            continue
        }
        paceRatio := *p.VelocitySmooth / avgPace
        if paceRatio > 0.9 && paceRatio < 1.1 {
            steadyStreams = append(steadyStreams, p)
        }
    }

    if len(steadyStreams) < 60 {
        return 0
    }

    // Compare first quarter vs last quarter HR
    q1 := len(steadyStreams) / 4
    firstQuarter := steadyStreams[:q1]
    lastQuarter := steadyStreams[len(steadyStreams)-q1:]

    firstHR := averageHR(firstQuarter)
    lastHR := averageHR(lastQuarter)

    if firstHR == 0 {
        return 0
    }

    // Return absolute HR drift
    return lastHR - firstHR
}

func averageHR(streams []StreamPoint) float64 {
    var total float64
    var count int
    for _, p := range streams {
        if p.Heartrate != nil && *p.Heartrate > 0 {
            total += float64(*p.Heartrate)
            count++
        }
    }
    if count == 0 {
        return 0
    }
    return total / float64(count)
}
```

### Training Load (TRIMP / CTL / ATL / TSB)

Training Impulse (TRIMP) quantifies workout stress. Chronic Training Load (CTL) and Acute Training Load (ATL) are rolling averages that model fitness and fatigue.

```go
// internal/analysis/training_load.go

package analysis

import (
    "math"
    "sort"
    "time"
)

// HRZones represents athlete's heart rate zones
type HRZones struct {
    RestingHR float64
    MaxHR     float64
}

// DefaultZones returns sensible defaults if not configured
func DefaultZones() HRZones {
    return HRZones{
        RestingHR: 50,
        MaxHR:     185,
    }
}

// TRIMP calculates Training Impulse (Banister model)
// TRIMP = duration (min) * ΔHR ratio * e^(b * ΔHR ratio)
// where b = 1.92 for men, 1.67 for women
func TRIMP(activity Activity, streams []StreamPoint, zones HRZones) float64 {
    duration := float64(activity.MovingTime) / 60.0 // Convert to minutes
    
    avgHR := averageHR(streams)
    if avgHR == 0 && activity.AverageHeartrate != nil {
        avgHR = *activity.AverageHeartrate
    }
    if avgHR == 0 {
        return 0
    }

    // Heart rate reserve ratio
    hrReserve := zones.MaxHR - zones.RestingHR
    hrRatio := (avgHR - zones.RestingHR) / hrReserve
    if hrRatio < 0 {
        hrRatio = 0
    }
    if hrRatio > 1 {
        hrRatio = 1
    }

    // Gender coefficient (using male default)
    b := 1.92

    return duration * hrRatio * math.Exp(b*hrRatio)
}

// HRSS calculates Heart Rate Stress Score
// Normalized to ~100 for a 1-hour threshold effort
func HRSS(activity Activity, streams []StreamPoint, zones HRZones) float64 {
    trimp := TRIMP(activity, streams, zones)
    
    // Threshold TRIMP for 1 hour at lactate threshold (~88% max HR)
    // Approximately 100 TRIMP for 1 hour at threshold
    thresholdTRIMP := 100.0
    
    return (trimp / thresholdTRIMP) * 100
}

// FitnessLoad calculates CTL, ATL, and TSB from daily TRIMP values
type DailyLoad struct {
    Date  time.Time
    TRIMP float64
}

type FitnessMetrics struct {
    Date time.Time
    CTL  float64 // Chronic Training Load (42-day EMA)
    ATL  float64 // Acute Training Load (7-day EMA)
    TSB  float64 // Training Stress Balance (CTL - ATL)
}

// CalculateFitnessTrend computes CTL/ATL/TSB from daily loads
func CalculateFitnessTrend(dailyLoads []DailyLoad) []FitnessMetrics {
    if len(dailyLoads) == 0 {
        return nil
    }

    // Sort by date
    sort.Slice(dailyLoads, func(i, j int) bool {
        return dailyLoads[i].Date.Before(dailyLoads[j].Date)
    })

    // EMA decay constants
    ctlDecay := 2.0 / (42.0 + 1.0) // 42-day time constant
    atlDecay := 2.0 / (7.0 + 1.0)  // 7-day time constant

    var metrics []FitnessMetrics
    var ctl, atl float64

    // Fill in missing days with zero load
    startDate := dailyLoads[0].Date
    endDate := dailyLoads[len(dailyLoads)-1].Date
    loadMap := make(map[string]float64)
    for _, dl := range dailyLoads {
        key := dl.Date.Format("2006-01-02")
        loadMap[key] += dl.TRIMP // Sum multiple activities on same day
    }

    for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
        key := d.Format("2006-01-02")
        trimp := loadMap[key] // 0 if no activity

        // Exponential moving average
        ctl = ctl + ctlDecay*(trimp-ctl)
        atl = atl + atlDecay*(trimp-atl)
        tsb := ctl - atl

        metrics = append(metrics, FitnessMetrics{
            Date: d,
            CTL:  ctl,
            ATL:  atl,
            TSB:  tsb,
        })
    }

    return metrics
}
```

### Trend Detection

```go
// internal/analysis/trends.go

package analysis

import (
    "math"
    "time"
)

// TrendDirection indicates whether a metric is improving or declining
type TrendDirection int

const (
    TrendFlat TrendDirection = iota
    TrendUp
    TrendDown
)

// TrendAnalysis contains analysis of a metric over time
type TrendAnalysis struct {
    Direction      TrendDirection
    ChangePercent  float64
    Slope          float64 // Units per day
    RSquared       float64 // Goodness of fit (0-1)
}

// AnalyzeTrend performs linear regression on time series data
func AnalyzeTrend(values []float64, dates []time.Time) TrendAnalysis {
    n := len(values)
    if n < 3 {
        return TrendAnalysis{Direction: TrendFlat}
    }

    // Convert dates to days from start
    startDate := dates[0]
    x := make([]float64, n)
    for i, d := range dates {
        x[i] = d.Sub(startDate).Hours() / 24.0
    }

    // Linear regression: y = mx + b
    var sumX, sumY, sumXY, sumX2, sumY2 float64
    for i := 0; i < n; i++ {
        sumX += x[i]
        sumY += values[i]
        sumXY += x[i] * values[i]
        sumX2 += x[i] * x[i]
        sumY2 += values[i] * values[i]
    }

    nf := float64(n)
    slope := (nf*sumXY - sumX*sumY) / (nf*sumX2 - sumX*sumX)
    intercept := (sumY - slope*sumX) / nf

    // R-squared
    yMean := sumY / nf
    var ssTotal, ssResidual float64
    for i := 0; i < n; i++ {
        predicted := slope*x[i] + intercept
        ssTotal += (values[i] - yMean) * (values[i] - yMean)
        ssResidual += (values[i] - predicted) * (values[i] - predicted)
    }
    rSquared := 1 - (ssResidual / ssTotal)
    if math.IsNaN(rSquared) {
        rSquared = 0
    }

    // Determine direction
    startValue := intercept
    endValue := slope*x[n-1] + intercept
    changePercent := 0.0
    if startValue != 0 {
        changePercent = ((endValue - startValue) / math.Abs(startValue)) * 100
    }

    direction := TrendFlat
    if rSquared > 0.3 { // Only consider trend if fit is reasonable
        if changePercent > 5 {
            direction = TrendUp
        } else if changePercent < -5 {
            direction = TrendDown
        }
    }

    return TrendAnalysis{
        Direction:     direction,
        ChangePercent: changePercent,
        Slope:         slope,
        RSquared:      rSquared,
    }
}

// MovingAverage calculates simple moving average
func MovingAverage(values []float64, window int) []float64 {
    if len(values) < window {
        return values
    }

    result := make([]float64, len(values)-window+1)
    var sum float64

    // Initialize first window
    for i := 0; i < window; i++ {
        sum += values[i]
    }
    result[0] = sum / float64(window)

    // Slide window
    for i := window; i < len(values); i++ {
        sum = sum - values[i-window] + values[i]
        result[i-window+1] = sum / float64(window)
    }

    return result
}

// RollingStats calculates rolling statistics for display
type RollingStats struct {
    Mean   float64
    StdDev float64
    Min    float64
    Max    float64
    Trend  TrendAnalysis
}

func CalculateRollingStats(values []float64, dates []time.Time) RollingStats {
    if len(values) == 0 {
        return RollingStats{}
    }

    var sum, min, max float64
    min = values[0]
    max = values[0]

    for _, v := range values {
        sum += v
        if v < min {
            min = v
        }
        if v > max {
            max = v
        }
    }

    mean := sum / float64(len(values))

    var variance float64
    for _, v := range values {
        diff := v - mean
        variance += diff * diff
    }
    stdDev := math.Sqrt(variance / float64(len(values)))

    return RollingStats{
        Mean:   mean,
        StdDev: stdDev,
        Min:    min,
        Max:    max,
        Trend:  AnalyzeTrend(values, dates),
    }
}
```

### Putting It Together

```go
// internal/analysis/compute.go

package analysis

import "strava-fitness/internal/store"

// ComputeActivityMetrics calculates all metrics for a single activity
func ComputeActivityMetrics(activity store.Activity, streams []store.StreamPoint) store.ActivityMetrics {
    zones := DefaultZones()
    
    ef := EfficiencyFactor(streams)
    nef := NormalizedEfficiencyFactor(streams)
    decoupling := AerobicDecoupling(streams)
    drift := CardiacDrift(streams, activity)
    trimp := TRIMP(activity, streams, zones)
    hrss := HRSS(activity, streams, zones)
    
    // Data quality: what percentage of stream points have HR data?
    validPoints := 0
    for _, p := range streams {
        if p.Heartrate != nil && *p.Heartrate > 0 {
            validPoints++
        }
    }
    quality := float64(validPoints) / float64(len(streams))
    
    return store.ActivityMetrics{
        ActivityID:        activity.ID,
        EfficiencyFactor:  &ef,
        AerobicDecoupling: &decoupling,
        CardiacDrift:      &drift,
        TRIMP:             &trimp,
        HRSS:              &hrss,
        DataQualityScore:  &quality,
    }
}
```

---

## TUI Design

### Screen Layouts

#### Dashboard (Home)

```
┌─ Aerobic Fitness Analyzer ─────────────────────────────────────────────┐
│  [1] Dashboard  [2] Trends  [3] Activities  [4] Sync  [?] Help  [q]uit │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌─ Current Fitness ────────────────┐  ┌─ This Week ────────────────┐  │
│  │                                  │  │                            │  │
│  │  Efficiency Factor    1.42  ↑3%  │  │  Runs         4            │  │
│  │  Aerobic Decoupling   4.1%       │  │  Distance     42.5 km      │  │
│  │  Fitness (CTL)        48         │  │  Time         4h 12m       │  │
│  │  Fatigue (ATL)        62         │  │  Avg EF       1.38         │  │
│  │  Form (TSB)           -14        │  │                            │  │
│  │                                  │  │                            │  │
│  └──────────────────────────────────┘  └────────────────────────────┘  │
│                                                                        │
│  ┌─ Efficiency Factor - Last 90 Days ────────────────────────────────┐ │
│  │ 1.50│                                              ·              │ │
│  │     │                                    ·    ·  ·   ·  ·         │ │
│  │ 1.40│                          ·    · ·                           │ │
│  │     │                    ·  ·                                     │ │
│  │ 1.30│         ·    ·  ·                                           │ │
│  │     │    · ·                                                      │ │
│  │ 1.20│ ·                                                           │ │
│  │     └─────────────────────────────────────────────────────────────│ │
│  │       Nov              Dec              Jan                       │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  ┌─ Recent Activities ───────────────────────────────────────────────┐ │
│  │  Jan 24  Easy Run          8.2 km   45:12   EF 1.45   Dec 2.1%    │ │
│  │  Jan 22  Tempo Intervals   6.5 km   32:45   EF 1.52   Dec 5.8%    │ │
│  │  Jan 20  Long Run         16.1 km  1:28:33  EF 1.38   Dec 3.2%    │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  Last synced: 2 hours ago    Press 's' to sync now                     │
└────────────────────────────────────────────────────────────────────────┘
```

#### Trends View

```
┌─ Aerobic Fitness Analyzer ─ Trends ────────────────────────────────────┐
│  [1] Dashboard  [2] Trends  [3] Activities  [4] Sync  [?] Help  [q]uit │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  Time Range: [30d] [90d] [180d] [1y] [All]         Metric: EF ▼        │
│                                                                        │
│  ┌─ Efficiency Factor Trend ─────────────────────────────────────────┐ │
│  │ 1.55│                                                     ·       │ │
│  │     │                                               ·   ·   ·     │ │
│  │ 1.45│                                    ·    ·   ·               │ │
│  │     │                              ·  ·    ·                      │ │
│  │ 1.35│                    ·    · ·                                 │ │
│  │     │              ·  ·                                           │ │
│  │ 1.25│    ·    · ·                                                 │ │
│  │     │ ·                                                           │ │
│  │ 1.15└─────────────────────────────────────────────────────────────│ │
│  │       Oct         Nov         Dec         Jan                     │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  ┌─ Statistics ──────────────────┐  ┌─ Training Load ──────────────┐   │
│  │                               │  │                              │   │
│  │  90-Day Trend    ↑ +8.3%      │  │  CTL (Fitness)    48  ██████ │   │
│  │  Current Avg     1.42         │  │  ATL (Fatigue)    62  ████████│  │
│  │  90-Day Avg      1.34         │  │  TSB (Form)      -14  ██     │   │
│  │  Best            1.58 (Jan 18)│  │                              │   │
│  │  Worst           1.18 (Oct 5) │  │  Form: Tired but fit         │   │
│  │                               │  │                              │   │
│  └───────────────────────────────┘  └──────────────────────────────┘   │
│                                                                        │
│  ← → Change range   ↑ ↓ Change metric   Enter: View details            │
└────────────────────────────────────────────────────────────────────────┘
```

#### Activities List

```
┌─ Aerobic Fitness Analyzer ─ Activities ────────────────────────────────┐
│  [1] Dashboard  [2] Trends  [3] Activities  [4] Sync  [?] Help  [q]uit │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  Filter: [All] [Long Runs] [Tempo] [Easy]    Sort: Date ▼              │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │    Date     Name              Dist    Time     EF    Decoup  HR  │  │
│  ├──────────────────────────────────────────────────────────────────┤  │
│  │ ▸ Jan 24   Easy Run           8.2k   45:12   1.45    2.1%   142  │  │
│  │   Jan 22   Tempo Intervals    6.5k   32:45   1.52    5.8%   168  │  │
│  │   Jan 20   Long Run          16.1k  1:28:33  1.38    3.2%   148  │  │
│  │   Jan 18   Easy Run           7.8k   42:55   1.58    1.8%   138  │  │
│  │   Jan 16   Hill Repeats       5.2k   35:20   1.31    4.5%   172  │  │
│  │   Jan 14   Recovery Run       5.0k   32:10   1.48    1.2%   132  │  │
│  │   Jan 12   Long Run          18.5k  1:42:18  1.35    4.8%   152  │  │
│  │   Jan 10   Easy Run           8.0k   44:30   1.44    2.3%   140  │  │
│  │   Jan 08   Tempo Run          8.2k   38:45   1.49    3.1%   165  │  │
│  │   Jan 06   Easy Run           6.5k   36:20   1.42    1.9%   138  │  │
│  │                                                                  │  │
│  │                         Page 1 of 12                             │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ↑ ↓ Navigate   Enter: View details   f: Filter   s: Sort   /: Search │
└────────────────────────────────────────────────────────────────────────┘
```

#### Activity Detail

```
┌─ Aerobic Fitness Analyzer ─ Activity Detail ───────────────────────────┐
│  ← Back                                                                │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  Long Run                                         January 20, 2025     │
│  ─────────────────────────────────────────────────────────────────     │
│                                                                        │
│  ┌─ Summary ────────────────────┐  ┌─ Fitness Metrics ─────────────┐   │
│  │                              │  │                               │   │
│  │  Distance      16.1 km       │  │  Efficiency Factor   1.38     │   │
│  │  Time          1:28:33       │  │  Aerobic Decoupling  3.2%  ✓  │   │
│  │  Pace          5:30 /km      │  │  Cardiac Drift       +8 bpm   │   │
│  │  Elevation     +182m         │  │  TRIMP               142      │   │
│  │  Avg HR        148 bpm       │  │  Data Quality        98%      │   │
│  │  Max HR        168 bpm       │  │                               │   │
│  │                              │  │  Steady state: 78% of run     │   │
│  └──────────────────────────────┘  └───────────────────────────────┘   │
│                                                                        │
│  ┌─ Heart Rate vs Pace ──────────────────────────────────────────────┐ │
│  │ HR │····································                          │ │
│  │ 170│                         ·  ·    ···                          │ │
│  │ 160│                    ·····    ····                             │ │
│  │ 150│          ···········                                         │ │
│  │ 140│    ······                                        Pace ────   │ │
│  │ 130│ ···                                              HR   ····   │ │
│  │    └──────────────────────────────────────────────────────────────│ │
│  │      0:00    0:20    0:40    1:00    1:20                         │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  Analysis: Good aerobic run. Decoupling under 5% indicates strong      │
│  aerobic base. HR drifted +8 bpm which is normal for this duration.    │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

#### Sync Progress

```
┌─ Aerobic Fitness Analyzer ─ Sync ──────────────────────────────────────┐
│  [1] Dashboard  [2] Trends  [3] Activities  [4] Sync  [?] Help  [q]uit │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│                           Syncing with Strava                          │
│                                                                        │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                                                                   │ │
│  │   Phase 1: Fetching Activities                          ✓ Done   │ │
│  │   ████████████████████████████████████████████████████  142 new  │ │
│  │                                                                   │ │
│  │   Phase 2: Fetching Stream Data                        In Progress│ │
│  │   ██████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░   52/142   │ │
│  │   Current: "Morning Easy Run" (Jan 15)                            │ │
│  │                                                                   │ │
│  │   Phase 3: Computing Metrics                             Pending  │ │
│  │   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░             │ │
│  │                                                                   │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  ┌─ Rate Limits ─────────────────────────────────────────────────────┐ │
│  │  15-minute:  48/100 remaining    Daily:  812/1000 remaining       │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  Estimated time remaining: ~8 minutes                                  │
│                                                                        │
│  Press 'c' to cancel sync                                              │
└────────────────────────────────────────────────────────────────────────┘
```

### Bubble Tea Architecture

```go
// internal/tui/app.go

package tui

import (
    "strava-fitness/internal/service"
    "strava-fitness/internal/store"

    tea "github.com/charmbracelet/bubbletea"
)

// Screen identifiers
type Screen int

const (
    ScreenDashboard Screen = iota
    ScreenTrends
    ScreenActivities
    ScreenActivityDetail
    ScreenSync
    ScreenHelp
)

// App is the root model
type App struct {
    screen       Screen
    prevScreen   Screen
    
    // Child models
    dashboard    DashboardModel
    trends       TrendsModel
    activities   ActivitiesModel
    detail       ActivityDetailModel
    sync         SyncModel
    help         HelpModel
    
    // Shared dependencies
    store        *store.DB
    syncService  *service.SyncService
    queryService *service.QueryService
    
    // Window size
    width  int
    height int
}

func NewApp(db *store.DB, syncSvc *service.SyncService, querySvc *service.QueryService) *App {
    return &App{
        screen:       ScreenDashboard,
        store:        db,
        syncService:  syncSvc,
        queryService: querySvc,
        dashboard:    NewDashboardModel(querySvc),
        trends:       NewTrendsModel(querySvc),
        activities:   NewActivitiesModel(querySvc),
        sync:         NewSyncModel(syncSvc),
        help:         NewHelpModel(),
    }
}

func (a *App) Init() tea.Cmd {
    return a.dashboard.Init()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Global keybindings
        switch msg.String() {
        case "q", "ctrl+c":
            return a, tea.Quit
        case "1":
            a.screen = ScreenDashboard
            return a, a.dashboard.Init()
        case "2":
            a.screen = ScreenTrends
            return a, a.trends.Init()
        case "3":
            a.screen = ScreenActivities
            return a, a.activities.Init()
        case "4":
            a.screen = ScreenSync
            return a, a.sync.Init()
        case "?":
            a.prevScreen = a.screen
            a.screen = ScreenHelp
            return a, nil
        case "esc":
            if a.screen == ScreenHelp {
                a.screen = a.prevScreen
            } else if a.screen == ScreenActivityDetail {
                a.screen = ScreenActivities
            }
            return a, nil
        }

    case tea.WindowSizeMsg:
        a.width = msg.Width
        a.height = msg.Height
        
    case ViewActivityMsg:
        a.detail = NewActivityDetailModel(a.queryService, msg.ActivityID)
        a.screen = ScreenActivityDetail
        return a, a.detail.Init()
    }

    // Delegate to current screen
    var cmd tea.Cmd
    switch a.screen {
    case ScreenDashboard:
        a.dashboard, cmd = a.dashboard.Update(msg)
    case ScreenTrends:
        a.trends, cmd = a.trends.Update(msg)
    case ScreenActivities:
        a.activities, cmd = a.activities.Update(msg)
    case ScreenActivityDetail:
        a.detail, cmd = a.detail.Update(msg)
    case ScreenSync:
        a.sync, cmd = a.sync.Update(msg)
    case ScreenHelp:
        a.help, cmd = a.help.Update(msg)
    }
    
    return a, cmd
}

func (a *App) View() string {
    header := a.renderHeader()
    
    var content string
    switch a.screen {
    case ScreenDashboard:
        content = a.dashboard.View()
    case ScreenTrends:
        content = a.trends.View()
    case ScreenActivities:
        content = a.activities.View()
    case ScreenActivityDetail:
        content = a.detail.View()
    case ScreenSync:
        content = a.sync.View()
    case ScreenHelp:
        content = a.help.View()
    }
    
    return header + "\n" + content
}

func (a *App) renderHeader() string {
    // Use Lip Gloss to style the header/navigation
    // Highlight current screen
    return styles.Header.Render("...") 
}

// ViewActivityMsg is sent when user wants to view activity details
type ViewActivityMsg struct {
    ActivityID int64
}
```

### Styles

```go
// internal/tui/styles.go

package tui

import "github.com/charmbracelet/lipgloss"

var (
    // Colors
    primaryColor   = lipgloss.Color("#7C3AED") // Purple
    secondaryColor = lipgloss.Color("#10B981") // Green
    warningColor   = lipgloss.Color("#F59E0B") // Amber
    errorColor     = lipgloss.Color("#EF4444") // Red
    mutedColor     = lipgloss.Color("#6B7280") // Gray
    
    // Base styles
    styles = struct {
        Header      lipgloss.Style
        NavItem     lipgloss.Style
        NavActive   lipgloss.Style
        Card        lipgloss.Style
        CardTitle   lipgloss.Style
        Metric      lipgloss.Style
        MetricLabel lipgloss.Style
        MetricValue lipgloss.Style
        TrendUp     lipgloss.Style
        TrendDown   lipgloss.Style
        Table       lipgloss.Style
        TableHeader lipgloss.Style
        TableRow    lipgloss.Style
        TableRowAlt lipgloss.Style
        Selected    lipgloss.Style
        Muted       lipgloss.Style
        Error       lipgloss.Style
    }{
        Header: lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("#FFFFFF")).
            Background(primaryColor).
            Padding(0, 1),
            
        NavItem: lipgloss.NewStyle().
            Foreground(mutedColor).
            Padding(0, 1),
            
        NavActive: lipgloss.NewStyle().
            Bold(true).
            Foreground(primaryColor).
            Padding(0, 1),
            
        Card: lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(mutedColor).
            Padding(1, 2),
            
        CardTitle: lipgloss.NewStyle().
            Bold(true).
            Foreground(primaryColor).
            MarginBottom(1),
            
        Metric: lipgloss.NewStyle().
            MarginBottom(0),
            
        MetricLabel: lipgloss.NewStyle().
            Foreground(mutedColor).
            Width(20),
            
        MetricValue: lipgloss.NewStyle().
            Bold(true),
            
        TrendUp: lipgloss.NewStyle().
            Foreground(secondaryColor),
            
        TrendDown: lipgloss.NewStyle().
            Foreground(errorColor),
            
        Table: lipgloss.NewStyle().
            Border(lipgloss.NormalBorder()).
            BorderForeground(mutedColor),
            
        TableHeader: lipgloss.NewStyle().
            Bold(true).
            Foreground(primaryColor).
            BorderBottom(true).
            BorderForeground(mutedColor),
            
        TableRow: lipgloss.NewStyle().
            Padding(0, 1),
            
        TableRowAlt: lipgloss.NewStyle().
            Padding(0, 1).
            Background(lipgloss.Color("#1F2937")),
            
        Selected: lipgloss.NewStyle().
            Bold(true).
            Background(primaryColor).
            Foreground(lipgloss.Color("#FFFFFF")),
            
        Muted: lipgloss.NewStyle().
            Foreground(mutedColor),
            
        Error: lipgloss.NewStyle().
            Foreground(errorColor),
    }
)

// Helper functions for common patterns
func renderMetric(label string, value string, trend string) string {
    trendStyle := styles.Muted
    if len(trend) > 0 {
        if trend[0] == '+' || trend[0] == '↑' {
            trendStyle = styles.TrendUp
        } else if trend[0] == '-' || trend[0] == '↓' {
            trendStyle = styles.TrendDown
        }
    }
    
    return lipgloss.JoinHorizontal(
        lipgloss.Left,
        styles.MetricLabel.Render(label),
        styles.MetricValue.Render(value),
        trendStyle.Render(" "+trend),
    )
}
```

---

## Project Structure

```
strava-fitness/
├── go.mod
├── go.sum
├── main.go                          # Entry point
├── Makefile                         # Build commands
├── README.md
├── config.example.json              # Example configuration
│
├── internal/
│   ├── auth/
│   │   ├── oauth.go                 # OAuth flow implementation
│   │   ├── refresh.go               # Token refresh logic
│   │   └── server.go                # Localhost callback server
│   │
│   ├── strava/
│   │   ├── client.go                # API client
│   │   ├── models.go                # API response structs
│   │   └── ratelimit.go             # Rate limiter
│   │
│   ├── store/
│   │   ├── db.go                    # Database connection
│   │   ├── models.go                # Domain models
│   │   ├── migrations.go            # Schema migrations
│   │   ├── auth.go                  # Auth token CRUD
│   │   ├── activities.go            # Activity CRUD
│   │   ├── streams.go               # Stream CRUD
│   │   ├── metrics.go               # Metrics CRUD
│   │   └── sync_state.go            # Sync state management
│   │
│   ├── analysis/
│   │   ├── efficiency.go            # Efficiency Factor
│   │   ├── decoupling.go            # Aerobic Decoupling
│   │   ├── training_load.go         # TRIMP, CTL, ATL, TSB
│   │   ├── trends.go                # Trend analysis
│   │   └── compute.go               # Orchestration
│   │
│   ├── service/
│   │   ├── sync.go                  # Sync orchestration
│   │   └── query.go                 # Read queries for TUI
│   │
│   └── tui/
│       ├── app.go                   # Root Bubble Tea model
│       ├── styles.go                # Lip Gloss styles
│       ├── keys.go                  # Keybinding definitions
│       │
│       ├── screens/
│       │   ├── dashboard.go         # Dashboard screen
│       │   ├── trends.go            # Trends screen
│       │   ├── activities.go        # Activity list screen
│       │   ├── detail.go            # Activity detail screen
│       │   ├── sync.go              # Sync progress screen
│       │   └── help.go              # Help screen
│       │
│       └── components/
│           ├── chart.go             # ASCII chart component
│           ├── table.go             # Data table component
│           ├── progress.go          # Progress bar component
│           └── metric_card.go       # Metric display card
│
└── scripts/
    └── install.sh                   # Installation helper
```

---

## Implementation Guide

### Phase 1: Foundation (Day 1)

**Goal**: Working database and basic models

1. Initialize Go module
   ```bash
   go mod init strava-fitness
   ```

2. Create `internal/store/db.go`
   - Database connection
   - Path resolution (`~/.strava-fitness/data.db`)
   - Connection pooling

3. Create `internal/store/migrations.go`
   - Schema from this document
   - Run on startup

4. Create `internal/store/models.go`
   - All domain structs

5. Write basic CRUD operations
   - `internal/store/auth.go`
   - `internal/store/activities.go`
   - `internal/store/sync_state.go`

**Verification**: Unit tests for CRUD operations pass

### Phase 2: Authentication (Day 1-2)

**Goal**: Complete OAuth flow, tokens stored and refreshable

1. Create `internal/auth/oauth.go`
   - OAuth config
   - Auth URL generation

2. Create `internal/auth/server.go`
   - Localhost callback server
   - Code exchange

3. Create `internal/auth/refresh.go`
   - Token refresh logic
   - Persistence callback

4. Create config file handling
   - Read client ID/secret from `~/.strava-fitness/config.json`

**Verification**: Can authenticate with Strava, tokens persist across restarts

### Phase 3: API Client (Day 2)

**Goal**: Fetch activities and streams from Strava

1. Create `internal/strava/models.go`
   - API response structs

2. Create `internal/strava/ratelimit.go`
   - Token bucket implementation
   - Header parsing

3. Create `internal/strava/client.go`
   - `GetActivities()` with pagination
   - `GetActivityStreams()`

**Verification**: Can fetch and print activities from API

### Phase 4: Sync Service (Day 2-3)

**Goal**: Full sync pipeline working

1. Create `internal/service/sync.go`
   - Activity sync with pagination
   - Stream fetching with rate limiting
   - Progress reporting via channel

2. Create `internal/store/streams.go`
   - Bulk stream insertion
   - Efficient querying

3. Implement backfill logic
   - Handle historical data
   - Resume interrupted syncs

**Verification**: Can sync all historical data (may take multiple sessions due to rate limits)

### Phase 5: Analysis Engine (Day 3-4)

**Goal**: All fitness metrics computing correctly

1. Create `internal/analysis/efficiency.go`
   - `EfficiencyFactor()`
   - `NormalizedEfficiencyFactor()`

2. Create `internal/analysis/decoupling.go`
   - `AerobicDecoupling()`
   - `CardiacDrift()`

3. Create `internal/analysis/training_load.go`
   - `TRIMP()`
   - `CalculateFitnessTrend()`

4. Create `internal/analysis/trends.go`
   - Linear regression
   - Moving averages

5. Create `internal/analysis/compute.go`
   - Orchestrate all calculations
   - Batch processing

6. Create `internal/store/metrics.go`
   - Store computed metrics

**Verification**: Metrics computed match manual calculations for sample activities

### Phase 6: TUI Shell (Day 4-5)

**Goal**: Basic navigation and dashboard displaying data

1. Create `internal/tui/styles.go`
   - Color palette
   - Component styles

2. Create `internal/tui/app.go`
   - Root model
   - Screen navigation

3. Create `internal/tui/screens/dashboard.go`
   - Current metrics display
   - Recent activities list

4. Create `internal/service/query.go`
   - Read queries for TUI
   - Aggregations

5. Wire up `main.go`
   - Initialize all components
   - Run Bubble Tea program

**Verification**: App launches, shows dashboard with real data

### Phase 7: Full TUI (Day 5-6)

**Goal**: All screens implemented

1. Create `internal/tui/components/chart.go`
   - Integrate asciigraph
   - Responsive sizing

2. Create `internal/tui/components/table.go`
   - Scrollable tables
   - Selection

3. Create `internal/tui/screens/trends.go`
   - Time range selection
   - Metric switching
   - Charts

4. Create `internal/tui/screens/activities.go`
   - Filterable list
   - Sorting
   - Search

5. Create `internal/tui/screens/detail.go`
   - Activity deep dive
   - HR/pace chart

6. Create `internal/tui/screens/sync.go`
   - Progress display
   - Cancel support

7. Create `internal/tui/screens/help.go`
   - Keybinding reference

**Verification**: All screens functional, navigation smooth

### Phase 8: Polish (Day 6-7)

**Goal**: Production-ready

1. Error handling review
   - Graceful degradation
   - User-friendly messages

2. Edge cases
   - No data yet
   - Missing HR data
   - API errors

3. Performance
   - Query optimization
   - Lazy loading

4. Documentation
   - README with screenshots
   - Installation instructions

5. Build & release
   - Cross-compilation
   - Release binaries

**Verification**: App handles all edge cases gracefully

---

## Configuration

### Config File Location

`~/.strava-fitness/config.json`

```json
{
  "strava": {
    "client_id": "YOUR_CLIENT_ID",
    "client_secret": "YOUR_CLIENT_SECRET"
  },
  "athlete": {
    "resting_hr": 50,
    "max_hr": 185,
    "threshold_hr": 165
  },
  "display": {
    "distance_unit": "km",
    "pace_unit": "min/km"
  }
}
```

### Data Directory Structure

```
~/.strava-fitness/
├── config.json          # User configuration
├── data.db              # SQLite database
└── logs/                # Optional debug logs
    └── sync.log
```

---

## Future Enhancements

These are explicitly out of scope for v1 but worth noting:

1. **Power-based metrics** - If running power data available
2. **Weather correlation** - Temperature/humidity impact on HR
3. **Route analysis** - Performance on specific routes over time
4. **Goal setting** - Target fitness levels with projections
5. **Export** - CSV/JSON export of metrics
6. **Comparison** - Compare time periods side-by-side
7. **Notifications** - Alert when fitness dropping
8. **Garmin/Wahoo** - Additional data sources
9. **Web dashboard** - Optional browser UI
10. **Mobile companion** - Quick stats on phone

---

## References

- [Strava API Documentation](https://developers.strava.com/docs/reference/)
- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss Documentation](https://github.com/charmbracelet/lipgloss)
- [Training Peaks: Efficiency Factor](https://www.trainingpeaks.com/blog/efficiency-factor-and-decoupling/)
- [Joe Friel: Aerobic Decoupling](https://www.joefrielsblog.com/2014/10/the-decoupling-test.html)
- [TRIMP: Banister Model](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC2375571/)

---

*Document Version: 1.0*
*Last Updated: January 2025*
