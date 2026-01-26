package strava

import "time"

// Activity represents a Strava activity from the API
type Activity struct {
	ID                 int64     `json:"id"`
	Athlete            Athlete   `json:"athlete"`
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	SportType          string    `json:"sport_type"`
	StartDate          time.Time `json:"start_date"`
	StartDateLocal     time.Time `json:"start_date_local"`
	Timezone           string    `json:"timezone"`
	Distance           float64   `json:"distance"`            // meters
	MovingTime         int       `json:"moving_time"`         // seconds
	ElapsedTime        int       `json:"elapsed_time"`        // seconds
	TotalElevationGain float64   `json:"total_elevation_gain"` // meters
	AverageSpeed       float64   `json:"average_speed"`       // m/s
	MaxSpeed           float64   `json:"max_speed"`           // m/s
	AverageHeartrate   float64   `json:"average_heartrate"`   // bpm
	MaxHeartrate       float64   `json:"max_heartrate"`       // bpm
	AverageCadence     float64   `json:"average_cadence"`     // rpm or spm
	SufferScore        int       `json:"suffer_score"`
	HasHeartrate       bool      `json:"has_heartrate"`
}

// Athlete represents a Strava athlete (minimal info in activity response)
type Athlete struct {
	ID int64 `json:"id"`
}

// Streams represents activity stream data from the API
// Strava returns streams keyed by type when key_by_type=true
type Streams struct {
	Time           *StreamData[int]       `json:"time"`
	LatLng         *StreamData[[2]float64] `json:"latlng"`
	Altitude       *StreamData[float64]   `json:"altitude"`
	VelocitySmooth *StreamData[float64]   `json:"velocity_smooth"`
	Heartrate      *StreamData[int]       `json:"heartrate"`
	Cadence        *StreamData[int]       `json:"cadence"`
	GradeSmooth    *StreamData[float64]   `json:"grade_smooth"`
	Distance       *StreamData[float64]   `json:"distance"`
}

// StreamData represents a single stream type
type StreamData[T any] struct {
	Data         []T    `json:"data"`
	SeriesType   string `json:"series_type"`
	OriginalSize int    `json:"original_size"`
	Resolution   string `json:"resolution"`
}

// Len returns the length of the stream, or 0 if nil
func (s *Streams) Len() int {
	if s == nil || s.Time == nil {
		return 0
	}
	return len(s.Time.Data)
}

// HasHeartrate returns true if heartrate data exists
func (s *Streams) HasHeartrate() bool {
	return s != nil && s.Heartrate != nil && len(s.Heartrate.Data) > 0
}
