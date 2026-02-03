-- name: SaveActivityMetrics :exec
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
    computed_at = CURRENT_TIMESTAMP;

-- name: GetActivityMetrics :one
SELECT activity_id, efficiency_factor, aerobic_decoupling, cardiac_drift,
    pace_at_z1, pace_at_z2, pace_at_z3, trimp, hrss,
    data_quality_score, steady_state_pct
FROM activity_metrics
WHERE activity_id = ?;

-- name: HasMetrics :one
SELECT 1 FROM activity_metrics WHERE activity_id = ? LIMIT 1;

-- name: GetAllMetrics :many
SELECT m.activity_id, m.efficiency_factor, m.aerobic_decoupling, m.cardiac_drift,
    m.pace_at_z1, m.pace_at_z2, m.pace_at_z3, m.trimp, m.hrss,
    m.data_quality_score, m.steady_state_pct
FROM activity_metrics m
JOIN activities a ON m.activity_id = a.id
ORDER BY a.start_date DESC;

-- name: CountMetrics :one
SELECT COUNT(*) FROM activity_metrics;

-- name: GetActivitiesWithMetricsRaw :many
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
LIMIT ? OFFSET ?;
