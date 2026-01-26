package store

import (
	"database/sql"
	"fmt"
)

// SaveStreams saves stream data for an activity
// It replaces any existing stream data for the activity
func (db *DB) SaveStreams(activityID int64, points []StreamPoint) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing streams for this activity
	if _, err := tx.Exec("DELETE FROM streams WHERE activity_id = ?", activityID); err != nil {
		return fmt.Errorf("deleting existing streams: %w", err)
	}

	// Prepare insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO streams (
			activity_id, time_offset, latlng_lat, latlng_lng, altitude,
			velocity_smooth, heartrate, cadence, grade_smooth, distance
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	// Insert all points
	for _, p := range points {
		_, err := stmt.Exec(
			p.ActivityID, p.TimeOffset, p.Lat, p.Lng, p.Altitude,
			p.VelocitySmooth, p.Heartrate, p.Cadence, p.GradeSmooth, p.Distance,
		)
		if err != nil {
			return fmt.Errorf("inserting stream point: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetStreams retrieves all stream points for an activity
func (db *DB) GetStreams(activityID int64) ([]StreamPoint, error) {
	rows, err := db.Query(`
		SELECT activity_id, time_offset, latlng_lat, latlng_lng, altitude,
			velocity_smooth, heartrate, cadence, grade_smooth, distance
		FROM streams
		WHERE activity_id = ?
		ORDER BY time_offset
	`, activityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []StreamPoint
	for rows.Next() {
		var p StreamPoint
		err := rows.Scan(
			&p.ActivityID, &p.TimeOffset, &p.Lat, &p.Lng, &p.Altitude,
			&p.VelocitySmooth, &p.Heartrate, &p.Cadence, &p.GradeSmooth, &p.Distance,
		)
		if err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, rows.Err()
}

// GetStreamCount returns the number of stream points for an activity
func (db *DB) GetStreamCount(activityID int64) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM streams WHERE activity_id = ?", activityID).Scan(&count)
	return count, err
}

// HasStreams checks if an activity has stream data
func (db *DB) HasStreams(activityID int64) (bool, error) {
	var exists int
	err := db.QueryRow(`
		SELECT 1 FROM streams WHERE activity_id = ? LIMIT 1
	`, activityID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetActivitiesNeedingMetrics returns activities that have streams but no computed metrics
func (db *DB) GetActivitiesNeedingMetrics() ([]Activity, error) {
	rows, err := db.Query(`
		SELECT a.id, a.athlete_id, a.name, a.type, a.start_date, a.start_date_local, a.timezone,
			a.distance, a.moving_time, a.elapsed_time, a.total_elevation_gain,
			a.average_speed, a.max_speed, a.average_heartrate, a.max_heartrate,
			a.average_cadence, a.suffer_score, a.has_heartrate, a.streams_synced
		FROM activities a
		WHERE a.streams_synced = 1
		AND NOT EXISTS (SELECT 1 FROM activity_metrics m WHERE m.activity_id = a.id)
		ORDER BY a.start_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanActivities(rows)
}

// DeleteStreams removes all stream data for an activity
func (db *DB) DeleteStreams(activityID int64) error {
	_, err := db.Exec("DELETE FROM streams WHERE activity_id = ?", activityID)
	return err
}
