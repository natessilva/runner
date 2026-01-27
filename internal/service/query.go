package service

import (
	"fmt"
	"time"

	"strava-fitness/internal/analysis"
	"strava-fitness/internal/store"
)

// QueryService provides read-only queries for the TUI
type QueryService struct {
	store *store.DB
	maxHR float64 // Configured max HR for zone calculations
}

// NewQueryService creates a new query service
func NewQueryService(store *store.DB, maxHR float64) *QueryService {
	if maxHR == 0 {
		maxHR = 185 // Default
	}
	return &QueryService{store: store, maxHR: maxHR}
}

// DashboardData contains all data needed for the dashboard
type DashboardData struct {
	// Current fitness
	CurrentEF        float64
	EFTrend          string // "+3%" or "-2%"
	CurrentFitness   float64 // CTL
	CurrentFatigue   float64 // ATL
	CurrentForm      float64 // TSB
	FormDescription  string

	// This week
	WeekRunCount     int
	WeekDistance     float64 // miles
	WeekTime         int     // seconds
	WeekAvgEF        float64

	// Recent activities
	RecentActivities []ActivityWithMetrics

	// For charts
	EFHistory        []float64
	EFDates          []time.Time
	WeeklyMileage    []float64  // Last 12 weeks of mileage
	WeeklyAvgCadence []float64  // Last 12 weeks avg cadence
	WeeklyAvgHR      []float64  // Last 12 weeks avg HR
	WeeklyLabels     []string   // Week labels (e.g., "Jan 06")
}

// ActivityWithMetrics combines activity and its metrics
type ActivityWithMetrics struct {
	Activity store.Activity
	Metrics  store.ActivityMetrics
}

