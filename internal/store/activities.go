package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrActivityNotFound is returned when an activity doesn't exist
var ErrActivityNotFound = errors.New("activity not found")

// UpsertActivity inserts or updates an activity
func (db *DB) UpsertActivity(a *Activity) error {
	_, err := db.Exec(`
		INSERT INTO activities (
			id, athlete_id, name, type, start_date, start_date_local, timezone,
			distance, moving_time, elapsed_time, total_elevation_gain,
			average_speed, max_speed, average_heartrate, max_heartrate,
			average_cadence, suffer_score, has_heartrate, streams_synced, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			athlete_id = excluded.athlete_id,
			name = excluded.name,
			type = excluded.type,
			start_date = excluded.start_date,
			start_date_local = excluded.start_date_local,
			timezone = excluded.timezone,
			distance = excluded.distance,
			moving_time = excluded.moving_time,
			elapsed_time = excluded.elapsed_time,
			total_elevation_gain = excluded.total_elevation_gain,
			average_speed = excluded.average_speed,
			max_speed = excluded.max_speed,
			average_heartrate = excluded.average_heartrate,
			max_heartrate = excluded.max_heartrate,
			average_cadence = excluded.average_cadence,
			suffer_score = excluded.suffer_score,
			has_heartrate = excluded.has_heartrate,
			updated_at = CURRENT_TIMESTAMP
	`,
		a.ID, a.AthleteID, a.Name, a.Type,
		a.StartDate.Format(time.RFC3339), a.StartDateLocal.Format(time.RFC3339), a.Timezone,
		a.Distance, a.MovingTime, a.ElapsedTime, a.TotalElevationGain,
		a.AverageSpeed, a.MaxSpeed, a.AverageHeartrate, a.MaxHeartrate,
		a.AverageCadence, a.SufferScore, boolToInt(a.HasHeartrate), boolToInt(a.StreamsSynced),
	)
	return err
}

// GetActivity retrieves an activity by ID
func (db *DB) GetActivity(id int64) (*Activity, error) {
	row := db.QueryRow(`
		SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
			distance, moving_time, elapsed_time, total_elevation_gain,
			average_speed, max_speed, average_heartrate, max_heartrate,
			average_cadence, suffer_score, has_heartrate, streams_synced
		FROM activities
		WHERE id = ?
	`, id)

	return scanActivity(row)
}

