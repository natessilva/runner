package analysis

import (
	"math"
	"time"

	"runner/internal/store"
)

// PredictionTarget represents a target distance for predictions
type PredictionTarget struct {
	Name          string  // "5k", "10k", "half", "marathon"
	DistanceMeters float64
}

// PredictionTargets defines the standard prediction distances
var PredictionTargets = []PredictionTarget{
	{"5k", Distance5K},
	{"10k", Distance10K},
	{"half", DistanceHalfMara},
	{"marathon", DistanceMarathon},
}

// RacePrediction represents a predicted race time
type RacePrediction struct {
	TargetName       string
	TargetMeters     float64
	PredictedSeconds int
	PredictedPace    float64 // seconds per mile
	VDOT             float64
	Confidence       string  // "high", "medium", "low"
	ConfidenceScore  float64 // 0.0 to 1.0
}

// SourcePR contains information about the PR used for predictions
type SourcePR struct {
	Category       string
	ActivityID     int64
	DistanceMeters float64
	DurationSeconds int
	AchievedAt     time.Time
}

// PRPriority defines the priority order for selecting source PRs
// Higher index = higher priority (longer race distances preferred)
var PRPriority = map[string]int{
	// Race distances (highest priority)
	"distance_full": 100,
	"distance_half": 90,
	"distance_10k":  80,
	"distance_5k":   70,
	"distance_1mi":  60,
	// Best efforts (lower priority)
	"effort_10k":    50,
	"effort_5k":     40,
	"effort_1mi":    30,
	"effort_1k":     20,
	"effort_400m":   10,
}

// SelectBestSourcePR chooses the best PR for race predictions
// Prefers longer race distances over best efforts
// Requires PR from last 365 days
func SelectBestSourcePR(prs []store.PersonalRecord) *SourcePR {
	if len(prs) == 0 {
		return nil
	}

	cutoff := time.Now().AddDate(-1, 0, 0) // 1 year ago
	var best *store.PersonalRecord
	bestPriority := -1

	for i := range prs {
		pr := &prs[i]

		// Skip PRs older than 1 year
		if pr.AchievedAt.Before(cutoff) {
			continue
		}

		// Skip categories that aren't race distances or best efforts
		priority, ok := PRPriority[pr.Category]
		if !ok {
			continue
		}

		// Select the highest priority PR
		if priority > bestPriority {
			bestPriority = priority
			best = pr
		}
	}

	if best == nil {
		return nil
	}

	return &SourcePR{
		Category:        best.Category,
		ActivityID:      best.ActivityID,
		DistanceMeters:  best.DistanceMeters,
		DurationSeconds: best.DurationSeconds,
		AchievedAt:      best.AchievedAt,
	}
}

// CalculateConfidence calculates a confidence score for a prediction
// Factors: distance extrapolation ratio, PR recency, EF trend
// Returns a score from 0.0 to 1.0
func CalculateConfidence(sourcePR *SourcePR, targetDistance float64, efTrendChange *float64) (float64, string) {
	if sourcePR == nil {
		return 0, "low"
	}

	score := 1.0

	// Factor 1: Distance extrapolation ratio
	// Predictions are less reliable when extrapolating to much longer distances
	ratio := targetDistance / sourcePR.DistanceMeters
	if ratio < 1 {
		ratio = 1 / ratio // Make ratio symmetric for shorter predictions
	}

	switch {
	case ratio > 4:
		score *= 0.7 // Large extrapolation (e.g., 5K to marathon)
	case ratio > 2:
		score *= 0.85 // Moderate extrapolation
	case ratio > 1.5:
		score *= 0.95 // Small extrapolation
	}

	// Factor 2: PR recency
	daysSincePR := time.Since(sourcePR.AchievedAt).Hours() / 24
	switch {
	case daysSincePR > 180:
		score *= 0.75 // PR older than 6 months
	case daysSincePR > 90:
		score *= 0.9 // PR older than 3 months
	case daysSincePR > 30:
		score *= 0.95 // PR older than 1 month
	}

	// Factor 3: EF trend (declining fitness reduces confidence)
	if efTrendChange != nil && *efTrendChange < -0.05 {
		// Significant decline in efficiency factor
		score *= 0.85
	}

	// Convert score to label
	var label string
	switch {
	case score >= 0.85:
		label = "high"
	case score >= 0.65:
		label = "medium"
	default:
		label = "low"
	}

	return score, label
}

// GeneratePredictions produces race time predictions for all target distances
func GeneratePredictions(sourcePR *SourcePR, efTrendChange *float64) []RacePrediction {
	if sourcePR == nil {
		return nil
	}

	// Calculate VDOT from source PR
	vdot := CalculateVDOT(sourcePR.DistanceMeters, sourcePR.DurationSeconds)
	if vdot <= 0 {
		return nil
	}

	var predictions []RacePrediction

	for _, target := range PredictionTargets {
		// Skip if target distance is too close to source distance (within 5%)
		if matchesDistance(target.DistanceMeters, sourcePR.DistanceMeters) {
			continue
		}

		predictedSeconds := PredictTime(vdot, target.DistanceMeters)
		if predictedSeconds <= 0 {
			continue
		}

		predictedPace := CalculatePacePerMile(target.DistanceMeters, predictedSeconds)
		confidenceScore, confidenceLabel := CalculateConfidence(sourcePR, target.DistanceMeters, efTrendChange)

		predictions = append(predictions, RacePrediction{
			TargetName:       target.Name,
			TargetMeters:     target.DistanceMeters,
			PredictedSeconds: predictedSeconds,
			PredictedPace:    predictedPace,
			VDOT:             vdot,
			Confidence:       confidenceLabel,
			ConfidenceScore:  math.Round(confidenceScore*100) / 100,
		})
	}

	return predictions
}

// GetCategoryDistance returns the distance in meters for a PR category
func GetCategoryDistance(category string) float64 {
	distances := map[string]float64{
		"distance_full": DistanceMarathon,
		"distance_half": DistanceHalfMara,
		"distance_10k":  Distance10K,
		"distance_5k":   Distance5K,
		"distance_1mi":  Distance1Mile,
		"effort_10k":    Distance10K,
		"effort_5k":     Distance5K,
		"effort_1mi":    Distance1Mile,
		"effort_1k":     Distance1K,
		"effort_400m":   Distance400m,
	}
	return distances[category]
}

// GetCategoryLabel returns a human-readable label for a PR category
func GetCategoryLabel(category string) string {
	labels := map[string]string{
		"distance_full": "Marathon",
		"distance_half": "Half Marathon",
		"distance_10k":  "10K",
		"distance_5k":   "5K",
		"distance_1mi":  "1 Mile",
		"effort_10k":    "10K (best effort)",
		"effort_5k":     "5K (best effort)",
		"effort_1mi":    "1 Mile (best effort)",
		"effort_1k":     "1K (best effort)",
		"effort_400m":   "400m (best effort)",
	}
	if label, ok := labels[category]; ok {
		return label
	}
	return category
}

// GetTargetLabel returns a human-readable label for a target distance
func GetTargetLabel(targetName string) string {
	labels := map[string]string{
		"5k":       "5K",
		"10k":      "10K",
		"half":     "Half Marathon",
		"marathon": "Marathon",
	}
	if label, ok := labels[targetName]; ok {
		return label
	}
	return targetName
}
