package analysis

import (
	"testing"

	"runner/internal/store"
)

func TestFindBestEffort_BasicCase(t *testing.T) {
	// Create stream data simulating a run with varying pace
	// Simulate 1K run with best segment around meters 200-1200
	streams := make([]store.StreamPoint, 0)

	// Build stream data: 2000 meters over ~600 seconds (10 min)
	// Make the segment from 200m-1200m (1000m) faster than the rest
	for i := 0; i <= 600; i++ {
		var dist float64
		if i <= 60 {
			// First minute: slow (200m in 60s = 3.33 m/s)
			dist = float64(i) * 3.33
		} else if i <= 360 {
			// Next 5 min: fast (1000m in 300s = 3.33 m/s but we'll make it faster)
			// Actually from 60s to 360s (300s), cover 1000m at 3.7 m/s
			dist = 200 + float64(i-60)*3.7
		} else {
			// Last 4 min: slow again
			dist = 1310 + float64(i-360)*2.5
		}

		d := dist
		streams = append(streams, store.StreamPoint{
			TimeOffset: i,
			Distance:   &d,
		})
	}

	// Find best 1K effort
	effort := FindBestEffort(streams, 1000)

	if effort == nil {
		t.Fatal("Expected to find a best effort, got nil")
	}

	// The best 1K should be somewhere in the fast section
	if effort.DurationSeconds < 200 || effort.DurationSeconds > 350 {
		t.Errorf("Expected duration around 270s (1000m at 3.7 m/s), got %d", effort.DurationSeconds)
	}

	if effort.DistanceMeters < 1000 {
		t.Errorf("Expected distance >= 1000m, got %.2f", effort.DistanceMeters)
	}
}

func TestFindBestEffort_TooShort(t *testing.T) {
	// Activity shorter than target distance
	streams := make([]store.StreamPoint, 0)

	for i := 0; i <= 60; i++ {
		d := float64(i) * 5 // Only 300m total
		streams = append(streams, store.StreamPoint{
			TimeOffset: i,
			Distance:   &d,
		})
	}

	effort := FindBestEffort(streams, 1000) // Looking for 1K

	if effort != nil {
		t.Error("Expected nil for activity shorter than target distance")
	}
}

func TestFindBestEffort_EmptyStreams(t *testing.T) {
	effort := FindBestEffort([]store.StreamPoint{}, 1000)
	if effort != nil {
		t.Error("Expected nil for empty streams")
	}
}

func TestFindBestEffort_InsufficientPoints(t *testing.T) {
	streams := make([]store.StreamPoint, 5)
	for i := range streams {
		d := float64(i) * 100
		streams[i] = store.StreamPoint{
			TimeOffset: i,
			Distance:   &d,
		}
	}

	effort := FindBestEffort(streams, 400)
	if effort != nil {
		t.Error("Expected nil for insufficient points")
	}
}

func TestFindBestEffort_NoDistanceData(t *testing.T) {
	streams := make([]store.StreamPoint, 100)
	for i := range streams {
		streams[i] = store.StreamPoint{
			TimeOffset: i,
			// Distance is nil
		}
	}

	effort := FindBestEffort(streams, 400)
	if effort != nil {
		t.Error("Expected nil when no distance data available")
	}
}

func TestFindBestEffort_WithHeartrate(t *testing.T) {
	streams := make([]store.StreamPoint, 0)

	// Simulate 1K run with HR data
	for i := 0; i <= 300; i++ {
		d := float64(i) * 4.0 // 1200m in 300s
		hr := 150 + (i / 10)  // HR increases over time

		streams = append(streams, store.StreamPoint{
			TimeOffset: i,
			Distance:   &d,
			Heartrate:  &hr,
		})
	}

	effort := FindBestEffort(streams, 1000)

	if effort == nil {
		t.Fatal("Expected to find a best effort, got nil")
	}

	if effort.AvgHeartrate <= 0 {
		t.Error("Expected positive average heartrate")
	}

	// HR should be in reasonable range given our test data
	if effort.AvgHeartrate < 150 || effort.AvgHeartrate > 180 {
		t.Errorf("Average HR %.1f outside expected range", effort.AvgHeartrate)
	}
}

