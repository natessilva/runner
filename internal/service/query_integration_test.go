package service

import (
	"database/sql"
	"testing"
	"time"

	"strava-fitness/internal/store"

	_ "modernc.org/sqlite"
)

// openTestDB creates an in-memory SQLite database with migrations applied
func openTestDB(t *testing.T) *store.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Run migrations inline (copied from store/migrations.go)
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS auth (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			athlete_id INTEGER NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
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
		`CREATE TABLE IF NOT EXISTS sync_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			db.Close()
			t.Fatalf("failed to run migration: %v", err)
		}
	}

	// Wrap in store.DB by embedding
	return &store.DB{DB: db}
}

// Helper to create a float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}

// createTestActivity inserts a test activity into the database
func createTestActivity(t *testing.T, db *store.DB, id int64, name string, startDate time.Time, distance float64, movingTime int, avgHR *float64) {
	t.Helper()
	activity := &store.Activity{
		ID:               id,
		AthleteID:        12345,
		Name:             name,
		Type:             "Run",
		StartDate:        startDate,
		StartDateLocal:   startDate,
		Distance:         distance,
		MovingTime:       movingTime,
		ElapsedTime:      movingTime + 60,
		AverageHeartrate: avgHR,
		HasHeartrate:     avgHR != nil,
		StreamsSynced:    true,
	}
	if err := db.UpsertActivity(activity); err != nil {
		t.Fatalf("failed to create test activity: %v", err)
	}
}

// createTestMetrics inserts test metrics for an activity
func createTestMetrics(t *testing.T, db *store.DB, activityID int64, ef, trimp *float64) {
	t.Helper()
	metrics := &store.ActivityMetrics{
		ActivityID:       activityID,
		EfficiencyFactor: ef,
		TRIMP:            trimp,
	}
	if err := db.SaveActivityMetrics(metrics); err != nil {
		t.Fatalf("failed to create test metrics: %v", err)
	}
}

// createTestStreams inserts test stream data for an activity
func createTestStreams(t *testing.T, db *store.DB, activityID int64, numPoints int, velocity float64, hr int) {
	t.Helper()
	points := make([]store.StreamPoint, numPoints)
	for i := range points {
		dist := float64(i) * velocity // cumulative distance
		points[i] = store.StreamPoint{
			ActivityID:     activityID,
			TimeOffset:     i,
			VelocitySmooth: &velocity,
			Heartrate:      &hr,
			Distance:       &dist,
		}
	}
	if err := db.SaveStreams(activityID, points); err != nil {
		t.Fatalf("failed to create test streams: %v", err)
	}
}

func TestQueryService_GetActivitiesList(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	svc := NewQueryService(db, 185)

	// Create test activities with metrics
	now := time.Now()
	createTestActivity(t, db, 1, "Morning Run", now, 5000, 1800, floatPtr(150))
	createTestMetrics(t, db, 1, floatPtr(1.2), floatPtr(100))

	createTestActivity(t, db, 2, "Evening Run", now.Add(-24*time.Hour), 10000, 3600, floatPtr(155))
	createTestMetrics(t, db, 2, floatPtr(1.3), floatPtr(180))

	createTestActivity(t, db, 3, "Long Run", now.Add(-48*time.Hour), 20000, 7200, floatPtr(145))
	createTestMetrics(t, db, 3, floatPtr(1.15), floatPtr(250))

	t.Run("returns activities in date order", func(t *testing.T) {
		results, err := svc.GetActivitiesList(10, 0)
		if err != nil {
			t.Fatalf("GetActivitiesList failed: %v", err)
		}

		if len(results) != 3 {
			t.Fatalf("expected 3 activities, got %d", len(results))
		}

		// Should be ordered by date descending (most recent first)
		if results[0].Activity.ID != 1 {
			t.Errorf("expected first activity ID=1, got %d", results[0].Activity.ID)
		}
		if results[1].Activity.ID != 2 {
			t.Errorf("expected second activity ID=2, got %d", results[1].Activity.ID)
		}
		if results[2].Activity.ID != 3 {
			t.Errorf("expected third activity ID=3, got %d", results[2].Activity.ID)
		}
	})

	t.Run("pagination works", func(t *testing.T) {
		results, err := svc.GetActivitiesList(2, 0)
		if err != nil {
			t.Fatalf("GetActivitiesList failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 activities with limit=2, got %d", len(results))
		}

		results, err = svc.GetActivitiesList(2, 2)
		if err != nil {
			t.Fatalf("GetActivitiesList with offset failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 activity with offset=2, got %d", len(results))
		}
		if results[0].Activity.ID != 3 {
			t.Errorf("expected activity ID=3 at offset=2, got %d", results[0].Activity.ID)
		}
	})

	t.Run("includes metrics", func(t *testing.T) {
		results, err := svc.GetActivitiesList(10, 0)
		if err != nil {
			t.Fatalf("GetActivitiesList failed: %v", err)
		}

		if results[0].Metrics.EfficiencyFactor == nil {
			t.Error("expected metrics.EfficiencyFactor to be set")
		} else if *results[0].Metrics.EfficiencyFactor != 1.2 {
			t.Errorf("expected EF=1.2, got %v", *results[0].Metrics.EfficiencyFactor)
		}
	})
}

func TestQueryService_GetActivityDetail(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	svc := NewQueryService(db, 185)

	now := time.Now()
	createTestActivity(t, db, 100, "Test Run", now, 8000, 2400, floatPtr(152))
	createTestMetrics(t, db, 100, floatPtr(1.25), floatPtr(150))
	createTestStreams(t, db, 100, 300, 3.0, 150)

	t.Run("returns activity with metrics and streams", func(t *testing.T) {
		result, streams, err := svc.GetActivityDetail(100)
		if err != nil {
			t.Fatalf("GetActivityDetail failed: %v", err)
		}

		if result.Activity.ID != 100 {
			t.Errorf("expected activity ID=100, got %d", result.Activity.ID)
		}
		if result.Activity.Name != "Test Run" {
			t.Errorf("expected name='Test Run', got %q", result.Activity.Name)
		}
		if result.Metrics.EfficiencyFactor == nil || *result.Metrics.EfficiencyFactor != 1.25 {
			t.Errorf("expected EF=1.25, got %v", result.Metrics.EfficiencyFactor)
		}
		if len(streams) != 300 {
			t.Errorf("expected 300 stream points, got %d", len(streams))
		}
	})

	t.Run("returns error for non-existent activity", func(t *testing.T) {
		_, _, err := svc.GetActivityDetail(999)
		if err == nil {
			t.Error("expected error for non-existent activity")
		}
	})
}

func TestQueryService_GetActivityDetailByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	svc := NewQueryService(db, 185)

	now := time.Now()
	// Create a 5km run with stream data
	createTestActivity(t, db, 200, "5K Run", now, 5000, 1500, floatPtr(160))
	createTestMetrics(t, db, 200, floatPtr(1.1), floatPtr(120))

	// Create streams with distance data for split calculation
	points := make([]store.StreamPoint, 1500)
	for i := range points {
		vel := 3.33 // ~5:00/km pace
		hr := 160
		dist := float64(i) * vel
		cad := 90
		points[i] = store.StreamPoint{
			ActivityID:     200,
			TimeOffset:     i,
			VelocitySmooth: &vel,
			Heartrate:      &hr,
			Distance:       &dist,
			Cadence:        &cad,
		}
	}
	if err := db.SaveStreams(200, points); err != nil {
		t.Fatalf("failed to save streams: %v", err)
	}

	t.Run("calculates splits and HR zones", func(t *testing.T) {
		detail, err := svc.GetActivityDetailByID(200)
		if err != nil {
			t.Fatalf("GetActivityDetailByID failed: %v", err)
		}

		if detail.Activity.Activity.ID != 200 {
			t.Errorf("expected activity ID=200, got %d", detail.Activity.Activity.ID)
		}

		// Should have calculated some splits (distance is ~5km)
		if len(detail.Splits) == 0 {
			t.Error("expected at least one split")
		}

		// Should have HR zones
		if len(detail.HRZones) != 5 {
			t.Errorf("expected 5 HR zones, got %d", len(detail.HRZones))
		}

		// Should have average HR and cadence
		if detail.AvgHR == 0 {
			t.Error("expected non-zero AvgHR")
		}
		if detail.AvgCadence == 0 {
			t.Error("expected non-zero AvgCadence")
		}
	})
}

func TestQueryService_GetTotalActivityCount(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	svc := NewQueryService(db, 185)

	t.Run("returns zero for empty database", func(t *testing.T) {
		count, err := svc.GetTotalActivityCount()
		if err != nil {
			t.Fatalf("GetTotalActivityCount failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})

	// Add some activities
	now := time.Now()
	createTestActivity(t, db, 1, "Run 1", now, 5000, 1800, floatPtr(150))
	createTestActivity(t, db, 2, "Run 2", now, 5000, 1800, floatPtr(150))
	createTestActivity(t, db, 3, "Run 3", now, 5000, 1800, floatPtr(150))

	t.Run("returns correct count", func(t *testing.T) {
		count, err := svc.GetTotalActivityCount()
		if err != nil {
			t.Fatalf("GetTotalActivityCount failed: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3, got %d", count)
		}
	})
}

func TestQueryService_GetPeriodStats(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	svc := NewQueryService(db, 185)

	// Create activities spread across multiple weeks
	now := time.Now()
	for i := 0; i < 5; i++ {
		id := int64(i + 1)
		date := now.AddDate(0, 0, -i*7) // one per week
		createTestActivity(t, db, id, "Weekly Run", date, 10000, 3600, floatPtr(150))
		createTestMetrics(t, db, id, floatPtr(1.2), floatPtr(100))
		createTestStreams(t, db, id, 100, 3.0, 150)
	}

	t.Run("weekly stats", func(t *testing.T) {
		stats, err := svc.GetPeriodStats("weekly", 4)
		if err != nil {
			t.Fatalf("GetPeriodStats failed: %v", err)
		}

		if len(stats) != 4 {
			t.Fatalf("expected 4 weekly periods, got %d", len(stats))
		}

		// Each period should have a label
		for i, s := range stats {
			if s.PeriodLabel == "" {
				t.Errorf("period %d has empty label", i)
			}
		}
	})

	t.Run("monthly stats", func(t *testing.T) {
		stats, err := svc.GetPeriodStats("monthly", 3)
		if err != nil {
			t.Fatalf("GetPeriodStats failed: %v", err)
		}

		if len(stats) != 3 {
			t.Fatalf("expected 3 monthly periods, got %d", len(stats))
		}
	})
}

func TestQueryService_GetDashboardData(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	svc := NewQueryService(db, 185)

	t.Run("handles empty database", func(t *testing.T) {
		data, err := svc.GetDashboardData()
		if err != nil {
			t.Fatalf("GetDashboardData failed: %v", err)
		}

		if data == nil {
			t.Fatal("expected non-nil data")
		}
		if len(data.RecentActivities) != 0 {
			t.Errorf("expected 0 recent activities, got %d", len(data.RecentActivities))
		}
	})

	// Add activities with metrics
	now := time.Now()
	for i := 0; i < 5; i++ {
		id := int64(i + 1)
		date := now.AddDate(0, 0, -i)
		createTestActivity(t, db, id, "Daily Run", date, 8000, 2400, floatPtr(150))
		createTestMetrics(t, db, id, floatPtr(1.2+float64(i)*0.01), floatPtr(100))
		createTestStreams(t, db, id, 100, 3.0, 150)
	}

	t.Run("returns dashboard data with activities", func(t *testing.T) {
		data, err := svc.GetDashboardData()
		if err != nil {
			t.Fatalf("GetDashboardData failed: %v", err)
		}

		if len(data.RecentActivities) != 5 {
			t.Errorf("expected 5 recent activities, got %d", len(data.RecentActivities))
		}

		// Should have calculated current EF
		if data.CurrentEF == 0 {
			t.Error("expected non-zero CurrentEF")
		}

		// Should have week stats
		if data.WeekRunCount == 0 {
			t.Error("expected non-zero WeekRunCount")
		}
	})
}
