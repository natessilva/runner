package service

import (
	"context"
	"fmt"
	"time"

	"runner/internal/analysis"
	"runner/internal/config"
	"runner/internal/store"
	"runner/internal/strava"
)

// SyncService orchestrates syncing data from Strava
type SyncService struct {
	client  *strava.Client
	store   *store.DB
	hrZones analysis.HRZones
}

// NewSyncService creates a new sync service with athlete config for HR calculations
func NewSyncService(client *strava.Client, store *store.DB, athleteCfg config.AthleteConfig) *SyncService {
	return &SyncService{
		client:  client,
		store:   store,
		hrZones: analysis.NewHRZones(athleteCfg.RestingHR, athleteCfg.MaxHR, athleteCfg.ThresholdHR),
	}
}

// SyncProgress reports progress during sync
type SyncProgress struct {
	Phase           string // "activities", "streams", "metrics"
	Total           int
	Completed       int
	CurrentActivity string
	Error           error
}

// SyncResult contains the results of a sync operation
type SyncResult struct {
	ActivitiesFetched int
	ActivitiesStored  int
	StreamsFetched    int
	MetricsComputed   int
	RunsWithHR        int
	Errors            []error
}

// SyncAll performs a full sync: activities -> streams
func (s *SyncService) SyncAll(ctx context.Context, progress chan<- SyncProgress) (*SyncResult, error) {
	if progress != nil {
		defer close(progress)
	}

	result := &SyncResult{}

	// Phase 1: Sync activity summaries
	if err := s.syncActivities(ctx, progress, result); err != nil {
		return result, fmt.Errorf("syncing activities: %w", err)
	}

	// Phase 2: Fetch streams for activities that need them
	if err := s.syncStreams(ctx, progress, result); err != nil {
		return result, fmt.Errorf("syncing streams: %w", err)
	}

	// Phase 3: Compute metrics for activities that need them
	if err := s.computeMetrics(ctx, progress, result); err != nil {
		return result, fmt.Errorf("computing metrics: %w", err)
	}

	return result, nil
}

// syncActivities fetches all activities from Strava and stores them
func (s *SyncService) syncActivities(ctx context.Context, progress chan<- SyncProgress, result *SyncResult) error {
	// Get last sync time
	lastSyncStr, _ := s.store.GetSyncState("last_activity_sync")
	var after time.Time
	if lastSyncStr != "" {
		after, _ = time.Parse(time.RFC3339, lastSyncStr)
	}

	if progress != nil {
		progress <- SyncProgress{Phase: "activities", Total: 0, Completed: 0}
	}

	page := 1
	perPage := 100

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		activities, err := s.client.GetActivities(ctx, after, page, perPage)
		if err != nil {
			return fmt.Errorf("fetching page %d: %w", page, err)
		}

		if len(activities) == 0 {
			break
		}

		result.ActivitiesFetched += len(activities)

		for _, a := range activities {
			// Only store runs with HR data
			if a.Type == "Run" && a.HasHeartrate {
				storeActivity := convertActivity(a)
				if err := s.store.UpsertActivity(storeActivity); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("storing activity %d: %w", a.ID, err))
					continue
				}
				result.ActivitiesStored++
				result.RunsWithHR++
			}
		}

		if progress != nil {
			progress <- SyncProgress{
				Phase:     "activities",
				Total:     result.ActivitiesFetched,
				Completed: result.ActivitiesStored,
			}
		}

		if len(activities) < perPage {
			break // Last page
		}

		page++
	}

	// Update last sync time
	s.store.SetSyncState("last_activity_sync", time.Now().Format(time.RFC3339))

	return nil
}

// syncStreams fetches detailed stream data for activities that need it
func (s *SyncService) syncStreams(ctx context.Context, progress chan<- SyncProgress, result *SyncResult) error {
	// Get activities that need streams (limit to batch size to respect rate limits)
	activities, err := s.store.GetActivitiesNeedingStreams(50)
	if err != nil {
		return fmt.Errorf("getting activities needing streams: %w", err)
	}

	if len(activities) == 0 {
		return nil
	}

	if progress != nil {
		progress <- SyncProgress{Phase: "streams", Total: len(activities), Completed: 0}
	}

	for i, activity := range activities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if progress != nil {
			progress <- SyncProgress{
				Phase:           "streams",
				Total:           len(activities),
				Completed:       i,
				CurrentActivity: activity.Name,
			}
		}

		streams, err := s.client.GetActivityStreams(ctx, activity.ID)
		if err != nil {
			// Log error but continue - some activities may not have streams
			result.Errors = append(result.Errors, fmt.Errorf("activity %d (%s): %w", activity.ID, activity.Name, err))
			continue
		}

		// Convert and store streams
		points := convertStreams(activity.ID, streams)
		if len(points) > 0 {
			if err := s.store.SaveStreams(activity.ID, points); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("saving streams for %d: %w", activity.ID, err))
				continue
			}
		}

		// Mark activity as having streams synced
		if err := s.store.MarkStreamsSynced(activity.ID); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("marking synced for %d: %w", activity.ID, err))
			continue
		}

		result.StreamsFetched++
	}

	if progress != nil {
		progress <- SyncProgress{
			Phase:     "streams",
			Total:     len(activities),
			Completed: len(activities),
		}
	}

	return nil
}