// GetDashboardData fetches all data needed for the dashboard
func (q *QueryService) GetDashboardData() (*DashboardData, error) {
	data := &DashboardData{}

	// Get recent activities with metrics
	activities, metrics, err := q.store.GetActivitiesWithMetrics(10, 0)
	if err != nil {
		return nil, err
	}

	for i := range activities {
		data.RecentActivities = append(data.RecentActivities, ActivityWithMetrics{
			Activity: activities[i],
			Metrics:  metrics[i],
		})
	}

	// Calculate current EF (average of last 7 days)
	if len(metrics) > 0 {
		var efSum float64
		var efCount int
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)

		for i, m := range metrics {
			if activities[i].StartDate.After(sevenDaysAgo) && m.EfficiencyFactor != nil {
				efSum += *m.EfficiencyFactor
				efCount++
			}
		}
		if efCount > 0 {
			data.CurrentEF = efSum / float64(efCount)
		}

		// Calculate 28-day average for trend comparison
		var ef28Sum float64
		var ef28Count int
		twentyEightDaysAgo := time.Now().AddDate(0, 0, -28)

		for i, m := range metrics {
			if activities[i].StartDate.After(twentyEightDaysAgo) && m.EfficiencyFactor != nil {
				ef28Sum += *m.EfficiencyFactor
				ef28Count++
			}
		}
		if ef28Count > 0 && data.CurrentEF > 0 {
			ef28Avg := ef28Sum / float64(ef28Count)
			pctChange := ((data.CurrentEF - ef28Avg) / ef28Avg) * 100
			if pctChange > 0 {
				data.EFTrend = "↑"
			} else if pctChange < 0 {
				data.EFTrend = "↓"
			}
		}
	}

	// Calculate this week's stats (week starts Monday)
	now := time.Now()
	daysFromMonday := (int(now.Weekday()) + 6) % 7 // Monday = 0
	weekStart := now.AddDate(0, 0, -daysFromMonday)
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
	for i, a := range activities {
		if !a.StartDate.Before(weekStart) {
			data.WeekRunCount++
			data.WeekDistance += a.Distance / 1609.34 // Convert to miles
			data.WeekTime += a.MovingTime
			if metrics[i].EfficiencyFactor != nil {
				data.WeekAvgEF += *metrics[i].EfficiencyFactor
			}
		}
	}
	if data.WeekRunCount > 0 {
		data.WeekAvgEF /= float64(data.WeekRunCount)
	}

	// Calculate CTL/ATL/TSB using pre-computed TRIMP values
	allActivities, allMetrics, err := q.store.GetActivitiesWithMetrics(200, 0)
	if err == nil && len(allActivities) > 0 {
		var dailyLoads []analysis.DailyLoad

		for i, a := range allActivities {
			if allMetrics[i].TRIMP != nil {
				dailyLoads = append(dailyLoads, analysis.DailyLoad{
					Date:  a.StartDate,
					TRIMP: *allMetrics[i].TRIMP,
				})
			}
			// Skip activities without TRIMP - they'll get computed on next sync
		}

		if len(dailyLoads) > 0 {
			fitness := analysis.GetCurrentFitness(dailyLoads)
			data.CurrentFitness = fitness.CTL
			data.CurrentFatigue = fitness.ATL
			data.CurrentForm = fitness.TSB
			data.FormDescription = analysis.FormDescription(fitness.TSB)
		}
	}

	// Build EF history for chart (last 90 days, most recent last)
	ninetyDaysAgo := time.Now().AddDate(0, 0, -90)
	for i := len(activities) - 1; i >= 0; i-- {
		a := activities[i]
		m := metrics[i]
		if a.StartDate.After(ninetyDaysAgo) && m.EfficiencyFactor != nil {
			data.EFHistory = append(data.EFHistory, *m.EfficiencyFactor)
			data.EFDates = append(data.EFDates, a.StartDate)
		}
	}

	// Build weekly stats for charts (last 12 weeks)
	numWeeks := 12

	// Find the start of the current week (Monday) - reuse 'now' from above
	currentWeekStart := now.AddDate(0, 0, -daysFromMonday)
	currentWeekStart = time.Date(currentWeekStart.Year(), currentWeekStart.Month(), currentWeekStart.Day(), 0, 0, 0, 0, currentWeekStart.Location())

	// Initialize weekly buckets
	weeklyMileage := make([]float64, numWeeks)
	weeklyCadenceSum := make([]float64, numWeeks)
	weeklyCadenceCount := make([]int, numWeeks)
	weeklyHRSum := make([]float64, numWeeks)
	weeklyHRCount := make([]int, numWeeks)
	weeklyLabels := make([]string, numWeeks)

	for i := 0; i < numWeeks; i++ {
		weekStart := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1-i))
		weeklyLabels[i] = weekStart.Format("Jan 02")
	}

	// Aggregate stats per week from activities in the 12-week window
	twelveWeeksAgo := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1))
	for _, a := range allActivities {
		// Skip activities outside the 12-week window
		if a.StartDate.Before(twelveWeeksAgo) {
			continue
		}

		// Find which week bucket this activity belongs to
		for i := 0; i < numWeeks; i++ {
			weekStart := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1-i))
			weekEnd := weekStart.AddDate(0, 0, 7)
			if !a.StartDate.Before(weekStart) && a.StartDate.Before(weekEnd) {
				weeklyMileage[i] += a.Distance / 1609.34 // Convert to miles

				// Get stream data for HR and cadence calculations
				streams, err := q.store.GetStreams(a.ID)
				if err == nil && len(streams) > 0 {
					for _, p := range streams {
						if p.Heartrate != nil && *p.Heartrate > 50 && *p.Heartrate < 220 {
							weeklyHRSum[i] += float64(*p.Heartrate)
							weeklyHRCount[i]++
						}
						if p.Cadence != nil && *p.Cadence > 0 {
							weeklyCadenceSum[i] += float64(*p.Cadence) * 2 // Strava reports single-leg
							weeklyCadenceCount[i]++
						}
					}
				}
				break
			}
		}
	}

	// Calculate averages
	weeklyAvgCadence := make([]float64, numWeeks)
	weeklyAvgHR := make([]float64, numWeeks)
	for i := 0; i < numWeeks; i++ {
		if weeklyCadenceCount[i] > 0 {
			weeklyAvgCadence[i] = weeklyCadenceSum[i] / float64(weeklyCadenceCount[i])
		}
		if weeklyHRCount[i] > 0 {
			weeklyAvgHR[i] = weeklyHRSum[i] / float64(weeklyHRCount[i])
		}
	}

	data.WeeklyMileage = weeklyMileage
	data.WeeklyAvgCadence = weeklyAvgCadence
	data.WeeklyAvgHR = weeklyAvgHR
	data.WeeklyLabels = weeklyLabels

	return data, nil
}

// GetActivitiesList returns paginated activities with metrics
func (q *QueryService) GetActivitiesList(limit, offset int) ([]ActivityWithMetrics, error) {
	activities, metrics, err := q.store.GetActivitiesWithMetrics(limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]ActivityWithMetrics, len(activities))
	for i := range activities {
		result[i] = ActivityWithMetrics{
			Activity: activities[i],
			Metrics:  metrics[i],
		}
	}
	return result, nil
}

