package service

const (
	// HR validation thresholds
	MinValidHeartrate = 50
	MaxValidHeartrate = 220
	DefaultMaxHR      = 185

	// Unit conversions
	MetersPerMile           = 1609.34
	StravaCadenceMultiplier = 2.0 // Strava reports single-leg cadence

	// Time windows
	EFCurrentPeriodDays = 7
	EFTrendCompareDays  = 28
	EFHistoryDays       = 90
	ChartWeeks          = 12

	// Pagination limits
	RecentActivitiesLimit     = 10
	HistoricalActivitiesLimit = 200
	PeriodStatsActivityLimit  = 500

	// Comparison windows
	Rolling30Days = 30

	// Partial mile threshold (0.1 miles in meters)
	PartialMileThreshold = 160

	// Minimum speed for pace calculation (m/s) - filters out stopped time
	MinSpeedForPace = 0.5

	// Seconds per minute for pace calculations
	SecondsPerMinute = 60
)

// HRZoneThresholds defines the upper bound percentage of max HR for each zone
var HRZoneThresholds = []float64{0.6, 0.7, 0.8, 0.9, 1.0}
