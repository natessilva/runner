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
		maxHR = DefaultMaxHR
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
	recent, err := q.getRecentActivities()
	if err != nil {
		return nil, err
	}
	data.RecentActivities = recent

	// Calculate EF metrics from recent activities
	data.CurrentEF, data.EFTrend = q.calculateCurrentEF(recent)

	// Calculate this week's stats
	data.WeekRunCount, data.WeekDistance, data.WeekTime, data.WeekAvgEF = q.calculateWeekStats(recent)

	// Fitness metrics need more history
	allActivities, allMetrics, err := q.store.GetActivitiesWithMetrics(HistoricalActivitiesLimit, 0)
	if err != nil {
		// Log but don't fail - dashboard can show partial data
		allActivities = nil
		allMetrics = nil
	}

	if len(allActivities) > 0 {
		data.CurrentFitness, data.CurrentFatigue, data.CurrentForm, data.FormDescription = q.calculateFitnessMetrics(allActivities, allMetrics)
	}

	// Build EF history for chart
	data.EFHistory, data.EFDates = q.buildEFHistory(recent)

	// Build weekly charts
	data.WeeklyMileage, data.WeeklyAvgCadence, data.WeeklyAvgHR, data.WeeklyLabels = q.buildWeeklyCharts(allActivities)

	return data, nil
}

// getRecentActivities fetches and wraps recent activities with metrics
func (q *QueryService) getRecentActivities() ([]ActivityWithMetrics, error) {
	activities, metrics, err := q.store.GetActivitiesWithMetrics(RecentActivitiesLimit, 0)
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

// calculateCurrentEF calculates the 7-day EF average and trend vs 28-day average
func (q *QueryService) calculateCurrentEF(recent []ActivityWithMetrics) (currentEF float64, trend string) {
	if len(recent) == 0 {
		return 0, ""
	}

	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -EFCurrentPeriodDays)
	twentyEightDaysAgo := now.AddDate(0, 0, -EFTrendCompareDays)

	var efSum, ef28Sum float64
	var efCount, ef28Count int

	for _, am := range recent {
		if am.Metrics.EfficiencyFactor == nil {
			continue
		}
		ef := *am.Metrics.EfficiencyFactor

		if am.Activity.StartDate.After(sevenDaysAgo) {
			efSum += ef
			efCount++
		}
		if am.Activity.StartDate.After(twentyEightDaysAgo) {
			ef28Sum += ef
			ef28Count++
		}
	}

	if efCount > 0 {
		currentEF = efSum / float64(efCount)
	}

	if ef28Count > 0 && currentEF > 0 {
		ef28Avg := ef28Sum / float64(ef28Count)
		pctChange := ((currentEF - ef28Avg) / ef28Avg) * 100
		if pctChange > 0 {
			trend = "↑"
		} else if pctChange < 0 {
			trend = "↓"
		}
	}

	return currentEF, trend
}

// calculateWeekStats calculates stats for the current week (Monday start)
func (q *QueryService) calculateWeekStats(recent []ActivityWithMetrics) (runCount int, distance float64, totalTime int, avgEF float64) {
	weekStart := getMonday(time.Now())

	var efSum float64
	for _, am := range recent {
		if !am.Activity.StartDate.Before(weekStart) {
			runCount++
			distance += metersToMiles(am.Activity.Distance)
			totalTime += am.Activity.MovingTime
			if am.Metrics.EfficiencyFactor != nil {
				efSum += *am.Metrics.EfficiencyFactor
			}
		}
	}

	if runCount > 0 {
		avgEF = efSum / float64(runCount)
	}
	return
}

// calculateFitnessMetrics calculates CTL/ATL/TSB from TRIMP values
func (q *QueryService) calculateFitnessMetrics(activities []store.Activity, metrics []store.ActivityMetrics) (ctl, atl, tsb float64, formDesc string) {
	var dailyLoads []analysis.DailyLoad

	for i, a := range activities {
		if metrics[i].TRIMP != nil {
			dailyLoads = append(dailyLoads, analysis.DailyLoad{
				Date:  a.StartDate,
				TRIMP: *metrics[i].TRIMP,
			})
		}
	}

	if len(dailyLoads) > 0 {
		fitness := analysis.GetCurrentFitness(dailyLoads)
		return fitness.CTL, fitness.ATL, fitness.TSB, analysis.FormDescription(fitness.TSB)
	}
	return 0, 0, 0, ""
}

// buildEFHistory builds EF chart data for the last 90 days
func (q *QueryService) buildEFHistory(recent []ActivityWithMetrics) ([]float64, []time.Time) {
	ninetyDaysAgo := time.Now().AddDate(0, 0, -EFHistoryDays)

	var history []float64
	var dates []time.Time

	// Iterate in reverse to get oldest first (most recent last)
	for i := len(recent) - 1; i >= 0; i-- {
		am := recent[i]
		if am.Activity.StartDate.After(ninetyDaysAgo) && am.Metrics.EfficiencyFactor != nil {
			history = append(history, *am.Metrics.EfficiencyFactor)
			dates = append(dates, am.Activity.StartDate)
		}
	}
	return history, dates
}