// GetActivityDetail returns detailed information about a single activity
func (q *QueryService) GetActivityDetail(id int64) (*ActivityWithMetrics, []store.StreamPoint, error) {
	activity, err := q.store.GetActivity(id)
	if err != nil {
		return nil, nil, err
	}

	metrics, err := q.store.GetActivityMetrics(id)
	if err != nil {
		return nil, nil, err
	}

	streams, err := q.store.GetStreams(id)
	if err != nil {
		return nil, nil, err
	}

	result := &ActivityWithMetrics{Activity: *activity}
	if metrics != nil {
		result.Metrics = *metrics
	}

	return result, streams, nil
}

// GetTotalActivityCount returns the total number of activities
func (q *QueryService) GetTotalActivityCount() (int, error) {
	return q.store.CountActivities()
}

// MileSplit represents stats for a single mile
type MileSplit struct {
	Mile     int
	Duration int     // seconds
	Pace     string  // "M:SS" format
	AvgHR    float64
	AvgCad   float64
}

// HRZoneTime represents time spent in an HR zone
type HRZoneTime struct {
	Zone    int
	Name    string
	Seconds int
	Percent float64
}

// ActivityDetail contains detailed info for a single activity
type ActivityDetail struct {
	Activity      ActivityWithMetrics
	Splits        []MileSplit
	HRZones       []HRZoneTime
	PaceData      []float64 // pace per minute for charting (min/mile)
	HRData        []float64 // HR per minute for charting
	TimeLabels    []string  // time labels for chart
	AvgHR         float64
	AvgCadence    float64
	MaxHR         int // Observed max HR during this activity
	ConfiguredMax int // Configured max HR used for zone calculations
}

// GetActivityDetailByID returns detailed analysis for a single activity
func (q *QueryService) GetActivityDetailByID(id int64) (*ActivityDetail, error) {
	activity, err := q.store.GetActivity(id)
	if err != nil {
		return nil, err
	}

	metrics, _ := q.store.GetActivityMetrics(id)
	streams, err := q.store.GetStreams(id)
	if err != nil {
		return nil, err
	}

	detail := &ActivityDetail{
		Activity: ActivityWithMetrics{
			Activity: *activity,
		},
		ConfiguredMax: int(q.maxHR),
	}
	if metrics != nil {
		detail.Activity.Metrics = *metrics
	}

	if len(streams) == 0 {
		return detail, nil
	}

	// Calculate splits, HR zones, and chart data from streams
	detail.calculateFromStreams(streams, activity.Distance, int(q.maxHR))

	return detail, nil
}

