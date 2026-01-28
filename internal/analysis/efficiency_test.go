package analysis

import (
	"math"
	"testing"

	"strava-fitness/internal/store"
)

// Helper functions for creating test data
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func makeStreamPoint(time int, velocity, hr float64) store.StreamPoint {
	return store.StreamPoint{
		TimeOffset:     time,
		VelocitySmooth: floatPtr(velocity),
		Heartrate:      intPtr(int(hr)),
	}
}

func makeStreamPointWithGrade(time int, velocity, hr, grade float64) store.StreamPoint {
	return store.StreamPoint{
		TimeOffset:     time,
		VelocitySmooth: floatPtr(velocity),
		Heartrate:      intPtr(int(hr)),
		GradeSmooth:    floatPtr(grade),
	}
}

func TestEfficiencyFactor(t *testing.T) {
	tests := []struct {
		name     string
		streams  []store.StreamPoint
		expected float64
		delta    float64 // allowable tolerance
	}{
		{
			name:     "empty streams",
			streams:  []store.StreamPoint{},
			expected: 0,
			delta:    0,
		},
		{
			name: "no valid data points - velocity too low",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 0.3, 140), // velocity below 0.5 threshold
				makeStreamPoint(1, 0.4, 145),
			},
			expected: 0,
			delta:    0,
		},
		{
			name: "no valid data points - HR too low",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 70), // HR below 80 threshold
				makeStreamPoint(1, 3.0, 75),
			},
			expected: 0,
			delta:    0,
		},
		{
			name: "no valid data points - HR too high",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 225), // HR above 220 threshold
				makeStreamPoint(1, 3.0, 230),
			},
			expected: 0,
			delta:    0,
		},
		{
			name: "constant pace and HR",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150), // 3 m/s = 180 m/min
				makeStreamPoint(1, 3.0, 150),
				makeStreamPoint(2, 3.0, 150),
				makeStreamPoint(3, 3.0, 150),
			},
			// EF = (3.0 * 60) / 150 = 180 / 150 = 1.2
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "varying pace same HR",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 2.5, 150),
				makeStreamPoint(1, 3.0, 150),
				makeStreamPoint(2, 3.5, 150),
				makeStreamPoint(3, 3.0, 150),
			},
			// avg velocity = 3.0 m/s = 180 m/min
			// EF = 180 / 150 = 1.2
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "same pace varying HR",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 140),
				makeStreamPoint(1, 3.0, 150),
				makeStreamPoint(2, 3.0, 160),
				makeStreamPoint(3, 3.0, 150),
			},
			// avg HR = 150
			// EF = 180 / 150 = 1.2
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "filters out invalid points",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150), // valid
				makeStreamPoint(1, 0.3, 150), // velocity too low - filtered
				makeStreamPoint(2, 3.0, 70),  // HR too low - filtered
				makeStreamPoint(3, 3.0, 150), // valid
			},
			// Only 2 valid points, both at 3.0 m/s and 150 HR
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "handles nil velocity",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				{TimeOffset: 1, VelocitySmooth: nil, Heartrate: intPtr(150)},
				makeStreamPoint(2, 3.0, 150),
			},
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "handles nil heartrate",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				{TimeOffset: 1, VelocitySmooth: floatPtr(3.0), Heartrate: nil},
				makeStreamPoint(2, 3.0, 150),
			},
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "higher efficiency - faster runner",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 4.0, 150), // 4 m/s = 240 m/min
				makeStreamPoint(1, 4.0, 150),
				makeStreamPoint(2, 4.0, 150),
			},
			// EF = 240 / 150 = 1.6
			expected: 1.6,
			delta:    0.001,
		},
		{
			name: "lower efficiency - slower runner at same HR",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 2.0, 150), // 2 m/s = 120 m/min
				makeStreamPoint(1, 2.0, 150),
				makeStreamPoint(2, 2.0, 150),
			},
			// EF = 120 / 150 = 0.8
			expected: 0.8,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EfficiencyFactor(tt.streams)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("EfficiencyFactor() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestNormalizedEfficiencyFactor(t *testing.T) {
	tests := []struct {
		name     string
		streams  []store.StreamPoint
		expected float64
		delta    float64
	}{
		{
			name:     "empty streams",
			streams:  []store.StreamPoint{},
			expected: 0,
			delta:    0,
		},
		{
			name: "flat terrain - same as regular EF",
			streams: []store.StreamPoint{
				makeStreamPointWithGrade(0, 3.0, 150, 0),
				makeStreamPointWithGrade(1, 3.0, 150, 0),
				makeStreamPointWithGrade(2, 3.0, 150, 0),
			},
			// grade = 0, so gradeFactor = 1.0
			// NGP = 3.0 / 1.0 = 3.0
			// NEF = (3.0 * 60) / 150 = 1.2
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "uphill - lower normalized EF",
			streams: []store.StreamPoint{
				makeStreamPointWithGrade(0, 3.0, 150, 10), // 10% grade
				makeStreamPointWithGrade(1, 3.0, 150, 10),
				makeStreamPointWithGrade(2, 3.0, 150, 10),
			},
			// grade = 0.10, gradeFactor = 1 + (0.10 * 3.0) = 1.30
			// NGP = 3.0 / 1.30 = 2.31
			// NEF = (2.31 * 60) / 150 = 0.923
			expected: 0.923,
			delta:    0.01,
		},
		{
			name: "downhill - capped adjustment",
			streams: []store.StreamPoint{
				makeStreamPointWithGrade(0, 3.0, 150, -20), // -20% grade
				makeStreamPointWithGrade(1, 3.0, 150, -20),
				makeStreamPointWithGrade(2, 3.0, 150, -20),
			},
			// grade = -0.20, gradeFactor = 1 + (-0.20 * 3.0) = 0.4, capped to 0.5
			// NGP = 3.0 / 0.5 = 6.0
			// NEF = (6.0 * 60) / 150 = 2.4
			expected: 2.4,
			delta:    0.01,
		},
		{
			name: "very steep uphill - capped at 3.0",
			streams: []store.StreamPoint{
				makeStreamPointWithGrade(0, 3.0, 150, 100), // 100% grade (extremely steep)
				makeStreamPointWithGrade(1, 3.0, 150, 100),
			},
			// gradeFactor = 1 + (1.0 * 3.0) = 4.0, capped to 3.0
			// NGP = 3.0 / 3.0 = 1.0
			// NEF = (1.0 * 60) / 150 = 0.4
			expected: 0.4,
			delta:    0.01,
		},
		{
			name: "no grade data - treats as flat",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				makeStreamPoint(1, 3.0, 150),
				makeStreamPoint(2, 3.0, 150),
			},
			// No grade data means grade = 0, gradeFactor = 1.0
			expected: 1.2,
			delta:    0.001,
		},
		{
			name: "mixed terrain",
			streams: []store.StreamPoint{
				makeStreamPointWithGrade(0, 3.0, 150, 0),  // flat
				makeStreamPointWithGrade(1, 3.0, 150, 5),  // slight uphill
				makeStreamPointWithGrade(2, 3.0, 150, -5), // slight downhill
			},
			// Point 1: gradeFactor = 1.0, NGP = 3.0
			// Point 2: gradeFactor = 1.15, NGP = 2.61
			// Point 3: gradeFactor = 0.85, NGP = 3.53
			// avg NGP = 3.047
			// NEF = (3.047 * 60) / 150 = 1.22
			expected: 1.22,
			delta:    0.02,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizedEfficiencyFactor(tt.streams)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("NormalizedEfficiencyFactor() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestPaceAtHR(t *testing.T) {
	tests := []struct {
		name      string
		streams   []store.StreamPoint
		targetHR  float64
		tolerance float64
		expected  float64
		delta     float64
	}{
		{
			name:      "empty streams",
			streams:   []store.StreamPoint{},
			targetHR:  150,
			tolerance: 5,
			expected:  0,
			delta:     0,
		},
		{
			name: "insufficient data at target HR",
			streams: func() []store.StreamPoint {
				// Only 20 points at target HR (need 30)
				streams := make([]store.StreamPoint, 20)
				for i := 0; i < 20; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			targetHR:  150,
			tolerance: 5,
			expected:  0,
			delta:     0,
		},
		{
			name: "sufficient data at target HR",
			streams: func() []store.StreamPoint {
				// 40 points at target HR
				streams := make([]store.StreamPoint, 40)
				for i := 0; i < 40; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			targetHR:  150,
			tolerance: 5,
			// pace = (1000 / 3.0) / 60 = 5.56 min/km
			expected: 5.56,
			delta:    0.01,
		},
		{
			name: "within tolerance range",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 50)
				for i := 0; i < 50; i++ {
					// HR varies between 147 and 153 (within ±5 of 150)
					hr := 147.0 + float64(i%7)
					streams[i] = makeStreamPoint(i, 3.0, hr)
				}
				return streams
			}(),
			targetHR:  150,
			tolerance: 5,
			expected:  5.56,
			delta:     0.01,
		},
		{
			name: "outside tolerance range - no match",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 50)
				for i := 0; i < 50; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 170) // all at 170 HR
				}
				return streams
			}(),
			targetHR:  150,
			tolerance: 5,
			expected:  0,
			delta:     0,
		},
		{
			name: "filters low velocity",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 50)
				for i := 0; i < 50; i++ {
					vel := 0.3 // below 0.5 threshold
					streams[i] = makeStreamPoint(i, vel, 150)
				}
				return streams
			}(),
			targetHR:  150,
			tolerance: 5,
			expected:  0,
			delta:     0,
		},
		{
			name: "faster pace at lower HR",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 40)
				for i := 0; i < 40; i++ {
					streams[i] = makeStreamPoint(i, 4.0, 130) // fast pace, low HR
				}
				return streams
			}(),
			targetHR:  130,
			tolerance: 5,
			// pace = (1000 / 4.0) / 60 = 4.17 min/km
			expected: 4.17,
			delta:    0.01,
		},
		{
			name: "slower pace at higher HR",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 40)
				for i := 0; i < 40; i++ {
					streams[i] = makeStreamPoint(i, 2.5, 170) // slow pace, high HR
				}
				return streams
			}(),
			targetHR:  170,
			tolerance: 5,
			// pace = (1000 / 2.5) / 60 = 6.67 min/km
			expected: 6.67,
			delta:    0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PaceAtHR(tt.streams, tt.targetHR, tt.tolerance)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("PaceAtHR() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}
