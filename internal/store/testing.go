package store

import (
	"database/sql"
)

// NewTestStore creates a Store for testing with an in-memory database.
// This is only intended for use in tests.
func NewTestStore(sqlDB *sql.DB) *Store {
	return newStore(sqlDB)
}
