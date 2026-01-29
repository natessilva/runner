package analysis

import "runner/internal/store"

// BestEffort represents the fastest segment of a given distance within an activity
type BestEffort struct {
	DistanceMeters  float64
	DurationSeconds int
	StartOffset     int // time offset in stream where effort starts
	EndOffset       int // time offset in stream where effort ends
	AvgHeartrate    float64
}

// Standard effort distances in meters
const (
	Distance400m        = 400
	Distance1K          = 1000
	Distance1Mile       = 1609.34
	Distance5K          = 5000
	Distance10K         = 10000
	DistanceHalfMara    = 21097
	DistanceMarathon    = 42195
	DistanceTolerance   = 0.05 // 5% tolerance for race distance matching
	MinPointsForEffort  = 10   // minimum stream points needed
)

// EffortDistances defines the standard best effort distances to track
var EffortDistances = []float64{
	Distance400m,
	Distance1K,
	Distance1Mile,
	Distance5K,
	Distance10K,
}

// RaceDistances defines the standard race distances for whole-activity PRs
var RaceDistances = map[string]float64{
	"distance_1mi":   Distance1Mile,
	"distance_5k":    Distance5K,
	"distance_10k":   Distance10K,
	"distance_half":  DistanceHalfMara,
	"distance_full":  DistanceMarathon,
}

// EffortCategories maps distances to their category names
var EffortCategories = map[float64]string{
	Distance400m:  "effort_400m",
	Distance1K:    "effort_1k",
	Distance1Mile: "effort_1mi",
	Distance5K:    "effort_5k",
	Distance10K:   "effort_10k",
}

// FindBestEffort finds the fastest segment of targetDistance meters within the stream data.
// Uses a sliding window algorithm with O(n) complexity.
// Returns nil if the activity is shorter than targetDistance or has insufficient data.
func FindBestEffort(streams []store.StreamPoint, targetDistance float64) *BestEffort {
	if len(streams) < MinPointsForEffort {
		return nil
	}

	// Filter to points with valid distance data
	var points []distPoint
	for _, p := range streams {
		if p.Distance != nil {
			points = append(points, distPoint{
				distance:   *p.Distance,
				timeOffset: p.TimeOffset,
				heartrate:  p.Heartrate,
			})
		}
	}

	if len(points) < MinPointsForEffort {
		return nil
	}

	// Check if activity is long enough
	totalDistance := points[len(points)-1].distance - points[0].distance
	if totalDistance < targetDistance {
		return nil
	}

	// Sliding window to find fastest segment
	// We iterate through all possible starting points and find the minimum
	// duration needed to cover targetDistance from each start
	var bestEffort *BestEffort
	bestDuration := int(^uint(0) >> 1) // max int

	for left := 0; left < len(points); left++ {
		// Binary search or linear scan to find the right endpoint
		// that gives us at least targetDistance
		for right := left + 1; right < len(points); right++ {
			segmentDist := points[right].distance - points[left].distance

			if segmentDist >= targetDistance {
				// Found a valid segment
				duration := points[right].timeOffset - points[left].timeOffset
				if duration <= 0 {
					continue
				}

				// Check if this is the fastest
				if duration < bestDuration {
					bestDuration = duration

					// Calculate average HR for this segment
					avgHR := calculateSegmentAvgHR(points, left, right)

					bestEffort = &BestEffort{
						DistanceMeters:  segmentDist,
						DurationSeconds: duration,
						StartOffset:     points[left].timeOffset,
						EndOffset:       points[right].timeOffset,
						AvgHeartrate:    avgHR,
					}
				}
				// Found the first segment >= targetDistance from this left point
				// No need to check further right points (they would be longer in distance and time)
				break
			}
		}
	}

	return bestEffort
}

// distPoint is a helper struct for sliding window algorithm
type distPoint struct {
	distance   float64
	timeOffset int
	heartrate  *int
}

// calculateSegmentAvgHR calculates average HR for a segment of points
func calculateSegmentAvgHR(points []distPoint, left, right int) float64 {
	var hrSum float64
	var hrCount int

	for i := left; i <= right; i++ {
		if points[i].heartrate != nil && *points[i].heartrate > 50 {
			hrSum += float64(*points[i].heartrate)
			hrCount++
		}
	}

	if hrCount > 0 {
		return hrSum / float64(hrCount)
	}
	return 0
}

// MatchesRaceDistance checks if an activity's total distance matches a standard race distance
// within the tolerance (±5%)
func MatchesRaceDistance(activityDistance float64, raceDistance float64) bool {
	lowerBound := raceDistance * (1 - DistanceTolerance)
	upperBound := raceDistance * (1 + DistanceTolerance)
	return activityDistance >= lowerBound && activityDistance <= upperBound
}

// GetMatchingRaceCategory returns the race category if the activity matches a standard distance
func GetMatchingRaceCategory(activityDistance float64) (category string, distance float64, matches bool) {
	for cat, dist := range RaceDistances {
		if MatchesRaceDistance(activityDistance, dist) {
			return cat, dist, true
		}
	}
	return "", 0, false
}

// CalculatePacePerMile calculates pace in seconds per mile
func CalculatePacePerMile(distanceMeters float64, durationSeconds int) float64 {
	if distanceMeters <= 0 || durationSeconds <= 0 {
		return 0
	}
	miles := distanceMeters / Distance1Mile
	return float64(durationSeconds) / miles
}
