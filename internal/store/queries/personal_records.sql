-- name: InsertPersonalRecord :exec
INSERT INTO personal_records (
    category, activity_id, distance_meters, duration_seconds,
    pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(category) DO UPDATE SET
    activity_id = excluded.activity_id,
    distance_meters = excluded.distance_meters,
    duration_seconds = excluded.duration_seconds,
    pace_per_mile = excluded.pace_per_mile,
    avg_heartrate = excluded.avg_heartrate,
    achieved_at = excluded.achieved_at,
    start_offset = excluded.start_offset,
    end_offset = excluded.end_offset;

-- name: GetPersonalRecordByCategory :one
SELECT id, category, activity_id, distance_meters, duration_seconds,
    pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
FROM personal_records
WHERE category = ?;

-- name: GetAllPersonalRecords :many
SELECT id, category, activity_id, distance_meters, duration_seconds,
    pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
FROM personal_records
ORDER BY category;

-- name: GetPersonalRecordsForActivity :many
SELECT id, category, activity_id, distance_meters, duration_seconds,
    pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
FROM personal_records
WHERE activity_id = ?
ORDER BY category;

-- name: DeletePersonalRecordsForActivity :exec
DELETE FROM personal_records WHERE activity_id = ?;
