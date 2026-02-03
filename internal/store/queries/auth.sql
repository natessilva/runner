-- name: GetAuth :one
SELECT athlete_id, access_token, refresh_token, expires_at
FROM auth
WHERE id = 1;

-- name: SaveAuth :exec
INSERT INTO auth (id, athlete_id, access_token, refresh_token, expires_at, updated_at)
VALUES (1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
    athlete_id = excluded.athlete_id,
    access_token = excluded.access_token,
    refresh_token = excluded.refresh_token,
    expires_at = excluded.expires_at,
    updated_at = CURRENT_TIMESTAMP;

-- name: UpdateTokens :execresult
UPDATE auth
SET access_token = ?, refresh_token = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = 1;
