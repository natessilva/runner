# Runner

A terminal-based aerobic fitness analyzer that syncs with Strava to track your running fitness trends.

## Features

- **Efficiency Factor (EF)** - Track your pace-to-heart-rate ratio over time
- **Aerobic Decoupling** - Monitor cardiac drift during runs (<5% indicates good aerobic base)
- **Training Load** - TRIMP-based load calculation with CTL/ATL/TSB metrics
- **Interactive Dashboard** - Scrollable TUI with charts for mileage, HR, cadence trends
- **Activity Browser** - Paginated list of all synced activities with metrics

## Installation

```bash
go install github.com/natessilva/runner@latest
```

Or build from source:

```bash
git clone https://github.com/natessilva/runner.git
cd runner
go build -o runner .
```

## Setup

### 1. Create a Strava API Application

1. Go to [https://www.strava.com/settings/api](https://www.strava.com/settings/api)
2. Create a new application with these settings:
   - **Application Name**: Anything you like (e.g., "My Fitness Analyzer")
   - **Category**: Choose any
   - **Website**: `http://localhost`
   - **Authorization Callback Domain**: `localhost`
3. Note your **Client ID** and **Client Secret**

### 2. Configure the App

Run the app once to generate a config file:

```bash
runner
```

This creates `~/.runner/config.json`. Edit it with your Strava credentials:

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
    "distance_unit": "mi",
    "pace_unit": "min/mi"
  }
}
```

#### Configuration Options

| Field | Description | Default |
|-------|-------------|---------|
| `strava.client_id` | Your Strava API client ID | Required |
| `strava.client_secret` | Your Strava API client secret | Required |
| `athlete.resting_hr` | Your resting heart rate | 50 |
| `athlete.max_hr` | Your maximum heart rate | 185 |
| `athlete.threshold_hr` | Your lactate threshold HR | 165 |

### 3. Authenticate with Strava

Run the app again:

```bash
runner
```

A browser window will open for Strava OAuth. Authorize the app to access your activities.

## Usage

Once authenticated, the TUI launches automatically.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `1` | Dashboard |
| `2` | Activities list |
| `3` or `s` | Sync with Strava |
| `?` | Help |
| `q` | Quit |
| `j/k` or arrows | Scroll |
| `r` | Refresh data |

### Dashboard

The dashboard shows:
- **Current Fitness** - EF, CTL (fitness), ATL (fatigue), TSB (form)
- **This Week** - Run count, distance, time, average EF
- **Charts** - EF trend, weekly mileage, cadence, and heart rate
- **Recent Activities** - Last 5 runs with key metrics

### Metrics Explained

| Metric | Description |
|--------|-------------|
| **EF (Efficiency Factor)** | Speed per heartbeat. Higher = better aerobic fitness |
| **Decoupling** | HR drift vs pace. <5% = good aerobic base |
| **TRIMP** | Training impulse (duration x intensity) |
| **CTL (Fitness)** | 42-day exponential average of TRIMP |
| **ATL (Fatigue)** | 7-day exponential average of TRIMP |
| **TSB (Form)** | CTL - ATL. Positive = fresh, negative = fatigued |

## Data Storage

All data is stored locally in `~/.runner/`:
- `config.json` - Your configuration
- `data.db` - SQLite database with activities and metrics

## Rate Limits

The app respects Strava's API rate limits:
- 100 requests per 15 minutes
- 1,000 requests per day

The sync screen shows current rate limit status.

## License

MIT
