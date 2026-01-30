package store

import "time"

// Auth represents OAuth tokens for Strava API access
type Auth struct {
	AthleteID    int64     `db:"athlete_id"`
	AccessToken  string    `db:"access_token"`
	RefreshToken string    `db:"refresh_token"`
	ExpiresAt    time.Time `db:"expires_at"`
}

// Activity represents a Strava activity summary
type Activity struct {
	ID                 int64     `db:"id"`
	AthleteID          int64     `db:"athlete_id"`
	Name               string    `db:"name"`
	Type               string    `db:"type"`
	StartDate          time.Time `db:"start_date"`
	StartDateLocal     time.Time `db:"start_date_local"`
	Timezone           string    `db:"timezone"`
	Distance           float64   `db:"distance"`            // meters
	MovingTime         int       `db:"moving_time"`         // seconds
	ElapsedTime        int       `db:"elapsed_time"`        // seconds
	TotalElevationGain float64   `db:"total_elevation_gain"`
	AverageSpeed       float64   `db:"average_speed"`       // m/s
	MaxSpeed           float64   `db:"max_speed"`           // m/s
	AverageHeartrate   *float64  `db:"average_heartrate"`   // nullable
	MaxHeartrate       *float64  `db:"max_heartrate"`       // nullable
	AverageCadence     *float64  `db:"average_cadence"`     // nullable
	SufferScore        *int      `db:"suffer_score"`        // nullable
	HasHeartrate       bool      `db:"has_heartrate"`
	StreamsSynced      bool      `db:"streams_synced"`
}

// StreamPoint represents a single data point from activity streams
type StreamPoint struct {
	ActivityID     int64    `db:"activity_id"`
	TimeOffset     int      `db:"time_offset"`     // seconds
	Lat            *float64 `db:"latlng_lat"`
	Lng            *float64 `db:"latlng_lng"`
	Altitude       *float64 `db:"altitude"`        // meters
	VelocitySmooth *float64 `db:"velocity_smooth"` // m/s
	Heartrate      *int     `db:"heartrate"`       // bpm
	Cadence        *int     `db:"cadence"`         // spm
	GradeSmooth    *float64 `db:"grade_smooth"`    // percent
	Distance       *float64 `db:"distance"`        // cumulative meters
}

// ActivityMetrics represents computed fitness metrics for an activity
type ActivityMetrics struct {
	ActivityID        int64    `db:"activity_id"`
	EfficiencyFactor  *float64 `db:"efficiency_factor"`
	AerobicDecoupling *float64 `db:"aerobic_decoupling"`
	CardiacDrift      *float64 `db:"cardiac_drift"`
	PaceAtZ1          *float64 `db:"pace_at_z1"`
	PaceAtZ2          *float64 `db:"pace_at_z2"`
	PaceAtZ3          *float64 `db:"pace_at_z3"`
	TRIMP             *float64 `db:"trimp"`
	HRSS              *float64 `db:"hrss"`
	DataQualityScore  *float64 `db:"data_quality_score"`
	SteadyStatePct    *float64 `db:"steady_state_pct"`
}

// FitnessTrend represents daily aggregated fitness metrics
type FitnessTrend struct {
	Date                string   `db:"date"` // YYYY-MM-DD
	CTL                 *float64 `db:"ctl"`
	ATL                 *float64 `db:"atl"`
	TSB                 *float64 `db:"tsb"`
	EfficiencyFactor7d  *float64 `db:"efficiency_factor_7d"`
	EfficiencyFactor28d *float64 `db:"efficiency_factor_28d"`
	EfficiencyFactor90d *float64 `db:"efficiency_factor_90d"`
	RunCount7d          int      `db:"run_count_7d"`
	TotalDistance7d     float64  `db:"total_distance_7d"`
	TotalTime7d         int      `db:"total_time_7d"`
}

// PersonalRecord represents a personal best for a specific category
type PersonalRecord struct {
	ID              int64     `db:"id"`
	Category        string    `db:"category"`         // e.g., "distance_5k", "effort_1mi", "longest_run"
	ActivityID      int64     `db:"activity_id"`
	DistanceMeters  float64   `db:"distance_meters"`
	DurationSeconds int       `db:"duration_seconds"`
	PacePerMile     *float64  `db:"pace_per_mile"`    // seconds per mile
	AvgHeartrate    *float64  `db:"avg_heartrate"`
	AchievedAt      time.Time `db:"achieved_at"`
	StartOffset     *int      `db:"start_offset"`     // for best efforts: start time offset in stream
	EndOffset       *int      `db:"end_offset"`       // for best efforts: end time offset in stream
}

// RacePrediction represents a predicted race time
type RacePrediction struct {
	ID               int64     `db:"id"`
	TargetDistance   string    `db:"target_distance"`   // "5k", "10k", "half", "marathon"
	TargetMeters     float64   `db:"target_meters"`
	PredictedSeconds int       `db:"predicted_seconds"`
	PredictedPace    float64   `db:"predicted_pace"`    // seconds per mile
	VDOT             float64   `db:"vdot"`
	SourceCategory   string    `db:"source_category"`   // PR category used
	SourceActivityID int64     `db:"source_activity_id"`
	Confidence       string    `db:"confidence"`        // "high", "medium", "low"
	ConfidenceScore  float64   `db:"confidence_score"`
	ComputedAt       time.Time `db:"computed_at"`
}
