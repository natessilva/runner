package store

import "database/sql"

// GetSyncState retrieves a sync state value by key
// Returns empty string if key doesn't exist
func (db *DB) GetSyncState(key string) (string, error) {
	var value string
	err := db.QueryRow(`
		SELECT value FROM sync_state WHERE key = ?
	`, key).Scan(&value)

	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSyncState sets a sync state value
func (db *DB) SetSyncState(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO sync_state (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
