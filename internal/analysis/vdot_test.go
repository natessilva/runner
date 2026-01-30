package analysis

import (
	"math"
	"testing"
)

func TestCalculateVDOT(t *testing.T) {
	tests := []struct {
		name            string
		distanceMeters  float64
		durationSeconds int
		wantVDOT        float64
		tolerance       float64
	}{
		{
			name:            "5K at 19:00 (VDOT ~50)",
			distanceMeters:  Distance5K,
			durationSeconds: 1140, // 19:00 - matches VDOT 50 in table
			wantVDOT:        50.0,
			tolerance:       1.0,
		},
		{
			name:            "5K at 23:42 (VDOT ~40)",
			distanceMeters:  Distance5K,
			durationSeconds: 1422, // 23:42 - matches VDOT 40 in table
			wantVDOT:        40.0,
			tolerance:       1.0,
		},
		{
			name:            "10K at 39:24 (VDOT ~50)",
			distanceMeters:  Distance10K,
			durationSeconds: 2364, // 39:24 - matches VDOT 50 in table
			wantVDOT:        50.0,
			tolerance:       1.0,
		},
		{
			name:            "Marathon at 2:54:54 (VDOT ~50)",
			distanceMeters:  DistanceMarathon,
			durationSeconds: 10494, // 2:54:54 - matches VDOT 50 in table
			wantVDOT:        50.0,
			tolerance:       1.0,
		},
		{
			name:            "Half marathon at 1:25:00 (VDOT ~50)",
			distanceMeters:  DistanceHalfMara,
			durationSeconds: 5100, // 1:25:00 - matches VDOT 50 in table
			wantVDOT:        50.0,
			tolerance:       1.0,
		},
		{
			name:            "1 Mile at 5:44 (VDOT ~50)",
			distanceMeters:  Distance1Mile,
			durationSeconds: 344, // 5:44 - matches VDOT 50 in table
			wantVDOT:        50.0,
			tolerance:       1.0,
		},
		{
			name:            "Elite 5K at 13:00 (VDOT ~75+)",
			distanceMeters:  Distance5K,
			durationSeconds: 786, // 13:06 - matches VDOT 75 in table
			wantVDOT:        75.0,
			tolerance:       2.0,
		},
		{
			name:            "Beginner 5K at 31:00 (VDOT ~31)",
			distanceMeters:  Distance5K,
			durationSeconds: 1806, // 30:06 - matches VDOT 31 in table
			wantVDOT:        31.0,
			tolerance:       1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateVDOT(tt.distanceMeters, tt.durationSeconds)
			if math.Abs(got-tt.wantVDOT) > tt.tolerance {
				t.Errorf("CalculateVDOT() = %v, want %v (±%v)", got, tt.wantVDOT, tt.tolerance)
			}
		})
	}
}

func TestCalculateVDOT_EdgeCases(t *testing.T) {
	// Zero duration should return 0
	if got := CalculateVDOT(Distance5K, 0); got != 0 {
		t.Errorf("CalculateVDOT with zero duration = %v, want 0", got)
	}

	// Negative duration should return 0
	if got := CalculateVDOT(Distance5K, -100); got != 0 {
		t.Errorf("CalculateVDOT with negative duration = %v, want 0", got)
	}

	// Very slow time should return minimum VDOT
	got := CalculateVDOT(Distance5K, 3600) // 1 hour 5K
	if got > 30 {
		t.Errorf("CalculateVDOT for very slow time = %v, want <= 30", got)
	}

	// Very fast time should return maximum VDOT
	got = CalculateVDOT(Distance5K, 600) // 10:00 5K (world record pace)
	if got < 80 {
		t.Errorf("CalculateVDOT for very fast time = %v, want >= 80", got)
	}
}