// buildWeeklyCharts builds the 12-week mileage, cadence, and HR chart data
func (q *QueryService) buildWeeklyCharts(activities []store.Activity) (mileage, avgCadence, avgHR []float64, labels []string) {
	numWeeks := ChartWeeks
	currentWeekStart := getMonday(time.Now())

	// Initialize weekly buckets
	mileage = make([]float64, numWeeks)
	cadenceSum := make([]float64, numWeeks)
	cadenceCount := make([]int, numWeeks)
	hrSum := make([]float64, numWeeks)
	hrCount := make([]int, numWeeks)
	labels = make([]string, numWeeks)

	// Build labels
	for i := 0; i < numWeeks; i++ {
		weekStart := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1-i))
		labels[i] = weekStart.Format("Jan 02")
	}

	if len(activities) == 0 {
		avgCadence = make([]float64, numWeeks)
		avgHR = make([]float64, numWeeks)
		return
	}

	// Filter activities within the 12-week window and collect IDs
	twelveWeeksAgo := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1))
	var relevantActivities []store.Activity
	var activityIDs []int64
	for _, a := range activities {
		if !a.StartDate.Before(twelveWeeksAgo) {
			relevantActivities = append(relevantActivities, a)
			activityIDs = append(activityIDs, a.ID)
		}
	}

	// Batch fetch all streams for relevant activities (fixes N+1 query)
	streamsMap, err := q.store.GetStreamsForActivities(activityIDs)
	if err != nil {
		streamsMap = make(map[int64][]store.StreamPoint)
	}

	// Aggregate stats per week
	for _, a := range relevantActivities {
		weekIdx := q.findWeekIndex(a.StartDate, currentWeekStart, numWeeks)
		if weekIdx < 0 {
			continue
		}

		mileage[weekIdx] += metersToMiles(a.Distance)

		streams := streamsMap[a.ID]
		if len(streams) == 0 {
			continue
		}

		stats := AggregateStreamStats(streams)
		hrSum[weekIdx] += stats.HRSum
		hrCount[weekIdx] += stats.HRCount
		cadenceSum[weekIdx] += stats.CadenceSum
		cadenceCount[weekIdx] += stats.CadenceCount
	}

	// Calculate averages
	avgCadence = make([]float64, numWeeks)
	avgHR = make([]float64, numWeeks)
	for i := 0; i < numWeeks; i++ {
		if cadenceCount[i] > 0 {
			avgCadence[i] = cadenceSum[i] / float64(cadenceCount[i])
		}
		if hrCount[i] > 0 {
			avgHR[i] = hrSum[i] / float64(hrCount[i])
		}
	}

	return
}

// findWeekIndex returns the index of the week bucket for the given date
func (q *QueryService) findWeekIndex(date time.Time, currentWeekStart time.Time, numWeeks int) int {
	for i := 0; i < numWeeks; i++ {
		weekStart := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1-i))
		weekEnd := weekStart.AddDate(0, 0, 7)
		if !date.Before(weekStart) && date.Before(weekEnd) {
			return i
		}
	}
	return -1
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
	currentMile := 1
	mileStartIdx := 0
	var lastDistance float64

	for i, p := range streams {
		if p.Distance == nil {
			continue
		}

		dist := *p.Distance
		mileThreshold := float64(currentMile) * MetersPerMile

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
	remainingDist := totalDistance - float64(currentMile-1)*MetersPerMile
	if remainingDist > PartialMileThreshold && mileStartIdx < len(streams)-1 {
		split := d.calculateSplit(streams, mileStartIdx, len(streams)-1, currentMile)
		// Adjust pace for partial mile
		if remainingDist > 0 {
			partialMiles := remainingDist / MetersPerMile
			split.Duration = int(float64(split.Duration) / partialMiles)
			split.Pace = formatPace(split.Duration)
		}
		d.Splits = append(d.Splits, split)
	}

	// HR zones (using 5-zone model based on configured max HR)
	// Also record observed max HR during this activity
	d.MaxHR = findMaxHeartrate(streams)

	// Use configured max HR for zone calculations (not the activity's max)
	if configuredMaxHR > 0 {
		d.HRZones = d.calculateHRZones(streams, configuredMaxHR)
	}

	// Calculate averages using helper
	stats := AggregateStreamStats(streams)
	d.AvgHR = stats.AvgHR()
	d.AvgCadence = stats.AvgCadence()

	// Build chart data (minute-by-minute aggregation)
	d.buildChartData(streams)
}

// findMaxHeartrate returns the highest heart rate in the stream
func findMaxHeartrate(streams []store.StreamPoint) int {
	maxHR := 0
	for _, p := range streams {
		if p.Heartrate != nil && *p.Heartrate > maxHR {
			maxHR = *p.Heartrate
		}
	}
	return maxHR
}

