package store

import "database/sql"

// migrate runs all database migrations
func migrate(db *sql.DB) error {
	migrations := []string{
		// Authentication (singleton row)
		`CREATE TABLE IF NOT EXISTS auth (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			athlete_id INTEGER NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,

		// Activities (summary data from /athlete/activities)
		`CREATE TABLE IF NOT EXISTS activities (
			id INTEGER PRIMARY KEY,
			athlete_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			start_date TEXT NOT NULL,
			start_date_local TEXT NOT NULL,
			timezone TEXT,
			distance REAL NOT NULL,
			moving_time INTEGER NOT NULL,
			elapsed_time INTEGER NOT NULL,
			total_elevation_gain REAL,
			average_speed REAL,
			max_speed REAL,
			average_heartrate REAL,
			max_heartrate REAL,
			average_cadence REAL,
			suffer_score INTEGER,
			has_heartrate INTEGER NOT NULL,
			streams_synced INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_activities_start_date ON activities(start_date)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(type)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_has_hr ON activities(has_heartrate)`,

		// Streams (second-by-second data from /activities/{id}/streams)
		`CREATE TABLE IF NOT EXISTS streams (
			activity_id INTEGER NOT NULL,
			time_offset INTEGER NOT NULL,
			latlng_lat REAL,
			latlng_lng REAL,
			altitude REAL,
			velocity_smooth REAL,
			heartrate INTEGER,
			cadence INTEGER,
			grade_smooth REAL,
			distance REAL,
			PRIMARY KEY (activity_id, time_offset),
			FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_streams_activity ON streams(activity_id)`,

		// Computed Metrics (per activity)
		`CREATE TABLE IF NOT EXISTS activity_metrics (
			activity_id INTEGER PRIMARY KEY,
			efficiency_factor REAL,
			aerobic_decoupling REAL,
			cardiac_drift REAL,
			pace_at_z1 REAL,
			pace_at_z2 REAL,
			pace_at_z3 REAL,
			trimp REAL,
			hrss REAL,
			data_quality_score REAL,
			steady_state_pct REAL,
			computed_at TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
		)`,

		// Daily Fitness Trends
		`CREATE TABLE IF NOT EXISTS fitness_trends (
			date TEXT PRIMARY KEY,
			ctl REAL,
			atl REAL,
			tsb REAL,
			efficiency_factor_7d REAL,
			efficiency_factor_28d REAL,
			efficiency_factor_90d REAL,
			run_count_7d INTEGER,
			total_distance_7d REAL,
			total_time_7d INTEGER,
			computed_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,

		// Sync State (key-value store for sync tracking)
		`CREATE TABLE IF NOT EXISTS sync_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,

		// Personal Records (PRs for race distances and best efforts)
		`CREATE TABLE IF NOT EXISTS personal_records (
			id INTEGER PRIMARY KEY,
			category TEXT NOT NULL UNIQUE,
			activity_id INTEGER NOT NULL,
			distance_meters REAL NOT NULL,
			duration_seconds INTEGER NOT NULL,
			pace_per_mile REAL,
			avg_heartrate REAL,
			achieved_at TEXT NOT NULL,
			start_offset INTEGER,
			end_offset INTEGER,
			FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_personal_records_activity ON personal_records(activity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_personal_records_category ON personal_records(category)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return err
		}
	}

	return nil
}
