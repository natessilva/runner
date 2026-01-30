package analysis

import (
	"testing"
	"time"

	"runner/internal/store"
)

func TestSelectBestSourcePR(t *testing.T) {
	now := time.Now()
	recent := now.AddDate(0, -1, 0)  // 1 month ago
	old := now.AddDate(-2, 0, 0)     // 2 years ago

	tests := []struct {
		name     string
		prs      []store.PersonalRecord
		wantCat  string
		wantNil  bool
	}{
		{
			name:    "empty PRs returns nil",
			prs:     nil,
			wantNil: true,
		},
		{
			name: "prefers marathon over half",
			prs: []store.PersonalRecord{
				{Category: "distance_half", AchievedAt: recent, DistanceMeters: DistanceHalfMara, DurationSeconds: 5400},
				{Category: "distance_full", AchievedAt: recent, DistanceMeters: DistanceMarathon, DurationSeconds: 11400},
			},
			wantCat: "distance_full",
		},
		{
			name: "prefers race distance over best effort",
			prs: []store.PersonalRecord{
				{Category: "effort_10k", AchievedAt: recent, DistanceMeters: Distance10K, DurationSeconds: 2400},
				{Category: "distance_5k", AchievedAt: recent, DistanceMeters: Distance5K, DurationSeconds: 1200},
			},
			wantCat: "distance_5k",
		},
		{
			name: "skips old PRs",
			prs: []store.PersonalRecord{
				{Category: "distance_full", AchievedAt: old, DistanceMeters: DistanceMarathon, DurationSeconds: 11400},
				{Category: "distance_5k", AchievedAt: recent, DistanceMeters: Distance5K, DurationSeconds: 1200},
			},
			wantCat: "distance_5k",
		},
		{
			name: "returns nil if all PRs are old",
			prs: []store.PersonalRecord{
				{Category: "distance_full", AchievedAt: old, DistanceMeters: DistanceMarathon, DurationSeconds: 11400},
				{Category: "distance_half", AchievedAt: old, DistanceMeters: DistanceHalfMara, DurationSeconds: 5400},
			},
			wantNil: true,
		},
		{
			name: "ignores non-runnable categories",
			prs: []store.PersonalRecord{
				{Category: "longest_run", AchievedAt: recent, DistanceMeters: 50000, DurationSeconds: 18000},
				{Category: "effort_5k", AchievedAt: recent, DistanceMeters: Distance5K, DurationSeconds: 1200},
			},
			wantCat: "effort_5k",
		},
		{
			name: "uses best effort if no race distances",
			prs: []store.PersonalRecord{
				{Category: "effort_1k", AchievedAt: recent, DistanceMeters: Distance1K, DurationSeconds: 210},
				{Category: "effort_5k", AchievedAt: recent, DistanceMeters: Distance5K, DurationSeconds: 1200},
			},
			wantCat: "effort_5k",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectBestSourcePR(tt.prs)
			if tt.wantNil {
				if got != nil {
					t.Errorf("SelectBestSourcePR() = %v, want nil", got.Category)
				}
				return
			}
			if got == nil {
				t.Fatalf("SelectBestSourcePR() = nil, want %v", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("SelectBestSourcePR() = %v, want %v", got.Category, tt.wantCat)
			}
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		sourcePR        *SourcePR
		targetDistance  float64
		efTrendChange   *float64
		wantLabel       string
		wantScoreMin    float64
		wantScoreMax    float64
	}{
		{
			name:           "nil source PR",
			sourcePR:       nil,
			targetDistance: Distance5K,
			wantLabel:      "low",
			wantScoreMax:   0,
		},
		{
			name: "recent 10K to 5K (short extrapolation, recent)",
			sourcePR: &SourcePR{
				Category:        "distance_10k",
				DistanceMeters:  Distance10K,
				DurationSeconds: 2400,
				AchievedAt:      now.AddDate(0, 0, -7), // 1 week ago
			},
			targetDistance: Distance5K,
			wantLabel:      "high",
			wantScoreMin:   0.85,
		},
		{
			name: "5K to marathon (large extrapolation)",
			sourcePR: &SourcePR{
				Category:        "distance_5k",
				DistanceMeters:  Distance5K,
				DurationSeconds: 1200,
				AchievedAt:      now.AddDate(0, 0, -7),
			},
			targetDistance: DistanceMarathon,
			wantLabel:      "medium",
			wantScoreMin:   0.65,
			wantScoreMax:   0.85,
		},
		{
			name: "old PR reduces confidence",
			sourcePR: &SourcePR{
				Category:        "distance_10k",
				DistanceMeters:  Distance10K,
				DurationSeconds: 2400,
				AchievedAt:      now.AddDate(0, -7, 0), // 7 months ago
			},
			targetDistance: Distance5K,
			wantLabel:      "medium",
		},
		{
			name: "declining EF reduces confidence",
			sourcePR: &SourcePR{
				Category:        "distance_10k",
				DistanceMeters:  Distance10K,
				DurationSeconds: 2400,
				AchievedAt:      now.AddDate(0, 0, -7),
			},
			targetDistance: Distance5K,
			efTrendChange:  floatPtr(-0.1), // 10% decline
			wantLabel:      "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, label := CalculateConfidence(tt.sourcePR, tt.targetDistance, tt.efTrendChange)

			if label != tt.wantLabel {
				t.Errorf("CalculateConfidence() label = %v, want %v", label, tt.wantLabel)
			}

			if tt.wantScoreMin > 0 && score < tt.wantScoreMin {
				t.Errorf("CalculateConfidence() score = %v, want >= %v", score, tt.wantScoreMin)
			}

			if tt.wantScoreMax > 0 && score > tt.wantScoreMax {
				t.Errorf("CalculateConfidence() score = %v, want <= %v", score, tt.wantScoreMax)
			}
		})
	}
}

func TestGeneratePredictions(t *testing.T) {
	now := time.Now()

	t.Run("nil source returns nil", func(t *testing.T) {
		got := GeneratePredictions(nil, nil)
		if got != nil {
			t.Errorf("GeneratePredictions(nil) = %v, want nil", got)
		}
	})

	t.Run("generates predictions for all target distances except source", func(t *testing.T) {
		source := &SourcePR{
			Category:        "distance_5k",
			DistanceMeters:  Distance5K,
			DurationSeconds: 1200, // 20:00 5K
			AchievedAt:      now.AddDate(0, 0, -7),
		}

		predictions := GeneratePredictions(source, nil)

		// Should have predictions for 10K, half, marathon (not 5K since that's source)
		if len(predictions) != 3 {
			t.Fatalf("GeneratePredictions() = %d predictions, want 3", len(predictions))
		}

		// Check that 5K is not included
		for _, p := range predictions {
			if p.TargetName == "5k" {
				t.Error("GeneratePredictions() should not include source distance")
			}
		}

		// Check that predictions are reasonable
		for _, p := range predictions {
			if p.PredictedSeconds <= 0 {
				t.Errorf("Prediction for %s has invalid time: %d", p.TargetName, p.PredictedSeconds)
			}
			if p.VDOT <= 0 {
				t.Errorf("Prediction for %s has invalid VDOT: %f", p.TargetName, p.VDOT)
			}
			if p.Confidence == "" {
				t.Errorf("Prediction for %s has empty confidence", p.TargetName)
			}
		}
	})

	t.Run("predictions get progressively longer", func(t *testing.T) {
		source := &SourcePR{
			Category:        "distance_5k",
			DistanceMeters:  Distance5K,
			DurationSeconds: 1200,
			AchievedAt:      now.AddDate(0, 0, -7),
		}

		predictions := GeneratePredictions(source, nil)

		for i := 1; i < len(predictions); i++ {
			if predictions[i].PredictedSeconds <= predictions[i-1].PredictedSeconds {
				t.Errorf("Predictions should increase: %s (%d) <= %s (%d)",
					predictions[i].TargetName, predictions[i].PredictedSeconds,
					predictions[i-1].TargetName, predictions[i-1].PredictedSeconds)
			}
		}
	})
}

func TestGetCategoryDistance(t *testing.T) {
	tests := []struct {
		category     string
		wantDistance float64
	}{
		{"distance_full", DistanceMarathon},
		{"distance_half", DistanceHalfMara},
		{"distance_10k", Distance10K},
		{"distance_5k", Distance5K},
		{"distance_1mi", Distance1Mile},
		{"effort_10k", Distance10K},
		{"effort_5k", Distance5K},
		{"effort_1mi", Distance1Mile},
		{"effort_1k", Distance1K},
		{"effort_400m", Distance400m},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			got := GetCategoryDistance(tt.category)
			if got != tt.wantDistance {
				t.Errorf("GetCategoryDistance(%s) = %v, want %v", tt.category, got, tt.wantDistance)
			}
		})
	}
}

func TestGetTargetLabel(t *testing.T) {
	tests := []struct {
		target    string
		wantLabel string
	}{
		{"5k", "5K"},
		{"10k", "10K"},
		{"half", "Half Marathon"},
		{"marathon", "Marathon"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := GetTargetLabel(tt.target)
			if got != tt.wantLabel {
				t.Errorf("GetTargetLabel(%s) = %v, want %v", tt.target, got, tt.wantLabel)
			}
		})
	}
}