func TestFindBestEffort_ReturnsOffsets(t *testing.T) {
	streams := make([]store.StreamPoint, 0)

	for i := 0; i <= 200; i++ {
		d := float64(i) * 5.0 // 1000m in 200s
		streams = append(streams, store.StreamPoint{
			TimeOffset: i,
			Distance:   &d,
		})
	}

	effort := FindBestEffort(streams, 400)

	if effort == nil {
		t.Fatal("Expected to find a best effort")
	}

	if effort.StartOffset < 0 {
		t.Error("StartOffset should be non-negative")
	}

	if effort.EndOffset <= effort.StartOffset {
		t.Error("EndOffset should be greater than StartOffset")
	}
}

func TestMatchesRaceDistance(t *testing.T) {
	tests := []struct {
		activity float64
		race     float64
		expected bool
	}{
		{5000, Distance5K, true},         // Exact match
		{5100, Distance5K, true},         // Within +2%
		{4900, Distance5K, true},         // Within -2%
		{5300, Distance5K, false},        // > +5%
		{4700, Distance5K, false},        // < -5%
		{42195, DistanceMarathon, true},  // Marathon exact
		{42000, DistanceMarathon, true},  // Marathon within tolerance
		{1609, Distance1Mile, true},      // Mile exact
		{1650, Distance1Mile, true},      // Mile +2.5%
	}

	for _, tc := range tests {
		result := MatchesRaceDistance(tc.activity, tc.race)
		if result != tc.expected {
			t.Errorf("MatchesRaceDistance(%.0f, %.0f) = %v, expected %v",
				tc.activity, tc.race, result, tc.expected)
		}
	}
}

func TestGetMatchingRaceCategory(t *testing.T) {
	tests := []struct {
		distance       float64
		expectedCat    string
		expectedMatch  bool
	}{
		{5000, "distance_5k", true},
		{5100, "distance_5k", true},
		{10000, "distance_10k", true},
		{21097, "distance_half", true},
		{42195, "distance_full", true},
		{1609, "distance_1mi", true},
		{7500, "", false},  // Not a standard distance
		{3000, "", false},  // 3K not tracked
	}

	for _, tc := range tests {
		cat, _, matches := GetMatchingRaceCategory(tc.distance)
		if matches != tc.expectedMatch {
			t.Errorf("GetMatchingRaceCategory(%.0f): expected match=%v, got match=%v",
				tc.distance, tc.expectedMatch, matches)
		}
		if matches && cat != tc.expectedCat {
			t.Errorf("GetMatchingRaceCategory(%.0f): expected category=%s, got %s",
				tc.distance, tc.expectedCat, cat)
		}
	}
}

func TestCalculatePacePerMile(t *testing.T) {
	tests := []struct {
		distance    float64
		duration    int
		expectedPPM float64 // seconds per mile
	}{
		{Distance1Mile, 360, 360},           // 6:00/mile
		{Distance5K, 1200, 386.4},           // 20:00 5K ≈ 6:26/mi
		{1000, 300, 482.8},                  // 5:00/1K
	}

	for _, tc := range tests {
		result := CalculatePacePerMile(tc.distance, tc.duration)
		// Allow 1 second tolerance
		if result < tc.expectedPPM-1 || result > tc.expectedPPM+1 {
			t.Errorf("CalculatePacePerMile(%.0f, %d) = %.1f, expected ~%.1f",
				tc.distance, tc.duration, result, tc.expectedPPM)
		}
	}
}

func TestCalculatePacePerMile_ZeroInputs(t *testing.T) {
	if pace := CalculatePacePerMile(0, 300); pace != 0 {
		t.Error("Expected 0 pace for zero distance")
	}

	if pace := CalculatePacePerMile(1000, 0); pace != 0 {
		t.Error("Expected 0 pace for zero duration")
	}
}
