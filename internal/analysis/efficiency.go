package analysis

import "runner/internal/store"

// EfficiencyFactor calculates pace:HR efficiency
// Returns: (speed in m/s) / (average HR) * 100,000
// Higher is better - you're running faster for the same HR
// Typical values range from 1.0 to 2.0
func EfficiencyFactor(streams []store.StreamPoint) float64 {
	var totalVelocity, totalHR float64
	var count int

	for _, p := range streams {
		if p.VelocitySmooth != nil && p.Heartrate != nil {
			vel := *p.VelocitySmooth
			hr := float64(*p.Heartrate)
			// Filter noise: must be actually moving with reasonable HR
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

	avgVelocity := totalVelocity / float64(count) // m/s
	avgHR := totalHR / float64(count)

	// Convert to m/min and divide by HR for typical values 1.0-2.0
	// EF = (m/min) / HR
	avgVelocityMPM := avgVelocity * 60 // convert m/s to m/min
	return avgVelocityMPM / avgHR
}

// NormalizedEfficiencyFactor adjusts for elevation gain
// Uses grade-adjusted pace normalization
func NormalizedEfficiencyFactor(streams []store.StreamPoint) float64 {
	var totalNGP, totalHR float64
	var count int

	for _, p := range streams {
		if p.VelocitySmooth == nil || p.Heartrate == nil {
			continue
		}

		vel := *p.VelocitySmooth
		hr := float64(*p.Heartrate)

		if vel < 0.5 || hr < 80 || hr > 220 {
			continue
		}

		// Get grade if available, otherwise assume flat
		grade := 0.0
		if p.GradeSmooth != nil {
			grade = *p.GradeSmooth / 100.0 // Convert to decimal
		}

		// Normalize pace for grade
		// Approximate: +10% grade adds ~30s/km equivalent effort
		gradeFactor := 1.0 + (grade * 3.0)
		if gradeFactor < 0.5 {
			gradeFactor = 0.5 // Cap adjustment for steep descents
		}
		if gradeFactor > 3.0 {
			gradeFactor = 3.0 // Cap for very steep climbs
		}

		ngp := vel / gradeFactor
		totalNGP += ngp
		totalHR += hr
		count++
	}

	if count == 0 {
		return 0
	}

	avgNGP := totalNGP / float64(count)
	avgHR := totalHR / float64(count)

	// Convert to m/min and divide by HR for typical values 1.0-2.0
	avgNGPmpm := avgNGP * 60
	return avgNGPmpm / avgHR
}

// PaceAtHR calculates the average pace (min/km) at a target heart rate zone
// Returns 0 if insufficient data at that HR
func PaceAtHR(streams []store.StreamPoint, targetHR, tolerance float64) float64 {
	var totalPace float64
	var count int

	for _, p := range streams {
		if p.VelocitySmooth == nil || p.Heartrate == nil {
			continue
		}

		hr := float64(*p.Heartrate)
		vel := *p.VelocitySmooth

		// Check if HR is within tolerance of target
		if hr >= targetHR-tolerance && hr <= targetHR+tolerance && vel > 0.5 {
			// Convert m/s to min/km
			paceMinKm := (1000 / vel) / 60
			totalPace += paceMinKm
			count++
		}
	}

	if count < 30 { // Need at least 30 seconds of data
		return 0
	}

	return totalPace / float64(count)
}
