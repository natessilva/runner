-- name: GetSyncState :one
SELECT value FROM sync_state WHERE key = ?;

-- name: SetSyncState :exec
INSERT INTO sync_state (key, value, updated_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    updated_at = CURRENT_TIMESTAMP;