func TestPredictTime(t *testing.T) {
	tests := []struct {
		name           string
		vdot           float64
		targetDistance float64
		wantSeconds    int
		tolerance      int
	}{
		{
			name:           "VDOT 50 predicting 5K",
			vdot:           50.0,
			targetDistance: Distance5K,
			wantSeconds:    1140, // 19:00
			tolerance:      60,   // ±1 min
		},
		{
			name:           "VDOT 50 predicting 10K",
			vdot:           50.0,
			targetDistance: Distance10K,
			wantSeconds:    2364, // 39:24
			tolerance:      120,  // ±2 min
		},
		{
			name:           "VDOT 50 predicting half marathon",
			vdot:           50.0,
			targetDistance: DistanceHalfMara,
			wantSeconds:    5100, // 1:25:00
			tolerance:      180,  // ±3 min
		},
		{
			name:           "VDOT 50 predicting marathon",
			vdot:           50.0,
			targetDistance: DistanceMarathon,
			wantSeconds:    10494, // 2:54:54
			tolerance:      300,   // ±5 min
		},
		{
			name:           "VDOT 40 predicting 5K",
			vdot:           40.0,
			targetDistance: Distance5K,
			wantSeconds:    1422, // 23:42
			tolerance:      60,
		},
		{
			name:           "VDOT 60 predicting marathon",
			vdot:           60.0,
			targetDistance: DistanceMarathon,
			wantSeconds:    8664, // 2:24:24
			tolerance:      300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PredictTime(tt.vdot, tt.targetDistance)
			if abs(got-tt.wantSeconds) > tt.tolerance {
				t.Errorf("PredictTime() = %v (%v), want %v (±%v)",
					got, formatDuration(got), tt.wantSeconds, tt.tolerance)
			}
		})
	}
}

func TestPredictTime_EdgeCases(t *testing.T) {
	// Zero VDOT should return 0
	if got := PredictTime(0, Distance5K); got != 0 {
		t.Errorf("PredictTime with zero VDOT = %v, want 0", got)
	}

	// Negative VDOT should return 0
	if got := PredictTime(-50, Distance5K); got != 0 {
		t.Errorf("PredictTime with negative VDOT = %v, want 0", got)
	}
}

func TestGetVDOTLabel(t *testing.T) {
	tests := []struct {
		vdot      float64
		wantLabel string
	}{
		{80, "Elite"},
		{75, "Elite"},
		{70, "Highly Competitive"},
		{65, "Highly Competitive"},
		{60, "Competitive"},
		{55, "Competitive"},
		{50, "Advanced Recreational"},
		{45, "Advanced Recreational"},
		{42, "Intermediate"},
		{38, "Intermediate"},
		{35, "Beginner"},
		{30, "Beginner"},
		{25, "Novice"},
	}

	for _, tt := range tests {
		t.Run(tt.wantLabel, func(t *testing.T) {
			got := GetVDOTLabel(tt.vdot)
			if got != tt.wantLabel {
				t.Errorf("GetVDOTLabel(%v) = %v, want %v", tt.vdot, got, tt.wantLabel)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that calculating VDOT from a time and predicting back gives similar time
	tests := []struct {
		distance float64
		duration int
	}{
		{Distance5K, 1200},        // 20:00 5K
		{Distance10K, 2400},       // 40:00 10K
		{DistanceHalfMara, 5400},  // 1:30:00 half
		{DistanceMarathon, 11400}, // 3:10:00 marathon
	}

	for _, tt := range tests {
		t.Run(formatDuration(tt.duration), func(t *testing.T) {
			vdot := CalculateVDOT(tt.distance, tt.duration)
			predicted := PredictTime(vdot, tt.distance)

			// Should be within 2% of original time
			tolerance := int(float64(tt.duration) * 0.02)
			if abs(predicted-tt.duration) > tolerance {
				t.Errorf("Round trip: original %v, VDOT %.1f, predicted %v (diff: %v)",
					formatDuration(tt.duration), vdot, formatDuration(predicted), abs(predicted-tt.duration))
			}
		})
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return formatTime(h, m, s)
	}
	return formatMinSec(m, s)
}

func formatTime(h, m, s int) string {
	return string(rune('0'+h)) + ":" + padZero(m) + ":" + padZero(s)
}

func formatMinSec(m, s int) string {
	return padZero(m) + ":" + padZero(s)
}

func padZero(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
