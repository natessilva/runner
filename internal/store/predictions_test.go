package store

import (
	"testing"
	"time"
)

func TestRacePredictions(t *testing.T) {
	db := setupTestDB(t) // Uses activity IDs 1 and 2 from the setup

	now := time.Now().Truncate(time.Second)

	t.Run("UpsertRacePrediction inserts new prediction", func(t *testing.T) {
		prediction := &RacePrediction{
			TargetDistance:   "10k",
			TargetMeters:     10000,
			PredictedSeconds: 2400,
			PredictedPace:    386.4,
			VDOT:             50.0,
			SourceCategory:   "distance_5k",
			SourceActivityID: 1, // Use activity ID from setupTestDB
			Confidence:       "high",
			ConfidenceScore:  0.95,
			ComputedAt:       now,
		}

		err := db.UpsertRacePrediction(prediction)
		if err != nil {
			t.Fatalf("UpsertRacePrediction() error = %v", err)
		}

		// Verify it was inserted
		got, err := db.GetRacePrediction("10k")
		if err != nil {
			t.Fatalf("GetRacePrediction() error = %v", err)
		}

		if got.TargetDistance != "10k" {
			t.Errorf("TargetDistance = %v, want 10k", got.TargetDistance)
		}
		if got.PredictedSeconds != 2400 {
			t.Errorf("PredictedSeconds = %v, want 2400", got.PredictedSeconds)
		}
		if got.VDOT != 50.0 {
			t.Errorf("VDOT = %v, want 50.0", got.VDOT)
		}
	})

	t.Run("UpsertRacePrediction updates existing prediction", func(t *testing.T) {
		prediction := &RacePrediction{
			TargetDistance:   "10k",
			TargetMeters:     10000,
			PredictedSeconds: 2300, // Updated time
			PredictedPace:    370.0,
			VDOT:             52.0, // Updated VDOT
			SourceCategory:   "distance_5k",
			SourceActivityID: 1,
			Confidence:       "high",
			ConfidenceScore:  0.98,
			ComputedAt:       now,
		}

		err := db.UpsertRacePrediction(prediction)
		if err != nil {
			t.Fatalf("UpsertRacePrediction() error = %v", err)
		}

		got, err := db.GetRacePrediction("10k")
		if err != nil {
			t.Fatalf("GetRacePrediction() error = %v", err)
		}

		if got.PredictedSeconds != 2300 {
			t.Errorf("PredictedSeconds = %v, want 2300", got.PredictedSeconds)
		}
		if got.VDOT != 52.0 {
			t.Errorf("VDOT = %v, want 52.0", got.VDOT)
		}
	})

	t.Run("GetAllRacePredictions returns all predictions ordered by distance", func(t *testing.T) {
		// Add more predictions
		predictions := []*RacePrediction{
			{
				TargetDistance:   "5k",
				TargetMeters:     5000,
				PredictedSeconds: 1100,
				PredictedPace:    354.0,
				VDOT:             52.0,
				SourceCategory:   "distance_10k",
				SourceActivityID: 1,
				Confidence:       "high",
				ConfidenceScore:  0.95,
				ComputedAt:       now,
			},
			{
				TargetDistance:   "half",
				TargetMeters:     21097,
				PredictedSeconds: 5100,
				PredictedPace:    389.0,
				VDOT:             52.0,
				SourceCategory:   "distance_10k",
				SourceActivityID: 1,
				Confidence:       "medium",
				ConfidenceScore:  0.75,
				ComputedAt:       now,
			},
		}

		for _, p := range predictions {
			if err := db.UpsertRacePrediction(p); err != nil {
				t.Fatalf("UpsertRacePrediction() error = %v", err)
			}
		}

		all, err := db.GetAllRacePredictions()
		if err != nil {
			t.Fatalf("GetAllRacePredictions() error = %v", err)
		}

		if len(all) != 3 {
			t.Fatalf("GetAllRacePredictions() returned %d predictions, want 3", len(all))
		}

		// Should be ordered by target_meters
		if all[0].TargetDistance != "5k" {
			t.Errorf("First prediction = %v, want 5k", all[0].TargetDistance)
		}
		if all[1].TargetDistance != "10k" {
			t.Errorf("Second prediction = %v, want 10k", all[1].TargetDistance)
		}
		if all[2].TargetDistance != "half" {
			t.Errorf("Third prediction = %v, want half", all[2].TargetDistance)
		}
	})

	t.Run("GetRacePrediction returns error for non-existent prediction", func(t *testing.T) {
		_, err := db.GetRacePrediction("marathon")
		if err != ErrPredictionNotFound {
			t.Errorf("GetRacePrediction() error = %v, want ErrPredictionNotFound", err)
		}
	})

	t.Run("DeleteAllRacePredictions clears all predictions", func(t *testing.T) {
		err := db.DeleteAllRacePredictions()
		if err != nil {
			t.Fatalf("DeleteAllRacePredictions() error = %v", err)
		}

		all, err := db.GetAllRacePredictions()
		if err != nil {
			t.Fatalf("GetAllRacePredictions() error = %v", err)
		}

		if len(all) != 0 {
			t.Errorf("GetAllRacePredictions() returned %d predictions after delete, want 0", len(all))
		}
	})
}