// ListActivities returns activities ordered by start date descending
func (db *DB) ListActivities(limit, offset int) ([]Activity, error) {
	rows, err := db.Query(`
		SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
			distance, moving_time, elapsed_time, total_elevation_gain,
			average_speed, max_speed, average_heartrate, max_heartrate,
			average_cadence, suffer_score, has_heartrate, streams_synced
		FROM activities
		ORDER BY start_date DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanActivities(rows)
}

// GetActivitiesNeedingStreams returns activities that haven't had their streams synced
func (db *DB) GetActivitiesNeedingStreams(limit int) ([]Activity, error) {
	rows, err := db.Query(`
		SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
			distance, moving_time, elapsed_time, total_elevation_gain,
			average_speed, max_speed, average_heartrate, max_heartrate,
			average_cadence, suffer_score, has_heartrate, streams_synced
		FROM activities
		WHERE streams_synced = 0 AND has_heartrate = 1
		ORDER BY start_date DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanActivities(rows)
}

// MarkStreamsSynced marks an activity's streams as synced
func (db *DB) MarkStreamsSynced(id int64) error {
	result, err := db.Exec(`
		UPDATE activities
		SET streams_synced = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrActivityNotFound
	}
	return nil
}

// CountActivities returns the total number of activities
func (db *DB) CountActivities() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM activities").Scan(&count)
	return count, err
}

// scanActivity scans a single activity from a row
func scanActivity(row *sql.Row) (*Activity, error) {
	var a Activity
	var startDate, startDateLocal string
	var hasHR, streamsSynced int

	err := row.Scan(
		&a.ID, &a.AthleteID, &a.Name, &a.Type, &startDate, &startDateLocal, &a.Timezone,
		&a.Distance, &a.MovingTime, &a.ElapsedTime, &a.TotalElevationGain,
		&a.AverageSpeed, &a.MaxSpeed, &a.AverageHeartrate, &a.MaxHeartrate,
		&a.AverageCadence, &a.SufferScore, &hasHR, &streamsSynced,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrActivityNotFound
	}
	if err != nil {
		return nil, err
	}

	var parseErr error
	a.StartDate, parseErr = time.Parse(time.RFC3339, startDate)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing start_date %q: %w", startDate, parseErr)
	}
	a.StartDateLocal, parseErr = time.Parse(time.RFC3339, startDateLocal)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing start_date_local %q: %w", startDateLocal, parseErr)
	}
	a.HasHeartrate = hasHR == 1
	a.StreamsSynced = streamsSynced == 1

	return &a, nil
}

// scanActivities scans multiple activities from rows
func scanActivities(rows *sql.Rows) ([]Activity, error) {
	var activities []Activity

	for rows.Next() {
		var a Activity
		var startDate, startDateLocal string
		var hasHR, streamsSynced int

		err := rows.Scan(
			&a.ID, &a.AthleteID, &a.Name, &a.Type, &startDate, &startDateLocal, &a.Timezone,
			&a.Distance, &a.MovingTime, &a.ElapsedTime, &a.TotalElevationGain,
			&a.AverageSpeed, &a.MaxSpeed, &a.AverageHeartrate, &a.MaxHeartrate,
			&a.AverageCadence, &a.SufferScore, &hasHR, &streamsSynced,
		)
		if err != nil {
			return nil, err
		}

		var parseErr error
		a.StartDate, parseErr = time.Parse(time.RFC3339, startDate)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing start_date %q: %w", startDate, parseErr)
		}
		a.StartDateLocal, parseErr = time.Parse(time.RFC3339, startDateLocal)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing start_date_local %q: %w", startDateLocal, parseErr)
		}
		a.HasHeartrate = hasHR == 1
		a.StreamsSynced = streamsSynced == 1

		activities = append(activities, a)
	}

	return activities, rows.Err()
}

// GetActivitiesByIDs retrieves multiple activities by their IDs
// Returns a map of activity ID to activity for easy lookup
func (db *DB) GetActivitiesByIDs(ids []int64) (map[int64]*Activity, error) {
	if len(ids) == 0 {
		return make(map[int64]*Activity), nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
			distance, moving_time, elapsed_time, total_elevation_gain,
			average_speed, max_speed, average_heartrate, max_heartrate,
			average_cadence, suffer_score, has_heartrate, streams_synced
		FROM activities
		WHERE id IN (` + joinStrings(placeholders, ",") + `)`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]*Activity)
	for rows.Next() {
		var a Activity
		var startDate, startDateLocal string
		var hasHR, streamsSynced int

		err := rows.Scan(
			&a.ID, &a.AthleteID, &a.Name, &a.Type, &startDate, &startDateLocal, &a.Timezone,
			&a.Distance, &a.MovingTime, &a.ElapsedTime, &a.TotalElevationGain,
			&a.AverageSpeed, &a.MaxSpeed, &a.AverageHeartrate, &a.MaxHeartrate,
			&a.AverageCadence, &a.SufferScore, &hasHR, &streamsSynced,
		)
		if err != nil {
			return nil, err
		}

		var parseErr error
		a.StartDate, parseErr = time.Parse(time.RFC3339, startDate)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing start_date %q: %w", startDate, parseErr)
		}
		a.StartDateLocal, parseErr = time.Parse(time.RFC3339, startDateLocal)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing start_date_local %q: %w", startDateLocal, parseErr)
		}
		a.HasHeartrate = hasHR == 1
		a.StreamsSynced = streamsSynced == 1

		result[a.ID] = &a
	}

	return result, rows.Err()
}

// joinStrings joins strings with a separator (simple helper to avoid importing strings)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
