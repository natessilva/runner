package analysis

import "runner/internal/store"

// ComputeActivityMetrics calculates all metrics for a single activity
func ComputeActivityMetrics(activity store.Activity, streams []store.StreamPoint, zones HRZones) store.ActivityMetrics {
	metrics := store.ActivityMetrics{
		ActivityID: activity.ID,
	}

	if len(streams) == 0 {
		return metrics
	}

	// Efficiency Factor
	ef := EfficiencyFactor(streams)
	if ef > 0 {
		metrics.EfficiencyFactor = &ef
	}

	// Aerobic Decoupling
	decoupling := AerobicDecoupling(streams)
	if decoupling != 0 {
		metrics.AerobicDecoupling = &decoupling
	}

	// Cardiac Drift
	avgPace := activity.Distance / float64(activity.MovingTime) // m/s
	drift := CardiacDrift(streams, avgPace)
	if drift != 0 {
		metrics.CardiacDrift = &drift
	}

	// TRIMP and HRSS
	trimp := TRIMP(activity, streams, zones)
	if trimp > 0 {
		metrics.TRIMP = &trimp
	}

	hrss := HRSS(activity, streams, zones)
	if hrss > 0 {
		metrics.HRSS = &hrss
	}

	// Data Quality Score: % of stream points with HR data
	validPoints := 0
	for _, p := range streams {
		if p.Heartrate != nil && *p.Heartrate > 0 {
			validPoints++
		}
	}
	quality := float64(validPoints) / float64(len(streams))
	metrics.DataQualityScore = &quality

	// Steady State Percentage
	steadyPct := SteadyStatePct(streams, avgPace)
	if steadyPct > 0 {
		metrics.SteadyStatePct = &steadyPct
	}

	// Pace at HR Zones (using typical zone midpoints)
	// Z1: ~60% max HR, Z2: ~70% max HR, Z3: ~80% max HR
	z1HR := zones.RestingHR + (zones.MaxHR-zones.RestingHR)*0.6
	z2HR := zones.RestingHR + (zones.MaxHR-zones.RestingHR)*0.7
	z3HR := zones.RestingHR + (zones.MaxHR-zones.RestingHR)*0.8

	paceZ1 := PaceAtHR(streams, z1HR, 5)
	if paceZ1 > 0 {
		metrics.PaceAtZ1 = &paceZ1
	}

	paceZ2 := PaceAtHR(streams, z2HR, 5)
	if paceZ2 > 0 {
		metrics.PaceAtZ2 = &paceZ2
	}

	paceZ3 := PaceAtHR(streams, z3HR, 5)
	if paceZ3 > 0 {
		metrics.PaceAtZ3 = &paceZ3
	}

	return metrics
}

// DataQualityDescription returns a human-readable data quality assessment
func DataQualityDescription(score float64) string {
	switch {
	case score >= 0.95:
		return "Excellent"
	case score >= 0.85:
		return "Good"
	case score >= 0.70:
		return "Fair"
	case score >= 0.50:
		return "Poor"
	default:
		return "Very Poor"
	}
}

// DecouplingAssessment returns a human-readable decoupling assessment
func DecouplingAssessment(decoupling float64) string {
	switch {
	case decoupling < 3:
		return "Excellent aerobic base"
	case decoupling < 5:
		return "Good aerobic fitness"
	case decoupling < 8:
		return "Developing aerobic base"
	case decoupling < 12:
		return "Needs more easy miles"
	default:
		return "Aerobic system needs work"
	}
}
