package service

import (
	"fmt"
	"time"

	"runner/internal/analysis"
	"runner/internal/config"
	"runner/internal/store"
)

// QueryService provides read-only queries for the TUI
type QueryService struct {
	store      *store.DB
	athleteCfg config.AthleteConfig
}

// NewQueryService creates a new query service with athlete config
func NewQueryService(store *store.DB, athleteCfg config.AthleteConfig) *QueryService {
	// Apply defaults if not set
	if athleteCfg.MaxHR == 0 {
		athleteCfg.MaxHR = DefaultMaxHR
	}
	if athleteCfg.RestingHR == 0 {
		athleteCfg.RestingHR = 50
	}
	if athleteCfg.ThresholdHR == 0 {
		athleteCfg.ThresholdHR = 165
	}
	return &QueryService{store: store, athleteCfg: athleteCfg}
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
	ThresholdHR   int // Configured threshold HR (0 if using %maxHR zones)
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
		ConfiguredMax: int(q.athleteCfg.MaxHR),
		ThresholdHR:   int(q.athleteCfg.ThresholdHR),
	}
	if metrics != nil {
		detail.Activity.Metrics = *metrics
	}

	if len(streams) == 0 {
		return detail, nil
	}

	// Calculate splits, HR zones, and chart data from streams
	detail.calculateFromStreams(streams, activity.Distance, int(q.athleteCfg.MaxHR), int(q.athleteCfg.ThresholdHR))

	return detail, nil
}

