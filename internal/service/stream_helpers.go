package service

import (
	"fmt"
	"time"

	"runner/internal/store"
)

// StreamStats holds aggregated metrics from stream points
type StreamStats struct {
	HRSum        float64
	HRCount      int
	CadenceSum   float64
	CadenceCount int
}

// AggregateStreamStats calculates HR and cadence stats from streams
func AggregateStreamStats(streams []store.StreamPoint) StreamStats {
	var stats StreamStats
	for _, p := range streams {
		if isValidHeartrate(p.Heartrate) {
			stats.HRSum += float64(*p.Heartrate)
			stats.HRCount++
		}
		if isValidCadence(p.Cadence) {
			stats.CadenceSum += float64(*p.Cadence) * StravaCadenceMultiplier
			stats.CadenceCount++
		}
	}
	return stats
}

// AvgHR returns the average heart rate, or 0 if no valid readings
func (s StreamStats) AvgHR() float64 {
	if s.HRCount == 0 {
		return 0
	}
	return s.HRSum / float64(s.HRCount)
}

// AvgCadence returns the average cadence, or 0 if no valid readings
func (s StreamStats) AvgCadence() float64 {
	if s.CadenceCount == 0 {
		return 0
	}
	return s.CadenceSum / float64(s.CadenceCount)
}

// isValidHeartrate checks if HR is in valid range
func isValidHeartrate(hr *int) bool {
	return hr != nil && *hr > MinValidHeartrate && *hr < MaxValidHeartrate
}

// isValidCadence checks if cadence is present and positive
func isValidCadence(cad *int) bool {
	return cad != nil && *cad > 0
}

// metersToMiles converts distance from meters to miles
func metersToMiles(meters float64) float64 {
	return meters / MetersPerMile
}

// getMonday returns the Monday of the week containing t, at midnight
func getMonday(t time.Time) time.Time {
	daysFromMonday := (int(t.Weekday()) + 6) % 7 // Monday = 0
	monday := t.AddDate(0, 0, -daysFromMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
}

// formatDuration formats seconds as "H:MM:SS" or "M:SS"
func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