// computeMetrics calculates fitness metrics for activities that need them
func (s *SyncService) computeMetrics(ctx context.Context, progress chan<- SyncProgress, result *SyncResult) error {
	// Get activities that have streams but no metrics
	activities, err := s.store.GetActivitiesNeedingMetrics()
	if err != nil {
		return fmt.Errorf("getting activities needing metrics: %w", err)
	}

	if len(activities) == 0 {
		return nil
	}

	if progress != nil {
		progress <- SyncProgress{Phase: "metrics", Total: len(activities), Completed: 0}
	}

	zones := s.hrZones

	for i, activity := range activities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if progress != nil {
			progress <- SyncProgress{
				Phase:           "metrics",
				Total:           len(activities),
				Completed:       i,
				CurrentActivity: activity.Name,
			}
		}

		// Get streams for this activity
		streams, err := s.store.GetStreams(activity.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("getting streams for %d: %w", activity.ID, err))
			continue
		}

		if len(streams) == 0 {
			continue
		}

		// Compute metrics
		metrics := analysis.ComputeActivityMetrics(activity, streams, zones)

		// Save metrics
		if err := s.store.SaveActivityMetrics(&metrics); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("saving metrics for %d: %w", activity.ID, err))
			continue
		}

		result.MetricsComputed++
	}

	if progress != nil {
		progress <- SyncProgress{
			Phase:     "metrics",
			Total:     len(activities),
			Completed: len(activities),
		}
	}

	return nil
}

// RateLimitStatus returns the current rate limit status from the client
func (s *SyncService) RateLimitStatus() (shortRemaining, dailyRemaining int) {
	return s.client.RateLimitStatus()
}

// convertActivity converts a Strava API activity to a store activity
func convertActivity(a strava.Activity) *store.Activity {
	activity := &store.Activity{
		ID:                 a.ID,
		AthleteID:          a.Athlete.ID,
		Name:               a.Name,
		Type:               a.Type,
		StartDate:          a.StartDate,
		StartDateLocal:     a.StartDateLocal,
		Timezone:           a.Timezone,
		Distance:           a.Distance,
		MovingTime:         a.MovingTime,
		ElapsedTime:        a.ElapsedTime,
		TotalElevationGain: a.TotalElevationGain,
		AverageSpeed:       a.AverageSpeed,
		MaxSpeed:           a.MaxSpeed,
		HasHeartrate:       a.HasHeartrate,
		StreamsSynced:      false,
	}

	if a.AverageHeartrate > 0 {
		activity.AverageHeartrate = &a.AverageHeartrate
	}
	if a.MaxHeartrate > 0 {
		activity.MaxHeartrate = &a.MaxHeartrate
	}
	if a.AverageCadence > 0 {
		activity.AverageCadence = &a.AverageCadence
	}
	if a.SufferScore > 0 {
		activity.SufferScore = &a.SufferScore
	}

	return activity
}

// convertStreams converts Strava API streams to store stream points
func convertStreams(activityID int64, s *strava.Streams) []store.StreamPoint {
	if s == nil || s.Time == nil {
		return nil
	}

	length := len(s.Time.Data)
	points := make([]store.StreamPoint, length)

	for i := 0; i < length; i++ {
		p := store.StreamPoint{
			ActivityID: activityID,
			TimeOffset: s.Time.Data[i],
		}

		if s.LatLng != nil && i < len(s.LatLng.Data) {
			lat := s.LatLng.Data[i][0]
			lng := s.LatLng.Data[i][1]
			p.Lat = &lat
			p.Lng = &lng
		}

		if s.Altitude != nil && i < len(s.Altitude.Data) {
			alt := s.Altitude.Data[i]
			p.Altitude = &alt
		}

		if s.VelocitySmooth != nil && i < len(s.VelocitySmooth.Data) {
			vel := s.VelocitySmooth.Data[i]
			p.VelocitySmooth = &vel
		}

		if s.Heartrate != nil && i < len(s.Heartrate.Data) {
			hr := s.Heartrate.Data[i]
			p.Heartrate = &hr
		}

		if s.Cadence != nil && i < len(s.Cadence.Data) {
			cad := s.Cadence.Data[i]
			p.Cadence = &cad
		}

		if s.GradeSmooth != nil && i < len(s.GradeSmooth.Data) {
			grade := s.GradeSmooth.Data[i]
			p.GradeSmooth = &grade
		}

		if s.Distance != nil && i < len(s.Distance.Data) {
			dist := s.Distance.Data[i]
			p.Distance = &dist
		}

		points[i] = p
	}

	return points
}
