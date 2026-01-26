package analysis

import (
	"math"
	"sort"
	"time"

	"strava-fitness/internal/store"
)

// HRZones represents athlete's heart rate zones
type HRZones struct {
	RestingHR float64
	MaxHR     float64
}

// DefaultZones returns sensible defaults if not configured
func DefaultZones() HRZones {
	return HRZones{
		RestingHR: 50,
		MaxHR:     185,
	}
}

// TRIMP calculates Training Impulse (Banister model)
// TRIMP = duration (min) * ΔHR ratio * e^(b * ΔHR ratio)
// where b = 1.92 for men, 1.67 for women (using male default)
func TRIMP(activity store.Activity, streams []store.StreamPoint, zones HRZones) float64 {
	duration := float64(activity.MovingTime) / 60.0 // Convert to minutes

	avgHR := averageHR(streams)
	if avgHR == 0 && activity.AverageHeartrate != nil {
		avgHR = *activity.AverageHeartrate
	}
	if avgHR == 0 {
		return 0
	}

	// Heart rate reserve ratio
	hrReserve := zones.MaxHR - zones.RestingHR
	if hrReserve <= 0 {
		return 0
	}

	hrRatio := (avgHR - zones.RestingHR) / hrReserve
	if hrRatio < 0 {
		hrRatio = 0
	}
	if hrRatio > 1 {
		hrRatio = 1
	}

	// Gender coefficient (using male default)
	b := 1.92

	return duration * hrRatio * math.Exp(b*hrRatio)
}

// HRSS calculates Heart Rate Stress Score
// Normalized to ~100 for a 1-hour threshold effort
func HRSS(activity store.Activity, streams []store.StreamPoint, zones HRZones) float64 {
	trimp := TRIMP(activity, streams, zones)

	// Threshold TRIMP for 1 hour at lactate threshold (~88% max HR)
	// Approximately 100 TRIMP for 1 hour at threshold
	thresholdTRIMP := 100.0

	return (trimp / thresholdTRIMP) * 100
}

// DailyLoad represents training load for a single day
type DailyLoad struct {
	Date  time.Time
	TRIMP float64
}

// FitnessMetrics represents CTL/ATL/TSB for a day
type FitnessMetrics struct {
	Date time.Time
	CTL  float64 // Chronic Training Load (42-day EMA) - "Fitness"
	ATL  float64 // Acute Training Load (7-day EMA) - "Fatigue"
	TSB  float64 // Training Stress Balance (CTL - ATL) - "Form"
}

// CalculateFitnessTrend computes CTL/ATL/TSB from daily loads
func CalculateFitnessTrend(dailyLoads []DailyLoad) []FitnessMetrics {
	if len(dailyLoads) == 0 {
		return nil
	}

	// Sort by date
	sort.Slice(dailyLoads, func(i, j int) bool {
		return dailyLoads[i].Date.Before(dailyLoads[j].Date)
	})

	// EMA decay constants
	ctlDecay := 2.0 / (42.0 + 1.0) // 42-day time constant
	atlDecay := 2.0 / (7.0 + 1.0)  // 7-day time constant

	var metrics []FitnessMetrics
	var ctl, atl float64

	// Fill in missing days with zero load
	startDate := dailyLoads[0].Date.Truncate(24 * time.Hour)
	endDate := dailyLoads[len(dailyLoads)-1].Date.Truncate(24 * time.Hour)

	// Create map of loads by date
	loadMap := make(map[string]float64)
	for _, dl := range dailyLoads {
		key := dl.Date.Format("2006-01-02")
		loadMap[key] += dl.TRIMP // Sum multiple activities on same day
	}

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		trimp := loadMap[key] // 0 if no activity

		// Exponential moving average
		ctl = ctl + ctlDecay*(trimp-ctl)
		atl = atl + atlDecay*(trimp-atl)
		tsb := ctl - atl

		metrics = append(metrics, FitnessMetrics{
			Date: d,
			CTL:  ctl,
			ATL:  atl,
			TSB:  tsb,
		})
	}

	return metrics
}

// GetCurrentFitness returns the most recent CTL/ATL/TSB values
func GetCurrentFitness(dailyLoads []DailyLoad) FitnessMetrics {
	metrics := CalculateFitnessTrend(dailyLoads)
	if len(metrics) == 0 {
		return FitnessMetrics{}
	}
	return metrics[len(metrics)-1]
}

// FormDescription returns a human-readable description of TSB
func FormDescription(tsb float64) string {
	switch {
	case tsb > 25:
		return "Very fresh (possibly detrained)"
	case tsb > 10:
		return "Fresh and ready to race"
	case tsb > 0:
		return "Neutral - good for training"
	case tsb > -10:
		return "Slightly fatigued"
	case tsb > -25:
		return "Tired but building fitness"
	default:
		return "Very fatigued - rest needed"
	}
}
