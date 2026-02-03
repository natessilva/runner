package store

import (
	"context"
	"fmt"
	"time"

	"runner/internal/store/sqlc"
)

// GetActivitiesByIDs retrieves multiple activities by their IDs.
// Returns a map of activity ID to activity for easy lookup.
// This method uses dynamic SQL for the IN clause, which sqlc cannot generate.
func (s *Store) GetActivitiesByIDs(ids []int64) (map[int64]*Activity, error) {
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

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]*Activity)
	for rows.Next() {
		var a Activity
		var startDate, startDateLocal string
		var timezone *string
		var totalElevationGain, averageSpeed, maxSpeed, avgHR, maxHR, avgCadence *float64
		var sufferScore *int64
		var hasHR, streamsSynced int64

		err := rows.Scan(
			&a.ID, &a.AthleteID, &a.Name, &a.Type, &startDate, &startDateLocal, &timezone,
			&a.Distance, &a.MovingTime, &a.ElapsedTime, &totalElevationGain,
			&averageSpeed, &maxSpeed, &avgHR, &maxHR,
			&avgCadence, &sufferScore, &hasHR, &streamsSynced,
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

		if timezone != nil {
			a.Timezone = *timezone
		}
		if totalElevationGain != nil {
			a.TotalElevationGain = *totalElevationGain
		}
		if averageSpeed != nil {
			a.AverageSpeed = *averageSpeed
		}
		if maxSpeed != nil {
			a.MaxSpeed = *maxSpeed
		}
		a.AverageHeartrate = avgHR
		a.MaxHeartrate = maxHR
		a.AverageCadence = avgCadence
		if sufferScore != nil {
			ss := int(*sufferScore)
			a.SufferScore = &ss
		}
		a.HasHeartrate = hasHR == 1
		a.StreamsSynced = streamsSynced == 1

		result[a.ID] = &a
	}

	return result, rows.Err()
}

// GetStreamsForActivities retrieves stream points for multiple activities in a single query.
// Returns a map from activity ID to stream points, sorted by time offset.
// This method uses dynamic SQL for the IN clause, which sqlc cannot generate.
func (s *Store) GetStreamsForActivities(activityIDs []int64) (map[int64][]StreamPoint, error) {
	if len(activityIDs) == 0 {
		return make(map[int64][]StreamPoint), nil
	}

	// Build query with placeholders
	query := `
		SELECT activity_id, time_offset, latlng_lat, latlng_lng, altitude,
			velocity_smooth, heartrate, cadence, grade_smooth, distance
		FROM streams
		WHERE activity_id IN (`

	args := make([]interface{}, len(activityIDs))
	for i, id := range activityIDs {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args[i] = id
	}
	query += `) ORDER BY activity_id, time_offset`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]StreamPoint)
	for rows.Next() {
		var p StreamPoint
		err := rows.Scan(
			&p.ActivityID, &p.TimeOffset, &p.Lat, &p.Lng, &p.Altitude,
			&p.VelocitySmooth, &p.Heartrate, &p.Cadence, &p.GradeSmooth, &p.Distance,
		)
		if err != nil {
			return nil, err
		}
		result[p.ActivityID] = append(result[p.ActivityID], p)
	}

	return result, rows.Err()
}

// SaveStreams saves stream data for an activity.
// It replaces any existing stream data for the activity.
// This method uses transactions and prepared statements for efficiency.
func (s *Store) SaveStreams(activityID int64, points []StreamPoint) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Use sqlc's WithTx for the delete
	qtx := s.queries.WithTx(tx)
	if err := qtx.DeleteStreamsForActivity(context.Background(), activityID); err != nil {
		return fmt.Errorf("deleting existing streams: %w", err)
	}

	// Prepare insert statement for batch efficiency
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

// InsertStreamPoint inserts a single stream point.
// For bulk inserts, use SaveStreams instead.
func (s *Store) InsertStreamPoint(p StreamPoint) error {
	return s.queries.InsertStreamPoint(context.Background(), sqlc.InsertStreamPointParams{
		ActivityID:     p.ActivityID,
		TimeOffset:     int64(p.TimeOffset),
		LatlngLat:      ptrToNullFloat64(p.Lat),
		LatlngLng:      ptrToNullFloat64(p.Lng),
		Altitude:       ptrToNullFloat64(p.Altitude),
		VelocitySmooth: ptrToNullFloat64(p.VelocitySmooth),
		Heartrate:      ptrIntToNullInt64(p.Heartrate),
		Cadence:        ptrIntToNullInt64(p.Cadence),
		GradeSmooth:    ptrToNullFloat64(p.GradeSmooth),
		Distance:       ptrToNullFloat64(p.Distance),
	})
}

// joinStrings joins strings with a separator.
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
