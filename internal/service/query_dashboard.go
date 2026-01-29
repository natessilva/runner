package service

import (
	"time"

	"runner/internal/analysis"
	"runner/internal/store"
)

// DashboardData contains all data needed for the dashboard
type DashboardData struct {
	// Current fitness
	CurrentEF       float64
	EFTrend         string // "+3%" or "-2%"
	CurrentFitness  float64 // CTL
	CurrentFatigue  float64 // ATL
	CurrentForm     float64 // TSB
	FormDescription string

	// This week
	WeekRunCount int
	WeekDistance float64 // miles
	WeekTime     int     // seconds
	WeekAvgEF    float64

	// Recent activities
	RecentActivities []ActivityWithMetrics

	// For charts
	EFHistory        []float64
	EFDates          []time.Time
	WeeklyMileage    []float64 // Last 12 weeks of mileage
	WeeklyAvgCadence []float64 // Last 12 weeks avg cadence
	WeeklyAvgHR      []float64 // Last 12 weeks avg HR
	WeeklyLabels     []string  // Week labels (e.g., "Jan 06")
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
