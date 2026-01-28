package analysis

import (
	"math"
	"testing"
	"time"

	"runner/internal/store"
)

func TestDefaultZones(t *testing.T) {
	zones := DefaultZones()

	if zones.RestingHR != 50 {
		t.Errorf("DefaultZones().RestingHR = %v, want 50", zones.RestingHR)
	}
	if zones.MaxHR != 185 {
		t.Errorf("DefaultZones().MaxHR = %v, want 185", zones.MaxHR)
	}
}

func TestTRIMP(t *testing.T) {
	defaultZones := DefaultZones()

	tests := []struct {
		name     string
		activity store.Activity
		streams  []store.StreamPoint
		zones    HRZones
		expected float64
		delta    float64
	}{
		{
			name: "empty streams - uses activity avg HR",
			activity: store.Activity{
				MovingTime:       3600, // 60 minutes
				AverageHeartrate: floatPtr(150),
			},
			streams: []store.StreamPoint{},
			zones:   defaultZones,
			// duration = 60 min
			// hrRatio = (150-50)/(185-50) = 100/135 = 0.741
			// b = 1.92
			// TRIMP = 60 * 0.741 * e^(1.92*0.741)
			expected: 184.3,
			delta:    1,
		},
		{
			name: "no HR data available",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: nil,
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 0,
			delta:    0,
		},
		{
			name: "uses stream HR over activity HR",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(170), // higher HR
			},
			streams: func() []store.StreamPoint {
				streams := make([]store.StreamPoint, 100)
				for i := range streams {
					streams[i] = makeStreamPoint(i, 3.0, 150) // lower HR from streams
				}
				return streams
			}(),
			zones: defaultZones,
			// Should use stream avg HR of 150, not activity HR of 170
			expected: 184.3,
			delta:    1,
		},
		{
			name: "zero HR reserve",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(150),
			},
			streams: []store.StreamPoint{},
			zones: HRZones{
				RestingHR: 100,
				MaxHR:     100, // same as resting = zero reserve
			},
			expected: 0,
			delta:    0,
		},
		{
			name: "negative HR reserve",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(150),
			},
			streams: []store.StreamPoint{},
			zones: HRZones{
				RestingHR: 100,
				MaxHR:     80, // less than resting
			},
			expected: 0,
			delta:    0,
		},
		{
			name: "HR below resting - clamped to 0",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(40), // below resting
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 0,
			delta:    0,
		},
		{
			name: "HR above max - clamped to 1",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(200), // above max
			},
			streams: []store.StreamPoint{},
			zones:   defaultZones,
			// hrRatio clamped to 1.0
			// TRIMP = 60 * 1.0 * e^(1.92*1.0) = 60 * 6.82 = 409
			expected: 409,
			delta:    2,
		},
		{
			name: "short easy run",
			activity: store.Activity{
				MovingTime:       1800, // 30 minutes
				AverageHeartrate: floatPtr(130),
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 55.5, // lower HR, shorter time = lower TRIMP
			delta:    2,
		},
		{
			name: "long hard run",
			activity: store.Activity{
				MovingTime:       7200, // 2 hours
				AverageHeartrate: floatPtr(165),
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 525, // higher HR, longer time = higher TRIMP
			delta:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TRIMP(tt.activity, tt.streams, tt.zones)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("TRIMP() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestHRSS(t *testing.T) {
	defaultZones := DefaultZones()

	tests := []struct {
		name     string
		activity store.Activity
		streams  []store.StreamPoint
		zones    HRZones
		expected float64
		delta    float64
	}{
		{
			name: "threshold effort for 1 hour = ~100 HRSS",
			activity: store.Activity{
				MovingTime:       3600, // 60 minutes
				AverageHeartrate: floatPtr(163), // ~88% of 185 max
			},
			streams: []store.StreamPoint{},
			zones:   defaultZones,
			// HRSS = (TRIMP / 100) * 100 = TRIMP
			// At 163 HR, TRIMP is ~250
			expected: 250,
			delta:    10,
		},
		{
			name: "easy effort = low HRSS",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(130),
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 111, // TRIMP at 130 HR for 1 hour
			delta:    10,
		},
		{
			name: "hard effort = high HRSS",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: floatPtr(175),
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 329, // TRIMP at 175 HR for 1 hour
			delta:    10,
		},
		{
			name: "no HR data",
			activity: store.Activity{
				MovingTime:       3600,
				AverageHeartrate: nil,
			},
			streams:  []store.StreamPoint{},
			zones:    defaultZones,
			expected: 0,
			delta:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HRSS(tt.activity, tt.streams, tt.zones)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("HRSS() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestCalculateFitnessTrend(t *testing.T) {
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		dailyLoads []DailyLoad
		checkFn    func(t *testing.T, metrics []FitnessMetrics)
	}{
		{
			name:       "empty daily loads",
			dailyLoads: []DailyLoad{},
			checkFn: func(t *testing.T, metrics []FitnessMetrics) {
				if metrics != nil {
					t.Errorf("expected nil, got %v", metrics)
				}
			},
		},
		{
			name: "single day load",
			dailyLoads: []DailyLoad{
				{Date: baseDate, TRIMP: 100},
			},
			checkFn: func(t *testing.T, metrics []FitnessMetrics) {
				if len(metrics) != 1 {
					t.Fatalf("expected 1 metric, got %d", len(metrics))
				}
				// First day: CTL and ATL start at 0, then apply decay
				// CTL = 0 + 2/43 * (100-0) = 4.65
				// ATL = 0 + 2/8 * (100-0) = 25
				if math.Abs(metrics[0].CTL-4.65) > 0.5 {
					t.Errorf("CTL = %v, want ~4.65", metrics[0].CTL)
				}
				if math.Abs(metrics[0].ATL-25) > 0.5 {
					t.Errorf("ATL = %v, want ~25", metrics[0].ATL)
				}
				// TSB = CTL - ATL
				if math.Abs(metrics[0].TSB-(metrics[0].CTL-metrics[0].ATL)) > 0.01 {
					t.Errorf("TSB = %v, want CTL-ATL = %v", metrics[0].TSB, metrics[0].CTL-metrics[0].ATL)
				}
			},
		},
		{
			name: "consecutive daily loads - builds fitness",
			dailyLoads: func() []DailyLoad {
				loads := make([]DailyLoad, 14)
				for i := range loads {
					loads[i] = DailyLoad{
						Date:  baseDate.AddDate(0, 0, i),
						TRIMP: 100,
					}
				}
				return loads
			}(),
			checkFn: func(t *testing.T, metrics []FitnessMetrics) {
				if len(metrics) != 14 {
					t.Fatalf("expected 14 metrics, got %d", len(metrics))
				}
				// CTL should be increasing over time
				for i := 1; i < len(metrics); i++ {
					if metrics[i].CTL <= metrics[i-1].CTL {
						t.Errorf("CTL should increase: day %d CTL=%v, day %d CTL=%v",
							i-1, metrics[i-1].CTL, i, metrics[i].CTL)
					}
				}
				// ATL responds faster than CTL
				if metrics[6].ATL <= metrics[6].CTL {
					t.Errorf("After 7 days, ATL should be higher than CTL: ATL=%v, CTL=%v",
						metrics[6].ATL, metrics[6].CTL)
				}
			},
		},
		{
			name: "gap in training - fills missing days",
			dailyLoads: []DailyLoad{
				{Date: baseDate, TRIMP: 100},
				{Date: baseDate.AddDate(0, 0, 5), TRIMP: 100}, // 5 days later
			},
			checkFn: func(t *testing.T, metrics []FitnessMetrics) {
				if len(metrics) != 6 {
					t.Fatalf("expected 6 metrics (filling gaps), got %d", len(metrics))
				}
				// Check dates are consecutive
				for i := 0; i < len(metrics)-1; i++ {
					expected := baseDate.AddDate(0, 0, i)
					if !metrics[i].Date.Equal(expected) {
						t.Errorf("metric %d date = %v, want %v", i, metrics[i].Date, expected)
					}
				}
				// CTL should decay during rest days
				if metrics[4].CTL >= metrics[0].CTL {
					t.Errorf("CTL should decay during rest: day 0 CTL=%v, day 4 CTL=%v",
						metrics[0].CTL, metrics[4].CTL)
				}
			},
		},
		{
			name: "multiple activities same day - sums TRIMP",
			dailyLoads: []DailyLoad{
				{Date: baseDate, TRIMP: 50},
				{Date: baseDate, TRIMP: 50}, // same day
			},
			checkFn: func(t *testing.T, metrics []FitnessMetrics) {
				if len(metrics) != 1 {
					t.Fatalf("expected 1 metric, got %d", len(metrics))
				}
				// Should be same as single 100 TRIMP load
				singleLoad := CalculateFitnessTrend([]DailyLoad{{Date: baseDate, TRIMP: 100}})
				if math.Abs(metrics[0].CTL-singleLoad[0].CTL) > 0.01 {
					t.Errorf("CTL with split loads = %v, want %v", metrics[0].CTL, singleLoad[0].CTL)
				}
			},
		},
		{
			name: "unsorted input - should still work",
			dailyLoads: []DailyLoad{
				{Date: baseDate.AddDate(0, 0, 2), TRIMP: 100},
				{Date: baseDate, TRIMP: 100},
				{Date: baseDate.AddDate(0, 0, 1), TRIMP: 100},
			},
			checkFn: func(t *testing.T, metrics []FitnessMetrics) {
				if len(metrics) != 3 {
					t.Fatalf("expected 3 metrics, got %d", len(metrics))
				}
				// Should be sorted by date
				if !metrics[0].Date.Before(metrics[1].Date) {
					t.Error("metrics should be sorted by date")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateFitnessTrend(tt.dailyLoads)
			tt.checkFn(t, result)
		})
	}
}

func TestGetCurrentFitness(t *testing.T) {
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		dailyLoads []DailyLoad
		checkFn    func(t *testing.T, metrics FitnessMetrics)
	}{
		{
			name:       "empty loads",
			dailyLoads: []DailyLoad{},
			checkFn: func(t *testing.T, metrics FitnessMetrics) {
				if metrics.CTL != 0 || metrics.ATL != 0 || metrics.TSB != 0 {
					t.Errorf("expected zero metrics, got CTL=%v, ATL=%v, TSB=%v",
						metrics.CTL, metrics.ATL, metrics.TSB)
				}
			},
		},
		{
			name: "returns most recent day",
			dailyLoads: []DailyLoad{
				{Date: baseDate, TRIMP: 100},
				{Date: baseDate.AddDate(0, 0, 1), TRIMP: 50},
				{Date: baseDate.AddDate(0, 0, 2), TRIMP: 200},
			},
			checkFn: func(t *testing.T, metrics FitnessMetrics) {
				expectedDate := baseDate.AddDate(0, 0, 2)
				if !metrics.Date.Equal(expectedDate) {
					t.Errorf("Date = %v, want %v", metrics.Date, expectedDate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCurrentFitness(tt.dailyLoads)
			tt.checkFn(t, result)
		})
	}
}

func TestFormDescription(t *testing.T) {
	tests := []struct {
		tsb      float64
		expected string
	}{
		{30, "Very fresh (possibly detrained)"},
		{25.1, "Very fresh (possibly detrained)"},
		{25, "Fresh and ready to race"},
		{15, "Fresh and ready to race"},
		{10.1, "Fresh and ready to race"},
		{10, "Neutral - good for training"},
		{5, "Neutral - good for training"},
		{0.1, "Neutral - good for training"},
		{0, "Slightly fatigued"},
		{-5, "Slightly fatigued"},
		{-9.9, "Slightly fatigued"},
		{-10, "Tired but building fitness"},
		{-15, "Tired but building fitness"},
		{-24.9, "Tired but building fitness"},
		{-25, "Very fatigued - rest needed"},
		{-30, "Very fatigued - rest needed"},
		{-50, "Very fatigued - rest needed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormDescription(tt.tsb)
			if result != tt.expected {
				t.Errorf("FormDescription(%v) = %q, want %q", tt.tsb, result, tt.expected)
			}
		})
	}
}
