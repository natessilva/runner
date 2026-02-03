-- name: DeleteStreamsForActivity :exec
DELETE FROM streams WHERE activity_id = ?;

-- name: InsertStreamPoint :exec
INSERT INTO streams (
    activity_id, time_offset, latlng_lat, latlng_lng, altitude,
    velocity_smooth, heartrate, cadence, grade_smooth, distance
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetStreams :many
SELECT activity_id, time_offset, latlng_lat, latlng_lng, altitude,
    velocity_smooth, heartrate, cadence, grade_smooth, distance
FROM streams
WHERE activity_id = ?
ORDER BY time_offset;

-- name: GetStreamCount :one
SELECT COUNT(*) FROM streams WHERE activity_id = ?;

-- name: HasStreams :one
SELECT 1 FROM streams WHERE activity_id = ? LIMIT 1;

-- name: DeleteStreams :exec
DELETE FROM streams WHERE activity_id = ?;
