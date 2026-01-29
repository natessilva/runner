package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrPersonalRecordNotFound is returned when a personal record doesn't exist
var ErrPersonalRecordNotFound = errors.New("personal record not found")

// CompareMode determines how personal records are compared
type CompareMode int

const (
	CompareDuration CompareMode = iota // lower duration wins (default)
	CompareDistance                    // higher distance wins (longest_run)
	ComparePace                        // lower pace wins (fastest_pace)
)

// UpsertPersonalRecord inserts or updates a personal record
// Only updates if the new record is faster (lower duration for same distance category)
func (db *DB) UpsertPersonalRecord(pr *PersonalRecord) (updated bool, err error) {
	return db.UpsertPersonalRecordWithMode(pr, CompareDuration)
}

// UpsertPersonalRecordWithMode inserts or updates a personal record with the specified comparison mode.
// - CompareDuration: lower duration wins (default, for race times)
// - CompareDistance: higher distance wins (longest_run, highest_elevation)
// - ComparePace: lower pace wins (fastest_pace)
func (db *DB) UpsertPersonalRecordWithMode(pr *PersonalRecord, mode CompareMode) (updated bool, err error) {
	// Check if a record already exists for this category
	existing, err := db.GetPersonalRecordByCategory(pr.Category)
	if err != nil && !errors.Is(err, ErrPersonalRecordNotFound) {
		return false, err
	}

	// Compare based on mode
	if existing != nil {
		switch mode {
		case CompareDuration:
			if existing.DurationSeconds <= pr.DurationSeconds {
				return false, nil
			}
		case CompareDistance:
			if existing.DistanceMeters >= pr.DistanceMeters {
				return false, nil
			}
		case ComparePace:
			if existing.PacePerMile != nil && pr.PacePerMile != nil && *existing.PacePerMile <= *pr.PacePerMile {
				return false, nil
			}
		}
	}

	_, err = db.Exec(`
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
			end_offset = excluded.end_offset
	`,
		pr.Category, pr.ActivityID, pr.DistanceMeters, pr.DurationSeconds,
		pr.PacePerMile, pr.AvgHeartrate, pr.AchievedAt.Format(time.RFC3339),
		pr.StartOffset, pr.EndOffset,
	)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetPersonalRecordByCategory retrieves a personal record by category
func (db *DB) GetPersonalRecordByCategory(category string) (*PersonalRecord, error) {
	row := db.QueryRow(`
		SELECT id, category, activity_id, distance_meters, duration_seconds,
			pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
		FROM personal_records
		WHERE category = ?
	`, category)

	return scanPersonalRecord(row)
}

// GetAllPersonalRecords retrieves all personal records
func (db *DB) GetAllPersonalRecords() ([]PersonalRecord, error) {
	rows, err := db.Query(`
		SELECT id, category, activity_id, distance_meters, duration_seconds,
			pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
		FROM personal_records
		ORDER BY category
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPersonalRecords(rows)
}

// GetPersonalRecordsForActivity retrieves all personal records achieved during a specific activity
func (db *DB) GetPersonalRecordsForActivity(activityID int64) ([]PersonalRecord, error) {
	rows, err := db.Query(`
		SELECT id, category, activity_id, distance_meters, duration_seconds,
			pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
		FROM personal_records
		WHERE activity_id = ?
		ORDER BY category
	`, activityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPersonalRecords(rows)
}

// GetPreviousRecord retrieves the previous record for a category before a given activity
// This is useful for showing improvement (e.g., "previous: 24:12")
func (db *DB) GetPreviousRecord(category string, currentActivityID int64) (*PersonalRecord, error) {
	// Since we only keep the best record, we can't actually get the previous one
	// This would require storing history. For now, return nil.
	return nil, nil
}

// DeletePersonalRecordsForActivity removes all PRs associated with an activity
// This is useful when an activity is deleted
func (db *DB) DeletePersonalRecordsForActivity(activityID int64) error {
	_, err := db.Exec(`DELETE FROM personal_records WHERE activity_id = ?`, activityID)
	return err
}

// scanPersonalRecord scans a single personal record from a row
func scanPersonalRecord(row *sql.Row) (*PersonalRecord, error) {
	var pr PersonalRecord
	var achievedAt string

	err := row.Scan(
		&pr.ID, &pr.Category, &pr.ActivityID, &pr.DistanceMeters, &pr.DurationSeconds,
		&pr.PacePerMile, &pr.AvgHeartrate, &achievedAt, &pr.StartOffset, &pr.EndOffset,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPersonalRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	var parseErr error
	pr.AchievedAt, parseErr = time.Parse(time.RFC3339, achievedAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing achieved_at %q: %w", achievedAt, parseErr)
	}
	return &pr, nil
}

// scanPersonalRecords scans multiple personal records from rows
func scanPersonalRecords(rows *sql.Rows) ([]PersonalRecord, error) {
	var records []PersonalRecord

	for rows.Next() {
		var pr PersonalRecord
		var achievedAt string

		err := rows.Scan(
			&pr.ID, &pr.Category, &pr.ActivityID, &pr.DistanceMeters, &pr.DurationSeconds,
			&pr.PacePerMile, &pr.AvgHeartrate, &achievedAt, &pr.StartOffset, &pr.EndOffset,
		)
		if err != nil {
			return nil, err
		}

		var parseErr error
		pr.AchievedAt, parseErr = time.Parse(time.RFC3339, achievedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing achieved_at %q: %w", achievedAt, parseErr)
		}
		records = append(records, pr)
	}

	return records, rows.Err()
}
