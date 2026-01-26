package analysis

import "strava-fitness/internal/store"

// AerobicDecoupling calculates the pace:HR drift between first and second half
// Returns percentage - positive means second half was less efficient
// < 5% on long runs indicates good aerobic base
func AerobicDecoupling(streams []store.StreamPoint) float64 {
	if len(streams) < 120 { // Need at least 2 minutes of data
		return 0
	}

	// Split into halves
	mid := len(streams) / 2
	firstHalf := streams[:mid]
	secondHalf := streams[mid:]

	firstEF := calculateHalfEF(firstHalf)
	secondEF := calculateHalfEF(secondHalf)

	if firstEF == 0 || secondEF == 0 {
		return 0
	}

	// Positive decoupling = second half less efficient (worse)
	// Formula: ((first / second) - 1) * 100
	decoupling := ((firstEF / secondEF) - 1) * 100

	return decoupling
}

// calculateHalfEF calculates efficiency factor for a portion of the run
func calculateHalfEF(streams []store.StreamPoint) float64 {
	var totalVelocity, totalHR float64
	var count int

	for _, p := range streams {
		if p.VelocitySmooth != nil && p.Heartrate != nil {
			vel := *p.VelocitySmooth
			hr := float64(*p.Heartrate)
			if vel > 0.5 && hr > 80 && hr < 220 {
				totalVelocity += vel
				totalHR += hr
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	return (totalVelocity / float64(count)) / (totalHR / float64(count))
}

// CardiacDrift measures HR increase during steady-state running
// Filters to segments where pace is relatively constant
// Returns the HR difference (bpm) between first and last quarter
func CardiacDrift(streams []store.StreamPoint, avgPace float64) float64 {
	if len(streams) < 240 || avgPace == 0 { // Need at least 4 minutes
		return 0
	}

	// Find steady-state segments (pace within 10% of average)
	var steadyStreams []store.StreamPoint
	for _, p := range streams {
		if p.VelocitySmooth == nil || p.Heartrate == nil {
			continue
		}

		paceRatio := *p.VelocitySmooth / avgPace
		if paceRatio > 0.9 && paceRatio < 1.1 {
			steadyStreams = append(steadyStreams, p)
		}
	}

	if len(steadyStreams) < 120 { // Need at least 2 minutes of steady data
		return 0
	}

	// Compare first quarter vs last quarter HR
	q1Len := len(steadyStreams) / 4
	firstQuarter := steadyStreams[:q1Len]
	lastQuarter := steadyStreams[len(steadyStreams)-q1Len:]

	firstHR := averageHR(firstQuarter)
	lastHR := averageHR(lastQuarter)

	if firstHR == 0 {
		return 0
	}

	// Return absolute HR drift
	return lastHR - firstHR
}

// averageHR calculates the average heart rate from stream points
func averageHR(streams []store.StreamPoint) float64 {
	var total float64
	var count int
	for _, p := range streams {
		if p.Heartrate != nil && *p.Heartrate > 0 {
			total += float64(*p.Heartrate)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// SteadyStatePct calculates what percentage of the run was at steady effort
// (pace within 10% of average)
func SteadyStatePct(streams []store.StreamPoint, avgPace float64) float64 {
	if len(streams) == 0 || avgPace == 0 {
		return 0
	}

	steadyCount := 0
	validCount := 0

	for _, p := range streams {
		if p.VelocitySmooth == nil {
			continue
		}
		validCount++

		paceRatio := *p.VelocitySmooth / avgPace
		if paceRatio > 0.9 && paceRatio < 1.1 {
			steadyCount++
		}
	}

	if validCount == 0 {
		return 0
	}

	return float64(steadyCount) / float64(validCount) * 100
}