func (d *ActivityDetail) calculateFromStreams(streams []store.StreamPoint, totalDistance float64, configuredMaxHR int, thresholdHR int) {
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
		d.HRZones = d.calculateHRZones(streams, configuredMaxHR, thresholdHR)
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

func (d *ActivityDetail) calculateHRZones(streams []store.StreamPoint, maxHR int, thresholdHR int) []HRZoneTime {
	// Use threshold-based zones if thresholdHR is set, otherwise use %maxHR zones
	var zones []HRZoneTime
	var thresholds []float64

	if thresholdHR > 0 {
		// Threshold-based zones (based on % of threshold HR)
		// Zone 1: <75% LTHR, Zone 2: 75-84% LTHR, Zone 3: 85-94% LTHR, Zone 4: 95-100% LTHR, Zone 5: >100% LTHR
		zones = []HRZoneTime{
			{Zone: 1, Name: "Warm Up (<75% LTHR)"},
			{Zone: 2, Name: "Easy (75-84% LTHR)"},
			{Zone: 3, Name: "Aerobic (85-94% LTHR)"},
			{Zone: 4, Name: "Threshold (95-100% LTHR)"},
			{Zone: 5, Name: "Maximum (>100% LTHR)"},
		}
		// Convert zone thresholds to actual HR values then to % of max for comparison
		// Zone boundaries match labels: Z2 75-84%, Z3 85-94%, Z4 95-100%
		// Using exclusive upper bounds so Z3 includes up to 94.99% and Z4 starts at 95%
		lthr := float64(thresholdHR)
		maxF := float64(maxHR)
		thresholds = []float64{
			(0.75 * lthr) / maxF, // Zone 1 upper bound: <75% LTHR
			(0.85 * lthr) / maxF, // Zone 2 upper bound: <85% LTHR
			(0.95 * lthr) / maxF, // Zone 3 upper bound: <95% LTHR
			lthr / maxF,          // Zone 4 upper bound: <=100% LTHR
			1.0,                  // Zone 5 upper bound: >100% LTHR
		}
	} else {
		// Traditional %maxHR zones
		zones = []HRZoneTime{
			{Zone: 1, Name: "Warm Up (<60%)"},
			{Zone: 2, Name: "Easy (60-70%)"},
			{Zone: 3, Name: "Aerobic (70-80%)"},
			{Zone: 4, Name: "Threshold (80-90%)"},
			{Zone: 5, Name: "Maximum (>90%)"},
		}
		thresholds = HRZoneThresholds
	}

	totalSeconds := 0

	for _, p := range streams {
		if p.Heartrate == nil || *p.Heartrate < MinValidHeartrate {
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
	AvgEF       float64
}

// ComparisonStats holds two periods and their deltas
type ComparisonStats struct {
	Label      string
	Current    PeriodStats
	Previous   PeriodStats
	DeltaRuns  int
	DeltaMiles float64
	DeltaHR    float64
	DeltaSPM   float64
	DeltaEF    float64
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

// GetWeeklyComparisons returns week-over-week and rolling 30-day comparisons
func (q *QueryService) GetWeeklyComparisons() ([]ComparisonStats, error) {
	now := time.Now()
	currentMonday := getMonday(now)
	lastMonday := currentMonday.AddDate(0, 0, -7)

	// This week vs last week
	thisWeek, err := q.getPeriodStatsForRange(currentMonday, now, "This Week")
	if err != nil {
		return nil, err
	}
	lastWeek, err := q.getPeriodStatsForRange(lastMonday, currentMonday, "Last Week")
	if err != nil {
		return nil, err
	}

	weekComparison := buildComparison("This Week vs Last Week", thisWeek, lastWeek)

	// Rolling 30-day comparison
	rolling30, err := q.getRolling30DayComparison()
	if err != nil {
		return nil, err
	}

	return []ComparisonStats{weekComparison, rolling30}, nil
}

// GetMonthlyComparisons returns month-over-month, year-over-year, and rolling 30-day comparisons
func (q *QueryService) GetMonthlyComparisons() ([]ComparisonStats, error) {
	now := time.Now()

	// This month vs last month
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)

	thisMonth, err := q.getPeriodStatsForRange(thisMonthStart, now, now.Format("Jan 2006"))
	if err != nil {
		return nil, err
	}
	lastMonth, err := q.getPeriodStatsForRange(lastMonthStart, thisMonthStart, lastMonthStart.Format("Jan 2006"))
	if err != nil {
		return nil, err
	}

	monthComparison := buildComparison("This Month vs Last Month", thisMonth, lastMonth)

	// Year over year (this month vs same month last year)
	lastYearStart := thisMonthStart.AddDate(-1, 0, 0)
	lastYearEnd := lastYearStart.AddDate(0, 1, 0)
	lastYearMonth, err := q.getPeriodStatsForRange(lastYearStart, lastYearEnd, lastYearStart.Format("Jan 2006"))
	if err != nil {
		return nil, err
	}

	yoyComparison := buildComparison("vs Same Month Last Year", thisMonth, lastYearMonth)

	// Rolling 30-day comparison
	rolling30, err := q.getRolling30DayComparison()
	if err != nil {
		return nil, err
	}

	return []ComparisonStats{monthComparison, yoyComparison, rolling30}, nil
}

// getRolling30DayComparison returns last 30 days vs prior 30 days
func (q *QueryService) getRolling30DayComparison() (ComparisonStats, error) {
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -Rolling30Days)
	sixtyDaysAgo := now.AddDate(0, 0, -Rolling30Days*2)

	current, err := q.getPeriodStatsForRange(thirtyDaysAgo, now, "Last 30 Days")
	if err != nil {
		return ComparisonStats{}, err
	}
	previous, err := q.getPeriodStatsForRange(sixtyDaysAgo, thirtyDaysAgo, "Prior 30 Days")
	if err != nil {
		return ComparisonStats{}, err
	}

	return buildComparison("Rolling 30 Days vs Prior 30", current, previous), nil
}

// getPeriodStatsForRange calculates stats for activities within a date range
func (q *QueryService) getPeriodStatsForRange(start, end time.Time, label string) (PeriodStats, error) {
	stats := PeriodStats{
		PeriodStart: start,
		PeriodLabel: label,
	}

	activities, metrics, err := q.store.GetActivitiesWithMetrics(PeriodStatsActivityLimit, 0)
	if err != nil {
		return stats, err
	}

	// Filter activities in range and collect IDs
	var relevantActivities []store.Activity
	var relevantMetrics []store.ActivityMetrics
	var activityIDs []int64

	for i, a := range activities {
		if !a.StartDate.Before(start) && a.StartDate.Before(end) {
			relevantActivities = append(relevantActivities, a)
			relevantMetrics = append(relevantMetrics, metrics[i])
			activityIDs = append(activityIDs, a.ID)
		}
	}

	if len(relevantActivities) == 0 {
		return stats, nil
	}

	// Batch fetch streams
	streamsMap, err := q.store.GetStreamsForActivities(activityIDs)
	if err != nil {
		streamsMap = make(map[int64][]store.StreamPoint)
	}

	// Aggregate stats
	var efSum float64
	var efCount int

	for i, a := range relevantActivities {
		stats.RunCount++
		stats.TotalMiles += metersToMiles(a.Distance)

		// EF from metrics
		if relevantMetrics[i].EfficiencyFactor != nil {
			efSum += *relevantMetrics[i].EfficiencyFactor
			efCount++
		}

		// HR and cadence from streams
		streams := streamsMap[a.ID]
		if len(streams) == 0 {
			continue
		}

		streamStats := AggregateStreamStats(streams)

		if streamStats.HRCount > 0 {
			activityAvgHR := streamStats.AvgHR()
			if stats.AvgHR == 0 {
				stats.AvgHR = activityAvgHR
			} else {
				n := float64(stats.RunCount)
				stats.AvgHR = stats.AvgHR*(n-1)/n + activityAvgHR/n
			}
		}
		if streamStats.CadenceCount > 0 {
			activityAvgSPM := streamStats.AvgCadence()
			if stats.AvgSPM == 0 {
				stats.AvgSPM = activityAvgSPM
			} else {
				n := float64(stats.RunCount)
				stats.AvgSPM = stats.AvgSPM*(n-1)/n + activityAvgSPM/n
			}
		}
	}

	if efCount > 0 {
		stats.AvgEF = efSum / float64(efCount)
	}

	return stats, nil
}

// buildComparison creates a ComparisonStats from two periods
func buildComparison(label string, current, previous PeriodStats) ComparisonStats {
	return ComparisonStats{
		Label:      label,
		Current:    current,
		Previous:   previous,
		DeltaRuns:  current.RunCount - previous.RunCount,
		DeltaMiles: current.TotalMiles - previous.TotalMiles,
		DeltaHR:    current.AvgHR - previous.AvgHR,
		DeltaSPM:   current.AvgSPM - previous.AvgSPM,
		DeltaEF:    current.AvgEF - previous.AvgEF,
	}
}

// PersonalRecordDisplay represents a formatted personal record for display
type PersonalRecordDisplay struct {
	Category       string
	CategoryLabel  string  // e.g., "5K", "1 Mile", "Best 400m"
	Time           string  // formatted duration "M:SS" or "H:MM:SS"
	Pace           string  // formatted pace "M:SS/mi"
	AvgHR          string  // formatted HR or "-"
	Date           string  // formatted date
	ActivityID     int64
	ActivityName   string
	IsEffort       bool    // true for best efforts, false for race distances
	DistanceMeters float64 // for display purposes
}

// PRsData contains all data needed for the PRs screen
type PRsData struct {
	RaceDistancePRs  []PersonalRecordDisplay
	BestEffortPRs    []PersonalRecordDisplay
	OtherPRs         []PersonalRecordDisplay
}

// GetPersonalRecords retrieves all personal records formatted for display
func (q *QueryService) GetPersonalRecords() (*PRsData, error) {
	records, err := q.store.GetAllPersonalRecords()
	if err != nil {
		return nil, err
	}

	// Get activity names for display
	activityNames := make(map[int64]string)
	for _, r := range records {
		if _, exists := activityNames[r.ActivityID]; !exists {
			if activity, err := q.store.GetActivity(r.ActivityID); err == nil {
				activityNames[r.ActivityID] = activity.Name
			}
		}
	}

	data := &PRsData{}

	for _, r := range records {
		display := PersonalRecordDisplay{
			Category:       r.Category,
			CategoryLabel:  formatCategoryLabel(r.Category),
			Time:           formatDuration(r.DurationSeconds),
			Date:           r.AchievedAt.Format("Jan 02, 2006"),
			ActivityID:     r.ActivityID,
			ActivityName:   activityNames[r.ActivityID],
			DistanceMeters: r.DistanceMeters,
		}

		if r.PacePerMile != nil {
			display.Pace = formatPace(int(*r.PacePerMile))
		} else {
			display.Pace = "-"
		}

		if r.AvgHeartrate != nil {
			display.AvgHR = fmt.Sprintf("%.0f", *r.AvgHeartrate)
		} else {
			display.AvgHR = "-"
		}

		// Categorize the record
		switch {
		case isRaceDistanceCategory(r.Category):
			data.RaceDistancePRs = append(data.RaceDistancePRs, display)
		case isEffortCategory(r.Category):
			display.IsEffort = true
			data.BestEffortPRs = append(data.BestEffortPRs, display)
		default:
			data.OtherPRs = append(data.OtherPRs, display)
		}
	}

	// Sort each category by distance
	sortPRsByDistance(data.RaceDistancePRs)
	sortPRsByDistance(data.BestEffortPRs)

	return data, nil
}

// GetActivityPRs retrieves personal records achieved during a specific activity
func (q *QueryService) GetActivityPRs(activityID int64) ([]PersonalRecordDisplay, error) {
	records, err := q.store.GetPersonalRecordsForActivity(activityID)
	if err != nil {
		return nil, err
	}

	var displays []PersonalRecordDisplay
	for _, r := range records {
		display := PersonalRecordDisplay{
			Category:       r.Category,
			CategoryLabel:  formatCategoryLabel(r.Category),
			Time:           formatDuration(r.DurationSeconds),
			Date:           r.AchievedAt.Format("Jan 02, 2006"),
			ActivityID:     r.ActivityID,
			DistanceMeters: r.DistanceMeters,
			IsEffort:       isEffortCategory(r.Category),
		}

		if r.PacePerMile != nil {
			display.Pace = formatPace(int(*r.PacePerMile))
		} else {
			display.Pace = "-"
		}

		if r.AvgHeartrate != nil {
			display.AvgHR = fmt.Sprintf("%.0f", *r.AvgHeartrate)
		} else {
			display.AvgHR = "-"
		}

		displays = append(displays, display)
	}

	return displays, nil
}

// formatCategoryLabel returns a human-readable label for a PR category
func formatCategoryLabel(category string) string {
	labels := map[string]string{
		"distance_1mi":     "1 Mile",
		"distance_5k":      "5K",
		"distance_10k":     "10K",
		"distance_half":    "Half Marathon",
		"distance_full":    "Marathon",
		"effort_400m":      "400m",
		"effort_1k":        "1K",
		"effort_1mi":       "1 Mile",
		"effort_5k":        "5K",
		"effort_10k":       "10K",
		"longest_run":      "Longest Run",
		"highest_elevation": "Most Elevation",
		"fastest_pace":     "Fastest Avg Pace",
	}

	if label, ok := labels[category]; ok {
		return label
	}
	return category
}

// isRaceDistanceCategory returns true if the category is a race distance PR
func isRaceDistanceCategory(category string) bool {
	return len(category) > 9 && category[:9] == "distance_"
}

// isEffortCategory returns true if the category is a best effort PR
func isEffortCategory(category string) bool {
	return len(category) > 7 && category[:7] == "effort_"
}

// sortPRsByDistance sorts PRs by their target distance (shortest first)
func sortPRsByDistance(prs []PersonalRecordDisplay) {
	// Define sort order
	order := map[string]int{
		"effort_400m":    1,
		"effort_1k":      2,
		"effort_1mi":     3,
		"distance_1mi":   3,
		"effort_5k":      4,
		"distance_5k":    4,
		"effort_10k":     5,
		"distance_10k":   5,
		"distance_half":  6,
		"distance_full":  7,
	}

	// Simple bubble sort for small slices
	for i := 0; i < len(prs); i++ {
		for j := i + 1; j < len(prs); j++ {
			orderI := order[prs[i].Category]
			orderJ := order[prs[j].Category]
			if orderI > orderJ {
				prs[i], prs[j] = prs[j], prs[i]
			}
		}
	}
}
