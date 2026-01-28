package analysis

import (
	"math"
	"testing"

	"runner/internal/store"
)

func TestAerobicDecoupling(t *testing.T) {
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
			name: "insufficient data - less than 2 minutes",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100) // only 100 seconds
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			expected: 0,
			delta:    0,
		},
		{
			name: "no decoupling - consistent efficiency",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				for i := 0; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			expected: 0,
			delta:    0.1,
		},
		{
			name: "positive decoupling - second half less efficient",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				// First half: faster at same HR
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				// Second half: slower at same HR (fatigued)
				for i := 100; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 2.7, 150)
				}
				return streams
			}(),
			// First half EF = 3.0/150 = 0.02
			// Second half EF = 2.7/150 = 0.018
			// Decoupling = ((0.02/0.018) - 1) * 100 = 11.1%
			expected: 11.1,
			delta:    0.5,
		},
		{
			name: "positive decoupling - HR drift at same pace",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				// First half: normal HR
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				// Second half: elevated HR (cardiac drift)
				for i := 100; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 165)
				}
				return streams
			}(),
			// First half EF = 3.0/150 = 0.02
			// Second half EF = 3.0/165 = 0.0182
			// Decoupling = ((0.02/0.0182) - 1) * 100 = 10%
			expected: 10,
			delta:    0.5,
		},
		{
			name: "negative decoupling - negative split (second half better)",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				// First half: moderate
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				// Second half: faster at same HR (warmed up)
				for i := 100; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 3.3, 150)
				}
				return streams
			}(),
			// First half EF = 3.0/150 = 0.02
			// Second half EF = 3.3/150 = 0.022
			// Decoupling = ((0.02/0.022) - 1) * 100 = -9.1%
			expected: -9.1,
			delta:    0.5,
		},
		{
			name: "excellent aerobic base - minimal decoupling",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				// First half
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				// Second half: slight efficiency loss
				for i := 100; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 2.94, 150)
				}
				return streams
			}(),
			// First EF = 0.02, Second EF = 0.0196
			// Decoupling = ((0.02/0.0196) - 1) * 100 ≈ 2%
			expected: 2.0,
			delta:    0.5,
		},
		{
			name: "filters invalid data points",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				// Add some invalid points in second half
				for i := 100; i < 150; i++ {
					streams[i] = makeStreamPoint(i, 0.3, 150) // velocity too low
				}
				for i := 150; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			// Second half has only 50 valid points vs 100 in first
			// But averages should still be same
			expected: 0,
			delta:    0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AerobicDecoupling(tt.streams)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("AerobicDecoupling() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestCardiacDrift(t *testing.T) {
	tests := []struct {
		name     string
		streams  []store.StreamPoint
		avgPace  float64
		expected float64
		delta    float64
	}{
		{
			name:     "empty streams",
			streams:  []store.StreamPoint{},
			avgPace:  3.0,
			expected: 0,
			delta:    0,
		},
		{
			name: "insufficient data - less than 4 minutes",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				for i := 0; i < 200; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 0,
			delta:    0,
		},
		{
			name: "zero avg pace",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 300)
				for i := 0; i < 300; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			avgPace:  0,
			expected: 0,
			delta:    0,
		},
		{
			name: "no drift - constant HR",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 300)
				for i := 0; i < 300; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 0,
			delta:    0.5,
		},
		{
			name: "positive drift - HR increases over time",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 400)
				for i := 0; i < 400; i++ {
					// HR gradually increases from 150 to 170
					hr := 150.0 + (float64(i)/400.0)*20
					streams[i] = makeStreamPoint(i, 3.0, hr)
				}
				return streams
			}(),
			avgPace: 3.0,
			// First quarter avg HR ≈ 152.5, last quarter avg HR ≈ 167.5
			// Drift = 167.5 - 152.5 = 15
			expected: 15,
			delta:    2,
		},
		{
			name: "negative drift (unusual) - HR decreases",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 400)
				for i := 0; i < 400; i++ {
					// HR gradually decreases (warm-up effect)
					hr := 170.0 - (float64(i)/400.0)*20
					streams[i] = makeStreamPoint(i, 3.0, hr)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: -15,
			delta:    2,
		},
		{
			name: "filters non-steady state segments",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 400)
				for i := 0; i < 400; i++ {
					var vel float64
					if i%50 < 10 {
						vel = 4.0 // surge - outside 10% of avg pace
					} else {
						vel = 3.0 // steady
					}
					streams[i] = makeStreamPoint(i, vel, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 0,
			delta:    1,
		},
		{
			name: "insufficient steady-state data",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 300)
				for i := 0; i < 300; i++ {
					// All points outside steady-state range
					streams[i] = makeStreamPoint(i, 4.5, 150) // 50% faster than avg
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 0,
			delta:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CardiacDrift(tt.streams, tt.avgPace)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("CardiacDrift() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestSteadyStatePct(t *testing.T) {
	tests := []struct {
		name     string
		streams  []store.StreamPoint
		avgPace  float64
		expected float64
		delta    float64
	}{
		{
			name:     "empty streams",
			streams:  []store.StreamPoint{},
			avgPace:  3.0,
			expected: 0,
			delta:    0,
		},
		{
			name: "zero avg pace",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				makeStreamPoint(1, 3.0, 150),
			},
			avgPace:  0,
			expected: 0,
			delta:    0,
		},
		{
			name: "100% steady state - all at avg pace",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 100,
			delta:    0.1,
		},
		{
			name: "within ±10% is steady state",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := 0; i < 100; i++ {
					// Oscillate between 2.75 and 3.25 (within ±10% of 3.0)
					vel := 2.75 + float64(i%2)*0.5
					streams[i] = makeStreamPoint(i, vel, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 100, // 2.75 and 3.25 are both within 0.9-1.1 of 3.0
			delta:    0.1,
		},
		{
			name: "50% steady state",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := 0; i < 100; i++ {
					var vel float64
					if i < 50 {
						vel = 3.0 // steady
					} else {
						vel = 4.5 // 50% faster - not steady
					}
					streams[i] = makeStreamPoint(i, vel, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 50,
			delta:    0.1,
		},
		{
			name: "0% steady state - all variable",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := 0; i < 100; i++ {
					streams[i] = makeStreamPoint(i, 5.0, 150) // well outside 10% range
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 0,
			delta:    0.1,
		},
		{
			name: "handles nil velocity",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				{TimeOffset: 1, VelocitySmooth: nil, Heartrate: intPtr(150)},
				makeStreamPoint(2, 3.0, 150),
			},
			avgPace:  3.0,
			expected: 100, // only 2 valid points, both steady
			delta:    0.1,
		},
		{
			name: "interval workout - low steady state",
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := 0; i < 100; i++ {
					var vel float64
					// Simulate 4x intervals with recovery
					phase := (i / 25) % 2
					if phase == 0 {
						vel = 4.5 // fast interval
					} else {
						vel = 2.0 // slow recovery
					}
					streams[i] = makeStreamPoint(i, vel, 150)
				}
				return streams
			}(),
			avgPace:  3.0,
			expected: 0, // both paces are outside ±10%
			delta:    0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SteadyStatePct(tt.streams, tt.avgPace)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("SteadyStatePct() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestAverageHR(t *testing.T) {
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
			name: "single point",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
			},
			expected: 150,
			delta:    0.1,
		},
		{
			name: "multiple points - average",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 140),
				makeStreamPoint(1, 3.0, 150),
				makeStreamPoint(2, 3.0, 160),
			},
			expected: 150,
			delta:    0.1,
		},
		{
			name: "filters zero HR",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				{TimeOffset: 1, VelocitySmooth: floatPtr(3.0), Heartrate: intPtr(0)},
				makeStreamPoint(2, 3.0, 150),
			},
			expected: 150,
			delta:    0.1,
		},
		{
			name: "handles nil HR",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				{TimeOffset: 1, VelocitySmooth: floatPtr(3.0), Heartrate: nil},
				makeStreamPoint(2, 3.0, 150),
			},
			expected: 150,
			delta:    0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := averageHR(tt.streams)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("averageHR() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestCalculateHalfEF(t *testing.T) {
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
			name: "valid data",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				makeStreamPoint(1, 3.0, 150),
				makeStreamPoint(2, 3.0, 150),
			},
			// EF = avgVel / avgHR = 3.0 / 150 = 0.02
			expected: 0.02,
			delta:    0.001,
		},
		{
			name: "filters invalid points",
			streams: []store.StreamPoint{
				makeStreamPoint(0, 3.0, 150),
				makeStreamPoint(1, 0.3, 150), // velocity too low
				makeStreamPoint(2, 3.0, 150),
			},
			expected: 0.02,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateHalfEF(tt.streams)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("calculateHalfEF() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}