func (d *ActivityDetail) calculateFromStreams(streams []store.StreamPoint, totalDistance float64, configuredMaxHR int) {
	// Mile splits
	mileInMeters := 1609.34
	currentMile := 1
	mileStartIdx := 0
	var lastDistance float64

	for i, p := range streams {
		if p.Distance == nil {
			continue
		}

		dist := *p.Distance
		mileThreshold := float64(currentMile) * mileInMeters

		if dist >= mileThreshold && lastDistance < mileThreshold {
			// Completed a mile
			split := d.calculateSplit(streams, mileStartIdx, i, currentMile)
			d.Splits = append(d.Splits, split)
			currentMile++
			mileStartIdx = i
		}
		lastDistance = dist
	}

	// Add final partial mile if significant (> 0.1 mile)
	remainingDist := totalDistance - float64(currentMile-1)*mileInMeters
	if remainingDist > 160 && mileStartIdx < len(streams)-1 {
		split := d.calculateSplit(streams, mileStartIdx, len(streams)-1, currentMile)
		// Adjust pace for partial mile
		if remainingDist > 0 {
			partialMiles := remainingDist / mileInMeters
			split.Duration = int(float64(split.Duration) / partialMiles)
			split.Pace = formatPace(split.Duration)
		}
		d.Splits = append(d.Splits, split)
	}

	// HR zones (using 5-zone model based on configured max HR)
	// Also record observed max HR during this activity
	observedMaxHR := 0
	for _, p := range streams {
		if p.Heartrate != nil && *p.Heartrate > observedMaxHR {
			observedMaxHR = *p.Heartrate
		}
	}
	d.MaxHR = observedMaxHR

	// Use configured max HR for zone calculations (not the activity's max)
	if configuredMaxHR > 0 {
		d.HRZones = d.calculateHRZones(streams, configuredMaxHR)
	}

	// Calculate averages and chart data
	var hrSum, cadSum float64
	var hrCount, cadCount int

	// Sample data every 60 seconds for charts
	minuteData := make(map[int]struct {
		paceSum   float64
		paceCount int
		hrSum     float64
		hrCount   int
	})

	var prevDist float64
	var prevTime int
	for _, p := range streams {
		if p.Heartrate != nil && *p.Heartrate > 50 && *p.Heartrate < 220 {
			hrSum += float64(*p.Heartrate)
			hrCount++
		}
		if p.Cadence != nil && *p.Cadence > 0 {
			cadSum += float64(*p.Cadence) * 2
			cadCount++
		}

		// Chart data - group by minute
		minute := p.TimeOffset / 60
		if p.Distance != nil && p.TimeOffset > prevTime {
			distDelta := *p.Distance - prevDist
			timeDelta := float64(p.TimeOffset - prevTime)
			if distDelta > 0 && timeDelta > 0 {
				// Calculate pace in min/mile
				speedMPS := distDelta / timeDelta
				if speedMPS > 0.5 { // Filter out stopped time
					paceMinPerMile := (1609.34 / speedMPS) / 60
					entry := minuteData[minute]
					entry.paceSum += paceMinPerMile
					entry.paceCount++
					minuteData[minute] = entry
				}
			}
			prevDist = *p.Distance
			prevTime = p.TimeOffset
		}
		if p.Heartrate != nil && *p.Heartrate > 50 {
			entry := minuteData[minute]
			entry.hrSum += float64(*p.Heartrate)
			entry.hrCount++
			minuteData[minute] = entry
		}
	}

	if hrCount > 0 {
		d.AvgHR = hrSum / float64(hrCount)
	}
	if cadCount > 0 {
		d.AvgCadence = cadSum / float64(cadCount)
	}

	// Build chart arrays
	maxMinute := 0
	for m := range minuteData {
		if m > maxMinute {
			maxMinute = m
		}
	}

	for m := 0; m <= maxMinute; m++ {
		entry := minuteData[m]
		if entry.paceCount > 0 {
			d.PaceData = append(d.PaceData, entry.paceSum/float64(entry.paceCount))
		} else if len(d.PaceData) > 0 {
			d.PaceData = append(d.PaceData, d.PaceData[len(d.PaceData)-1]) // carry forward
		} else {
			d.PaceData = append(d.PaceData, 0)
		}

		if entry.hrCount > 0 {
			d.HRData = append(d.HRData, entry.hrSum/float64(entry.hrCount))
		} else if len(d.HRData) > 0 {
			d.HRData = append(d.HRData, d.HRData[len(d.HRData)-1])
		} else {
			d.HRData = append(d.HRData, 0)
		}

		d.TimeLabels = append(d.TimeLabels, formatMinutes(m))
	}
}

func (d *ActivityDetail) calculateSplit(streams []store.StreamPoint, startIdx, endIdx int, mile int) MileSplit {
	split := MileSplit{Mile: mile}

	if endIdx <= startIdx || endIdx >= len(streams) {
		return split
	}

	startTime := streams[startIdx].TimeOffset
	endTime := streams[endIdx].TimeOffset
	split.Duration = endTime - startTime
	split.Pace = formatPace(split.Duration)

	// Calculate averages for this split
	var hrSum, cadSum float64
	var hrCount, cadCount int

	for i := startIdx; i <= endIdx && i < len(streams); i++ {
		p := streams[i]
		if p.Heartrate != nil && *p.Heartrate > 50 {
			hrSum += float64(*p.Heartrate)
			hrCount++
		}
		if p.Cadence != nil && *p.Cadence > 0 {
			cadSum += float64(*p.Cadence) * 2
			cadCount++
		}
	}

	if hrCount > 0 {
		split.AvgHR = hrSum / float64(hrCount)
	}
	if cadCount > 0 {
		split.AvgCad = cadSum / float64(cadCount)
	}

	return split
}

