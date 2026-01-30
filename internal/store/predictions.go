package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrPredictionNotFound is returned when a prediction doesn't exist
var ErrPredictionNotFound = errors.New("prediction not found")

// UpsertRacePrediction inserts or updates a race prediction
func (db *DB) UpsertRacePrediction(p *RacePrediction) error {
	_, err := db.Exec(`
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
			computed_at = excluded.computed_at
	`,
		p.TargetDistance, p.TargetMeters, p.PredictedSeconds, p.PredictedPace,
		p.VDOT, p.SourceCategory, p.SourceActivityID, p.Confidence, p.ConfidenceScore,
		p.ComputedAt.Format(time.RFC3339),
	)
	return err
}

// GetAllRacePredictions retrieves all race predictions ordered by distance
func (db *DB) GetAllRacePredictions() ([]RacePrediction, error) {
	rows, err := db.Query(`
		SELECT id, target_distance, target_meters, predicted_seconds, predicted_pace,
			vdot, source_category, source_activity_id, confidence, confidence_score, computed_at
		FROM race_predictions
		ORDER BY target_meters
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRacePredictions(rows)
}

// GetRacePrediction retrieves a single prediction by target distance
func (db *DB) GetRacePrediction(targetDistance string) (*RacePrediction, error) {
	row := db.QueryRow(`
		SELECT id, target_distance, target_meters, predicted_seconds, predicted_pace,
			vdot, source_category, source_activity_id, confidence, confidence_score, computed_at
		FROM race_predictions
		WHERE target_distance = ?
	`, targetDistance)

	return scanRacePrediction(row)
}

// DeleteAllRacePredictions removes all predictions
func (db *DB) DeleteAllRacePredictions() error {
	_, err := db.Exec(`DELETE FROM race_predictions`)
	return err
}

// scanRacePrediction scans a single prediction from a row
func scanRacePrediction(row *sql.Row) (*RacePrediction, error) {
	var p RacePrediction
	var computedAt string

	err := row.Scan(
		&p.ID, &p.TargetDistance, &p.TargetMeters, &p.PredictedSeconds, &p.PredictedPace,
		&p.VDOT, &p.SourceCategory, &p.SourceActivityID, &p.Confidence, &p.ConfidenceScore,
		&computedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPredictionNotFound
	}
	if err != nil {
		return nil, err
	}

	var parseErr error
	p.ComputedAt, parseErr = time.Parse(time.RFC3339, computedAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing computed_at %q: %w", computedAt, parseErr)
	}

	return &p, nil
}

// scanRacePredictions scans multiple predictions from rows
func scanRacePredictions(rows *sql.Rows) ([]RacePrediction, error) {
	var predictions []RacePrediction

	for rows.Next() {
		var p RacePrediction
		var computedAt string

		err := rows.Scan(
			&p.ID, &p.TargetDistance, &p.TargetMeters, &p.PredictedSeconds, &p.PredictedPace,
			&p.VDOT, &p.SourceCategory, &p.SourceActivityID, &p.Confidence, &p.ConfidenceScore,
			&computedAt,
		)
		if err != nil {
			return nil, err
		}

		var parseErr error
		p.ComputedAt, parseErr = time.Parse(time.RFC3339, computedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing computed_at %q: %w", computedAt, parseErr)
		}

		predictions = append(predictions, p)
	}

	return predictions, rows.Err()
}
