package service

import (
	"runner/internal/analysis"
	"runner/internal/store"
)

// PredictionDisplay represents a formatted prediction for display
type PredictionDisplay struct {
	TargetDistance   string  // "5k", "10k", "half", "marathon"
	TargetLabel      string  // "5K", "10K", "Half Marathon", "Marathon"
	PredictedTime    string  // formatted duration "M:SS" or "H:MM:SS"
	PredictedPace    string  // formatted pace "M:SS/mi"
	Confidence       string  // "High", "Medium", "Low"
	ConfidenceScore  float64
}

// PredictionsData contains all data needed for the predictions screen
type PredictionsData struct {
	Predictions    []PredictionDisplay
	VDOT           float64
	VDOTLabel      string // "Advanced Recreational", "Competitive", etc.
	SourceCategory string // "10K PR", "5K (best effort)", etc.
	SourceDate     string // "Oct 15, 2025"
	SourceTime     string // formatted source PR time
	LastUpdated    string // when predictions were computed
	HasPredictions bool
}

// GetRacePredictions retrieves all race predictions formatted for display
func (q *QueryService) GetRacePredictions() (*PredictionsData, error) {
	predictions, err := q.store.GetAllRacePredictions()
	if err != nil {
		return nil, err
	}

	data := &PredictionsData{
		HasPredictions: len(predictions) > 0,
	}

	if len(predictions) == 0 {
		return data, nil
	}

	// Get source PR info from the first prediction (all should have same source)
	firstPred := predictions[0]
	data.VDOT = firstPred.VDOT
	data.VDOTLabel = analysis.GetVDOTLabel(firstPred.VDOT)
	data.SourceCategory = formatSourceCategory(firstPred.SourceCategory)
	data.LastUpdated = firstPred.ComputedAt.Format("Jan 02, 2006")

	// Get source activity for date and time info
	sourcePR, err := q.store.GetPersonalRecordByCategory(firstPred.SourceCategory)
	if err == nil && sourcePR != nil {
		data.SourceDate = sourcePR.AchievedAt.Format("Jan 02, 2006")
		data.SourceTime = formatDuration(sourcePR.DurationSeconds)
	}

	// Format predictions
	for _, p := range predictions {
		display := PredictionDisplay{
			TargetDistance:   p.TargetDistance,
			TargetLabel:      analysis.GetTargetLabel(p.TargetDistance),
			PredictedTime:    formatDuration(p.PredictedSeconds),
			PredictedPace:    formatPace(int(p.PredictedPace)),
			Confidence:       capitalizeFirst(p.Confidence),
			ConfidenceScore:  p.ConfidenceScore,
		}
		data.Predictions = append(data.Predictions, display)
	}

	return data, nil
}

// formatSourceCategory returns a human-readable label for the source PR category
func formatSourceCategory(category string) string {
	labels := map[string]string{
		"distance_full": "Marathon PR",
		"distance_half": "Half Marathon PR",
		"distance_10k":  "10K PR",
		"distance_5k":   "5K PR",
		"distance_1mi":  "1 Mile PR",
		"effort_10k":    "10K Best Effort",
		"effort_5k":     "5K Best Effort",
		"effort_1mi":    "1 Mile Best Effort",
		"effort_1k":     "1K Best Effort",
		"effort_400m":   "400m Best Effort",
	}
	if label, ok := labels[category]; ok {
		return label
	}
	return category
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// GetSourcePRInfo retrieves information about the PR used for predictions
func (q *QueryService) GetSourcePRInfo(predictions []store.RacePrediction) (*store.PersonalRecord, *store.Activity, error) {
	if len(predictions) == 0 {
		return nil, nil, nil
	}

	// Get the source PR
	pr, err := q.store.GetPersonalRecordByCategory(predictions[0].SourceCategory)
	if err != nil {
		return nil, nil, err
	}

	// Get the source activity
	activity, err := q.store.GetActivity(pr.ActivityID)
	if err != nil {
		return pr, nil, err
	}

	return pr, activity, nil
}
