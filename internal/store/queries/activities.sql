-- name: UpsertActivity :exec
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
    updated_at = CURRENT_TIMESTAMP;

-- name: GetActivity :one
SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
    distance, moving_time, elapsed_time, total_elevation_gain,
    average_speed, max_speed, average_heartrate, max_heartrate,
    average_cadence, suffer_score, has_heartrate, streams_synced
FROM activities
WHERE id = ?;

-- name: ListActivities :many
SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
    distance, moving_time, elapsed_time, total_elevation_gain,
    average_speed, max_speed, average_heartrate, max_heartrate,
    average_cadence, suffer_score, has_heartrate, streams_synced
FROM activities
ORDER BY start_date DESC
LIMIT ? OFFSET ?;

-- name: GetActivitiesNeedingStreams :many
SELECT id, athlete_id, name, type, start_date, start_date_local, timezone,
    distance, moving_time, elapsed_time, total_elevation_gain,
    average_speed, max_speed, average_heartrate, max_heartrate,
    average_cadence, suffer_score, has_heartrate, streams_synced
FROM activities
WHERE streams_synced = 0 AND has_heartrate = 1
ORDER BY start_date DESC
LIMIT ?;

-- name: MarkStreamsSynced :execresult
UPDATE activities
SET streams_synced = 1, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CountActivities :one
SELECT COUNT(*) FROM activities;

-- name: GetActivitiesNeedingMetrics :many
SELECT a.id, a.athlete_id, a.name, a.type, a.start_date, a.start_date_local, a.timezone,
    a.distance, a.moving_time, a.elapsed_time, a.total_elevation_gain,
    a.average_speed, a.max_speed, a.average_heartrate, a.max_heartrate,
    a.average_cadence, a.suffer_score, a.has_heartrate, a.streams_synced
FROM activities a
WHERE a.streams_synced = 1
AND NOT EXISTS (SELECT 1 FROM activity_metrics m WHERE m.activity_id = a.id)
ORDER BY a.start_date DESC;