// buildChartData aggregates stream data into minute-by-minute chart arrays
func (d *ActivityDetail) buildChartData(streams []store.StreamPoint) {
	minuteData := make(map[int]struct {
		paceSum   float64
		paceCount int
		hrSum     float64
		hrCount   int
	})

	var prevDist float64
	var prevTime int
	for _, p := range streams {
		minute := p.TimeOffset / SecondsPerMinute

		// Pace calculation
		if p.Distance != nil && p.TimeOffset > prevTime {
			distDelta := *p.Distance - prevDist
			timeDelta := float64(p.TimeOffset - prevTime)
			if distDelta > 0 && timeDelta > 0 {
				speedMPS := distDelta / timeDelta
				if speedMPS > MinSpeedForPace {
					paceMinPerMile := (MetersPerMile / speedMPS) / SecondsPerMinute
					entry := minuteData[minute]
					entry.paceSum += paceMinPerMile
					entry.paceCount++
					minuteData[minute] = entry
				}
			}
			prevDist = *p.Distance
			prevTime = p.TimeOffset
		}

		// HR for chart (slightly different threshold than validation - just > 50)
		if p.Heartrate != nil && *p.Heartrate > MinValidHeartrate {
			entry := minuteData[minute]
			entry.hrSum += float64(*p.Heartrate)
			entry.hrCount++
			minuteData[minute] = entry
		}
	}

	// Find max minute
	maxMinute := 0
	for m := range minuteData {
		if m > maxMinute {
			maxMinute = m
		}
	}

	// Build chart arrays
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

	// Calculate averages for this split using the slice
	splitStreams := streams[startIdx : endIdx+1]
	stats := AggregateStreamStats(splitStreams)
	split.AvgHR = stats.AvgHR()
	split.AvgCad = stats.AvgCadence()

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

	totalSeconds := 0

	for _, p := range streams {
		if p.Heartrate == nil || *p.Heartrate < MinValidHeartrate {
			continue
		}

		pct := float64(*p.Heartrate) / float64(maxHR)
		totalSeconds++

		for i, thresh := range HRZoneThresholds {
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
	mins := seconds / SecondsPerMinute
	secs := seconds % SecondsPerMinute
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
	activities, _, err := q.store.GetActivitiesWithMetrics(PeriodStatsActivityLimit, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	stats := make([]PeriodStats, numPeriods)

	// Initialize periods
	currentMonday := getMonday(now)
	for i := 0; i < numPeriods; i++ {
		var periodStart time.Time
		var label string

		if periodType == "weekly" {
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

	// Collect activity IDs for batch stream fetch
	activityIDs := make([]int64, len(activities))
	for i, a := range activities {
		activityIDs[i] = a.ID
	}

	// Batch fetch all streams (fixes N+1 query)
	streamsMap, err := q.store.GetStreamsForActivities(activityIDs)
	if err != nil {
		streamsMap = make(map[int64][]store.StreamPoint)
	}

	// Aggregate activities into periods
	for _, a := range activities {
		periodIdx := q.findPeriodIndex(a.StartDate, stats, periodType)
		if periodIdx < 0 {
			continue
		}

		stats[periodIdx].RunCount++
		stats[periodIdx].TotalMiles += metersToMiles(a.Distance)

		streams := streamsMap[a.ID]
		if len(streams) == 0 {
			continue
		}

		streamStats := AggregateStreamStats(streams)

		// Weighted contribution to period average
		if streamStats.HRCount > 0 {
			activityAvgHR := streamStats.AvgHR()
			if stats[periodIdx].AvgHR == 0 {
				stats[periodIdx].AvgHR = activityAvgHR
			} else {
				n := float64(stats[periodIdx].RunCount)
				stats[periodIdx].AvgHR = stats[periodIdx].AvgHR*(n-1)/n + activityAvgHR/n
			}
		}
		if streamStats.CadenceCount > 0 {
			activityAvgSPM := streamStats.AvgCadence()
			if stats[periodIdx].AvgSPM == 0 {
				stats[periodIdx].AvgSPM = activityAvgSPM
			} else {
				n := float64(stats[periodIdx].RunCount)
				stats[periodIdx].AvgSPM = stats[periodIdx].AvgSPM*(n-1)/n + activityAvgSPM/n
			}
		}
	}

	return stats, nil
}

// findPeriodIndex returns the index of the period that contains the given date
func (q *QueryService) findPeriodIndex(date time.Time, stats []PeriodStats, periodType string) int {
	for i := range stats {
		var periodEnd time.Time
		if periodType == "weekly" {
			periodEnd = stats[i].PeriodStart.AddDate(0, 0, 7)
		} else {
			periodEnd = stats[i].PeriodStart.AddDate(0, 1, 0)
		}

		if !date.Before(stats[i].PeriodStart) && date.Before(periodEnd) {
			return i
		}
	}
	return -1
}
