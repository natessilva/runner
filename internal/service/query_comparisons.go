package service

import (
	"time"

	"runner/internal/store"
)

// PeriodStats holds aggregated stats for a time period
type PeriodStats struct {
	PeriodStart     time.Time
	PeriodLabel     string
	RunCount        int
	TotalMiles      float64
	AvgHR           float64
	AvgSPM          float64
	AvgEF           float64
	TotalMovingTime int     // total moving seconds for pace calculation
	TotalDistance   float64 // total distance in meters for pace calculation
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

		// Accumulate moving time and distance for pace calculation
		stats[periodIdx].TotalMovingTime += streamStats.MovingTime
		stats[periodIdx].TotalDistance += streamStats.TotalDistance

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
