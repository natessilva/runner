package service

import (
	"time"

	"strava-fitness/internal/analysis"
	"strava-fitness/internal/store"
)

// QueryService provides read-only queries for the TUI
type QueryService struct {
	store *store.DB
}

// NewQueryService creates a new query service
func NewQueryService(store *store.DB) *QueryService {
	return &QueryService{store: store}
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

	// Calculate this week's stats
	weekStart := time.Now().AddDate(0, 0, -7)
	for i, a := range activities {
		if a.StartDate.After(weekStart) {
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

	// Calculate CTL/ATL/TSB
	allActivities, allMetrics, err := q.store.GetActivitiesWithMetrics(200, 0)
	if err == nil && len(allActivities) > 0 {
		var dailyLoads []analysis.DailyLoad
		zones := analysis.DefaultZones()

		for i, a := range allActivities {
			if allMetrics[i].TRIMP != nil {
				dailyLoads = append(dailyLoads, analysis.DailyLoad{
					Date:  a.StartDate,
					TRIMP: *allMetrics[i].TRIMP,
				})
			} else {
				// Estimate TRIMP from activity data
				streams, _ := q.store.GetStreams(a.ID)
				trimp := analysis.TRIMP(a, streams, zones)
				if trimp > 0 {
					dailyLoads = append(dailyLoads, analysis.DailyLoad{
						Date:  a.StartDate,
						TRIMP: trimp,
					})
				}
			}
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
	now := time.Now()

	// Find the start of the current week (Sunday)
	weekday := int(now.Weekday())
	currentWeekStart := now.AddDate(0, 0, -weekday)
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

	// Aggregate stats per week from all activities
	for _, a := range allActivities {
		// Find which week bucket this activity belongs to
		for i := 0; i < numWeeks; i++ {
			weekStart := currentWeekStart.AddDate(0, 0, -7*(numWeeks-1-i))
			weekEnd := weekStart.AddDate(0, 0, 7)
			if !a.StartDate.Before(weekStart) && a.StartDate.Before(weekEnd) {
				weeklyMileage[i] += a.Distance / 1609.34 // Convert to miles
				if a.AverageCadence != nil && *a.AverageCadence > 0 {
					weeklyCadenceSum[i] += *a.AverageCadence * 2 // Strava reports single-leg, double for SPM
					weeklyCadenceCount[i]++
				}
				if a.AverageHeartrate != nil && *a.AverageHeartrate > 0 {
					weeklyHRSum[i] += *a.AverageHeartrate
					weeklyHRCount[i]++
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
