package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ErrNoAuth is returned when no authentication is stored
var ErrNoAuth = errors.New("no authentication stored")

// ErrActivityNotFound is returned when an activity doesn't exist
var ErrActivityNotFound = errors.New("activity not found")

// ErrPersonalRecordNotFound is returned when a personal record doesn't exist
var ErrPersonalRecordNotFound = errors.New("personal record not found")

// ErrPredictionNotFound is returned when a prediction doesn't exist
var ErrPredictionNotFound = errors.New("prediction not found")

// CompareMode determines how personal records are compared
type CompareMode int

const (
	CompareDuration CompareMode = iota // lower duration wins (default)
	CompareDistance                    // higher distance wins (longest_run)
	ComparePace                        // lower pace wins (fastest_pace)
)

// Open opens the SQLite database, creating it if necessary.
// The database is stored at ~/.runner/data.db
func Open() (*Store, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, fmt.Errorf("getting db path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	// Run migrations
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return newStore(db), nil
}

// getDBPath returns the path to the SQLite database file
func getDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".runner", "data.db"), nil
}