func (d *ActivityDetail) calculateHRZones(streams []store.StreamPoint, maxHR int) []HRZoneTime {
	zones := []HRZoneTime{
		{Zone: 1, Name: "Recovery (<60%)"},
		{Zone: 2, Name: "Aerobic (60-70%)"},
		{Zone: 3, Name: "Tempo (70-80%)"},
		{Zone: 4, Name: "Threshold (80-90%)"},
		{Zone: 5, Name: "VO2max (>90%)"},
	}

	thresholds := []float64{0.6, 0.7, 0.8, 0.9, 1.0}
	totalSeconds := 0

	for _, p := range streams {
		if p.Heartrate == nil || *p.Heartrate < 50 {
			continue
		}

		pct := float64(*p.Heartrate) / float64(maxHR)
		totalSeconds++

		for i, thresh := range thresholds {
			if pct <= thresh {
				zones[i].Seconds++
				break
			}
		}
	}

	// Calculate percentages
	if totalSeconds > 0 {
		for i := range zones {
			zones[i].Percent = float64(zones[i].Seconds) / float64(totalSeconds) * 100
		}
	}

	return zones
}

func formatPace(seconds int) string {
	mins := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}

func formatMinutes(m int) string {
	return fmt.Sprintf("%d:00", m)
}

// PeriodStats holds aggregated stats for a time period
type PeriodStats struct {
	PeriodStart time.Time
	PeriodLabel string
	RunCount    int
	TotalMiles  float64
	AvgHR       float64
	AvgSPM      float64
}

// GetPeriodStats returns aggregated stats by week or month
func (q *QueryService) GetPeriodStats(periodType string, numPeriods int) ([]PeriodStats, error) {
	// Get all activities
	activities, _, err := q.store.GetActivitiesWithMetrics(500, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	stats := make([]PeriodStats, numPeriods)

	// Initialize periods
	for i := 0; i < numPeriods; i++ {
		var periodStart time.Time
		var label string

		if periodType == "weekly" {
			// Find Monday of current week
			daysFromMonday := (int(now.Weekday()) + 6) % 7 // Monday = 0
			currentMonday := now.AddDate(0, 0, -daysFromMonday)
			currentMonday = time.Date(currentMonday.Year(), currentMonday.Month(), currentMonday.Day(), 0, 0, 0, 0, currentMonday.Location())
			periodStart = currentMonday.AddDate(0, 0, -7*(numPeriods-1-i))
			label = periodStart.Format("Jan 02")
		} else {
			// Monthly - first of month
			currentFirst := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			periodStart = currentFirst.AddDate(0, -(numPeriods-1-i), 0)
			label = periodStart.Format("Jan 2006")
		}

		stats[i] = PeriodStats{
			PeriodStart: periodStart,
			PeriodLabel: label,
		}
	}

	// Aggregate activities into periods
	for _, a := range activities {
		// Find which period this activity belongs to
		for i := 0; i < numPeriods; i++ {
			var periodEnd time.Time
			if periodType == "weekly" {
				periodEnd = stats[i].PeriodStart.AddDate(0, 0, 7)
			} else {
				periodEnd = stats[i].PeriodStart.AddDate(0, 1, 0)
			}

			if !a.StartDate.Before(stats[i].PeriodStart) && a.StartDate.Before(periodEnd) {
				stats[i].RunCount++
				stats[i].TotalMiles += a.Distance / 1609.34

				// Get stream data for accurate HR and cadence
				streams, err := q.store.GetStreams(a.ID)
				if err == nil && len(streams) > 0 {
					var hrSum float64
					var hrCount int
					var cadSum float64
					var cadCount int

					for _, p := range streams {
						if p.Heartrate != nil && *p.Heartrate > 50 && *p.Heartrate < 220 {
							hrSum += float64(*p.Heartrate)
							hrCount++
						}
						if p.Cadence != nil && *p.Cadence > 0 {
							cadSum += float64(*p.Cadence) * 2
							cadCount++
						}
					}

					// Weighted contribution to period average
					if hrCount > 0 {
						activityAvgHR := hrSum / float64(hrCount)
						// Use running weighted average
						if stats[i].AvgHR == 0 {
							stats[i].AvgHR = activityAvgHR
						} else {
							n := float64(stats[i].RunCount)
							stats[i].AvgHR = stats[i].AvgHR*(n-1)/n + activityAvgHR/n
						}
					}
					if cadCount > 0 {
						activityAvgSPM := cadSum / float64(cadCount)
						if stats[i].AvgSPM == 0 {
							stats[i].AvgSPM = activityAvgSPM
						} else {
							n := float64(stats[i].RunCount)
							stats[i].AvgSPM = stats[i].AvgSPM*(n-1)/n + activityAvgSPM/n
						}
					}
				}
				break
			}
		}
	}

	return stats, nil
}
