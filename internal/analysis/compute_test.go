package analysis

import (
	"math"
	"testing"

	"strava-fitness/internal/store"
)

func TestComputeActivityMetrics(t *testing.T) {
	defaultZones := DefaultZones()

	tests := []struct {
		name     string
		activity store.Activity
		streams  []store.StreamPoint
		zones    HRZones
		checkFn  func(t *testing.T, metrics store.ActivityMetrics)
	}{
		{
			name: "empty streams - minimal metrics",
			activity: store.Activity{
				ID:         123,
				Distance:   10000, // 10km
				MovingTime: 3600,  // 1 hour
			},
			streams: []store.StreamPoint{},
			zones:   defaultZones,
			checkFn: func(t *testing.T, metrics store.ActivityMetrics) {
				if metrics.ActivityID != 123 {
					t.Errorf("ActivityID = %v, want 123", metrics.ActivityID)
				}
				// No stream data means no computed metrics
				if metrics.EfficiencyFactor != nil {
					t.Error("EfficiencyFactor should be nil with no streams")
				}
			},
		},
		{
			name: "full metrics computation",
			activity: store.Activity{
				ID:         456,
				Distance:   10000,
				MovingTime: 3600,
			},
			streams: func() []store.StreamPoint {
				// Create 300 seconds of good data
				streams := make([]store.StreamPoint, 300)
				for i := range streams {
					streams[i] = store.StreamPoint{
						TimeOffset:     i,
						VelocitySmooth: floatPtr(3.0),  // 3 m/s
						Heartrate:      intPtr(150),
					}
				}
				return streams
			}(),
			zones: defaultZones,
			checkFn: func(t *testing.T, metrics store.ActivityMetrics) {
				// Efficiency Factor should be computed
				if metrics.EfficiencyFactor == nil {
					t.Fatal("EfficiencyFactor should not be nil")
				}
				// EF = (3.0 * 60) / 150 = 1.2
				if math.Abs(*metrics.EfficiencyFactor-1.2) > 0.01 {
					t.Errorf("EfficiencyFactor = %v, want ~1.2", *metrics.EfficiencyFactor)
				}

				// Aerobic Decoupling: with perfectly constant pace/HR, decoupling is 0
				// The code only stores non-zero values, so it should be nil
				if metrics.AerobicDecoupling != nil && math.Abs(*metrics.AerobicDecoupling) > 0.5 {
					t.Errorf("AerobicDecoupling = %v, want ~0 or nil", *metrics.AerobicDecoupling)
				}

				// TRIMP should be computed
				if metrics.TRIMP == nil {
					t.Fatal("TRIMP should not be nil")
				}
				if *metrics.TRIMP <= 0 {
					t.Errorf("TRIMP should be positive, got %v", *metrics.TRIMP)
				}

				// HRSS should be computed
				if metrics.HRSS == nil {
					t.Fatal("HRSS should not be nil")
				}
				if *metrics.HRSS <= 0 {
					t.Errorf("HRSS should be positive, got %v", *metrics.HRSS)
				}

				// Data Quality should be computed
				if metrics.DataQualityScore == nil {
					t.Fatal("DataQualityScore should not be nil")
				}
				// All points have HR data
				if *metrics.DataQualityScore != 1.0 {
					t.Errorf("DataQualityScore = %v, want 1.0", *metrics.DataQualityScore)
				}

				// Steady State Pct should be computed
				if metrics.SteadyStatePct == nil {
					t.Fatal("SteadyStatePct should not be nil")
				}
				// All points are at constant pace, so 100% steady
				if *metrics.SteadyStatePct != 100 {
					t.Errorf("SteadyStatePct = %v, want 100", *metrics.SteadyStatePct)
				}
			},
		},
		{
			name: "data quality with missing HR",
			activity: store.Activity{
				ID:         789,
				Distance:   5000,
				MovingTime: 1800,
			},
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := range streams {
					streams[i] = store.StreamPoint{
						TimeOffset:     i,
						VelocitySmooth: floatPtr(3.0),
					}
					// Only 50% have HR data
					if i%2 == 0 {
						streams[i].Heartrate = intPtr(150)
					}
				}
				return streams
			}(),
			zones: defaultZones,
			checkFn: func(t *testing.T, metrics store.ActivityMetrics) {
				if metrics.DataQualityScore == nil {
					t.Fatal("DataQualityScore should not be nil")
				}
				// 50% of points have HR
				if math.Abs(*metrics.DataQualityScore-0.5) > 0.01 {
					t.Errorf("DataQualityScore = %v, want 0.5", *metrics.DataQualityScore)
				}
			},
		},
		{
			name: "cardiac drift with insufficient data",
			activity: store.Activity{
				ID:         100,
				Distance:   1000,
				MovingTime: 200, // Only 200 seconds
			},
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 200)
				for i := range streams {
					streams[i] = makeStreamPoint(i, 3.0, 150)
				}
				return streams
			}(),
			zones: defaultZones,
			checkFn: func(t *testing.T, metrics store.ActivityMetrics) {
				// CardiacDrift needs 240+ seconds, so should be nil
				if metrics.CardiacDrift != nil {
					t.Errorf("CardiacDrift should be nil with insufficient data, got %v", *metrics.CardiacDrift)
				}
			},
		},
		{
			name: "pace at HR zones",
			activity: store.Activity{
				ID:         200,
				Distance:   10000,
				MovingTime: 3600,
			},
			streams: func() []store.StreamPoint {
				// Create data at different HR zones
				streams := make([]store.StreamPoint, 150)
				for i := range streams {
					var hr float64
					// Z1: 60% of max HR = 0.6 * 185 = 111 (but need 50 + 0.6*135 = 131)
					// Actually: z1HR = 50 + (185-50)*0.6 = 50 + 81 = 131
					// z2HR = 50 + 135*0.7 = 144.5
					// z3HR = 50 + 135*0.8 = 158
					if i < 50 {
						hr = 131 // Z1
					} else if i < 100 {
						hr = 144 // Z2
					} else {
						hr = 158 // Z3
					}
					streams[i] = makeStreamPoint(i, 3.0, hr)
				}
				return streams
			}(),
			zones: defaultZones,
			checkFn: func(t *testing.T, metrics store.ActivityMetrics) {
				// Should have pace data for at least some zones
				// (depends on having 30+ seconds at each zone)
				if metrics.PaceAtZ1 == nil {
					t.Error("PaceAtZ1 should not be nil")
				}
				if metrics.PaceAtZ2 == nil {
					t.Error("PaceAtZ2 should not be nil")
				}
				if metrics.PaceAtZ3 == nil {
					t.Error("PaceAtZ3 should not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeActivityMetrics(tt.activity, tt.streams, tt.zones)
			tt.checkFn(t, result)
		})
	}
}

