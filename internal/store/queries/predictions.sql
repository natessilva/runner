-- name: UpsertRacePrediction :exec
INSERT INTO race_predictions (
    target_distance, target_meters, predicted_seconds, predicted_pace,
    vdot, source_category, source_activity_id, confidence, confidence_score, computed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(target_distance) DO UPDATE SET
    target_meters = excluded.target_meters,
    predicted_seconds = excluded.predicted_seconds,
    predicted_pace = excluded.predicted_pace,
    vdot = excluded.vdot,
    source_category = excluded.source_category,
    source_activity_id = excluded.source_activity_id,
    confidence = excluded.confidence,
    confidence_score = excluded.confidence_score,
    computed_at = excluded.computed_at;

-- name: GetAllRacePredictions :many
SELECT id, target_distance, target_meters, predicted_seconds, predicted_pace,
    vdot, source_category, source_activity_id, confidence, confidence_score, computed_at
FROM race_predictions
ORDER BY target_meters;

-- name: GetRacePrediction :one
SELECT id, target_distance, target_meters, predicted_seconds, predicted_pace,
    vdot, source_category, source_activity_id, confidence, confidence_score, computed_at
FROM race_predictions
WHERE target_distance = ?;

-- name: DeleteAllRacePredictions :exec
DELETE FROM race_predictions;
