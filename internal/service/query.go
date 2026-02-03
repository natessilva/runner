package service

import (
	"runner/internal/config"
	"runner/internal/store"
)

// QueryService provides read-only queries for the TUI
type QueryService struct {
	store      *store.Store
	athleteCfg config.AthleteConfig
}

// NewQueryService creates a new query service with athlete config
func NewQueryService(store *store.Store, athleteCfg config.AthleteConfig) *QueryService {
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
