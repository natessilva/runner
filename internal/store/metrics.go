package store

import (
	"database/sql"
	"time"
)

// SaveActivityMetrics stores computed metrics for an activity
func (db *DB) SaveActivityMetrics(m *ActivityMetrics) error {
	_, err := db.Exec(`
		INSERT INTO activity_metrics (
			activity_id, efficiency_factor, aerobic_decoupling, cardiac_drift,
			pace_at_z1, pace_at_z2, pace_at_z3, trimp, hrss,
			data_quality_score, steady_state_pct, computed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(activity_id) DO UPDATE SET
			efficiency_factor = excluded.efficiency_factor,
			aerobic_decoupling = excluded.aerobic_decoupling,
			cardiac_drift = excluded.cardiac_drift,
			pace_at_z1 = excluded.pace_at_z1,
			pace_at_z2 = excluded.pace_at_z2,
			pace_at_z3 = excluded.pace_at_z3,
			trimp = excluded.trimp,
			hrss = excluded.hrss,
			data_quality_score = excluded.data_quality_score,
			steady_state_pct = excluded.steady_state_pct,
			computed_at = CURRENT_TIMESTAMP
	`,
		m.ActivityID, m.EfficiencyFactor, m.AerobicDecoupling, m.CardiacDrift,
		m.PaceAtZ1, m.PaceAtZ2, m.PaceAtZ3, m.TRIMP, m.HRSS,
		m.DataQualityScore, m.SteadyStatePct,
	)
	return err
}

// GetActivityMetrics retrieves computed metrics for an activity
func (db *DB) GetActivityMetrics(activityID int64) (*ActivityMetrics, error) {
	row := db.QueryRow(`
		SELECT activity_id, efficiency_factor, aerobic_decoupling, cardiac_drift,
			pace_at_z1, pace_at_z2, pace_at_z3, trimp, hrss,
			data_quality_score, steady_state_pct
		FROM activity_metrics
		WHERE activity_id = ?
	`, activityID)

	var m ActivityMetrics
	err := row.Scan(
		&m.ActivityID, &m.EfficiencyFactor, &m.AerobicDecoupling, &m.CardiacDrift,
		&m.PaceAtZ1, &m.PaceAtZ2, &m.PaceAtZ3, &m.TRIMP, &m.HRSS,
		&m.DataQualityScore, &m.SteadyStatePct,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// HasMetrics checks if an activity has computed metrics
func (db *DB) HasMetrics(activityID int64) (bool, error) {
	var exists int
	err := db.QueryRow(`
		SELECT 1 FROM activity_metrics WHERE activity_id = ? LIMIT 1
	`, activityID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetAllMetrics retrieves metrics for all activities, ordered by date
func (db *DB) GetAllMetrics() ([]ActivityMetrics, error) {
	rows, err := db.Query(`
		SELECT m.activity_id, m.efficiency_factor, m.aerobic_decoupling, m.cardiac_drift,
			m.pace_at_z1, m.pace_at_z2, m.pace_at_z3, m.trimp, m.hrss,
			m.data_quality_score, m.steady_state_pct
		FROM activity_metrics m
		JOIN activities a ON m.activity_id = a.id
		ORDER BY a.start_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []ActivityMetrics
	for rows.Next() {
		var m ActivityMetrics
		err := rows.Scan(
			&m.ActivityID, &m.EfficiencyFactor, &m.AerobicDecoupling, &m.CardiacDrift,
			&m.PaceAtZ1, &m.PaceAtZ2, &m.PaceAtZ3, &m.TRIMP, &m.HRSS,
			&m.DataQualityScore, &m.SteadyStatePct,
		)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// CountMetrics returns the number of activities with computed metrics
func (db *DB) CountMetrics() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM activity_metrics").Scan(&count)
	return count, err
}

// GetActivitiesWithMetrics retrieves activities that have computed metrics
func (db *DB) GetActivitiesWithMetrics(limit, offset int) ([]Activity, []ActivityMetrics, error) {
	rows, err := db.Query(`
		SELECT a.id, a.athlete_id, a.name, a.type, a.start_date, a.start_date_local, a.timezone,
			a.distance, a.moving_time, a.elapsed_time, a.total_elevation_gain,
			a.average_speed, a.max_speed, a.average_heartrate, a.max_heartrate,
			a.average_cadence, a.suffer_score, a.has_heartrate, a.streams_synced,
			m.efficiency_factor, m.aerobic_decoupling, m.cardiac_drift,
			m.pace_at_z1, m.pace_at_z2, m.pace_at_z3, m.trimp, m.hrss,
			m.data_quality_score, m.steady_state_pct
		FROM activities a
		JOIN activity_metrics m ON a.id = m.activity_id
		ORDER BY a.start_date DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var activities []Activity
	var metrics []ActivityMetrics

	for rows.Next() {
		var a Activity
		var m ActivityMetrics
		var startDate, startDateLocal string
		var hasHR, streamsSynced int

		err := rows.Scan(
			&a.ID, &a.AthleteID, &a.Name, &a.Type, &startDate, &startDateLocal, &a.Timezone,
			&a.Distance, &a.MovingTime, &a.ElapsedTime, &a.TotalElevationGain,
			&a.AverageSpeed, &a.MaxSpeed, &a.AverageHeartrate, &a.MaxHeartrate,
			&a.AverageCadence, &a.SufferScore, &hasHR, &streamsSynced,
			&m.EfficiencyFactor, &m.AerobicDecoupling, &m.CardiacDrift,
			&m.PaceAtZ1, &m.PaceAtZ2, &m.PaceAtZ3, &m.TRIMP, &m.HRSS,
			&m.DataQualityScore, &m.SteadyStatePct,
		)
		if err != nil {
			return nil, nil, err
		}

		a.StartDate, _ = parseTime(startDate)
		a.StartDateLocal, _ = parseTime(startDateLocal)
		a.HasHeartrate = hasHR == 1
		a.StreamsSynced = streamsSynced == 1
		m.ActivityID = a.ID

		activities = append(activities, a)
		metrics = append(metrics, m)
	}

	return activities, metrics, rows.Err()
}

// parseTime parses a time string in RFC3339 format
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