func TestDataQualityDescription(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{1.0, "Excellent"},
		{0.95, "Excellent"},
		{0.94, "Good"},
		{0.90, "Good"},
		{0.85, "Good"},
		{0.84, "Fair"},
		{0.75, "Fair"},
		{0.70, "Fair"},
		{0.69, "Poor"},
		{0.60, "Poor"},
		{0.50, "Poor"},
		{0.49, "Very Poor"},
		{0.30, "Very Poor"},
		{0.0, "Very Poor"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := DataQualityDescription(tt.score)
			if result != tt.expected {
				t.Errorf("DataQualityDescription(%v) = %q, want %q", tt.score, result, tt.expected)
			}
		})
	}
}

func TestDecouplingAssessment(t *testing.T) {
	tests := []struct {
		decoupling float64
		expected   string
	}{
		{0, "Excellent aerobic base"},
		{2.9, "Excellent aerobic base"},
		{3, "Good aerobic fitness"},
		{4.9, "Good aerobic fitness"},
		{5, "Developing aerobic base"},
		{7.9, "Developing aerobic base"},
		{8, "Needs more easy miles"},
		{11.9, "Needs more easy miles"},
		{12, "Aerobic system needs work"},
		{15, "Aerobic system needs work"},
		{20, "Aerobic system needs work"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := DecouplingAssessment(tt.decoupling)
			if result != tt.expected {
				t.Errorf("DecouplingAssessment(%v) = %q, want %q", tt.decoupling, result, tt.expected)
			}
		})
	}
}
