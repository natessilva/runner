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
	PRsComputed       int
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

	// Phase 4: Compute personal records
	if err := s.computePersonalRecords(ctx, progress, result); err != nil {
		return result, fmt.Errorf("computing personal records: %w", err)
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

// computePersonalRecords analyzes activities for personal records
func (s *SyncService) computePersonalRecords(ctx context.Context, progress chan<- SyncProgress, result *SyncResult) error {
	// Get all activities with streams for PR analysis
	activities, err := s.store.ListActivities(500, 0)
	if err != nil {
		return fmt.Errorf("getting activities for PR analysis: %w", err)
	}

	if len(activities) == 0 {
		return nil
	}

	if progress != nil {
		progress <- SyncProgress{Phase: "personal_records", Total: len(activities), Completed: 0}
	}

	for i, activity := range activities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if progress != nil {
			progress <- SyncProgress{
				Phase:           "personal_records",
				Total:           len(activities),
				Completed:       i,
				CurrentActivity: activity.Name,
			}
		}

		// Skip activities without streams
		if !activity.StreamsSynced {
			continue
		}

		// Check if activity matches a race distance
		if category, _, matches := analysis.GetMatchingRaceCategory(activity.Distance); matches {
			pacePerMile := analysis.CalculatePacePerMile(activity.Distance, activity.MovingTime)
			pr := &store.PersonalRecord{
				Category:        category,
				ActivityID:      activity.ID,
				DistanceMeters:  activity.Distance,
				DurationSeconds: activity.MovingTime,
				PacePerMile:     &pacePerMile,
				AvgHeartrate:    activity.AverageHeartrate,
				AchievedAt:      activity.StartDate,
			}
			if updated, err := s.store.UpsertPersonalRecord(pr); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("saving distance PR for %d: %w", activity.ID, err))
			} else if updated {
				result.PRsComputed++
			}
		}

		// Check other achievements: longest run, highest elevation, fastest avg pace
		s.checkOtherAchievements(&activity, result)

		// Get streams for best effort analysis
		streams, err := s.store.GetStreams(activity.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("getting streams for PR analysis %d: %w", activity.ID, err))
			continue
		}

		if len(streams) == 0 {
			continue
		}

		// Find best efforts for each target distance
		for targetDist, category := range analysis.EffortCategories {
			effort := analysis.FindBestEffort(streams, targetDist)
			if effort == nil {
				continue
			}

			pacePerMile := analysis.CalculatePacePerMile(effort.DistanceMeters, effort.DurationSeconds)
			var avgHR *float64
			if effort.AvgHeartrate > 0 {
				avgHR = &effort.AvgHeartrate
			}
			startOffset := effort.StartOffset
			endOffset := effort.EndOffset

			pr := &store.PersonalRecord{
				Category:        category,
				ActivityID:      activity.ID,
				DistanceMeters:  effort.DistanceMeters,
				DurationSeconds: effort.DurationSeconds,
				PacePerMile:     &pacePerMile,
				AvgHeartrate:    avgHR,
				AchievedAt:      activity.StartDate,
				StartOffset:     &startOffset,
				EndOffset:       &endOffset,
			}
			if updated, err := s.store.UpsertPersonalRecord(pr); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("saving effort PR for %d: %w", activity.ID, err))
			} else if updated {
				result.PRsComputed++
			}
		}
	}

	if progress != nil {
		progress <- SyncProgress{
			Phase:     "personal_records",
			Total:     len(activities),
			Completed: len(activities),
		}
	}

	return nil
}

// checkOtherAchievements checks for longest run, highest elevation, fastest average pace
func (s *SyncService) checkOtherAchievements(activity *store.Activity, result *SyncResult) {
	// Longest run
	pacePerMile := analysis.CalculatePacePerMile(activity.Distance, activity.MovingTime)
	longestPR := &store.PersonalRecord{
		Category:        "longest_run",
		ActivityID:      activity.ID,
		DistanceMeters:  activity.Distance,
		DurationSeconds: activity.MovingTime,
		PacePerMile:     &pacePerMile,
		AvgHeartrate:    activity.AverageHeartrate,
		AchievedAt:      activity.StartDate,
	}

	// For longest run, we need special logic - compare by distance not duration
	existing, _ := s.store.GetPersonalRecordByCategory("longest_run")
	if existing == nil || activity.Distance > existing.DistanceMeters {
		// Force update by using a very fast duration to pass the comparison check
		// Actually we need to handle this differently - longest run compares distance, not time
		if _, err := s.store.Exec(`
			INSERT INTO personal_records (
				category, activity_id, distance_meters, duration_seconds,
				pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(category) DO UPDATE SET
				activity_id = excluded.activity_id,
				distance_meters = excluded.distance_meters,
				duration_seconds = excluded.duration_seconds,
				pace_per_mile = excluded.pace_per_mile,
				avg_heartrate = excluded.avg_heartrate,
				achieved_at = excluded.achieved_at
			WHERE excluded.distance_meters > personal_records.distance_meters
		`, longestPR.Category, longestPR.ActivityID, longestPR.DistanceMeters, longestPR.DurationSeconds,
			longestPR.PacePerMile, longestPR.AvgHeartrate, longestPR.AchievedAt.Format(time.RFC3339), nil, nil,
		); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("saving longest run PR: %w", err))
		}
	}

	// Highest elevation gain
	elevPR := &store.PersonalRecord{
		Category:        "highest_elevation",
		ActivityID:      activity.ID,
		DistanceMeters:  activity.TotalElevationGain, // Store elevation in distance field
		DurationSeconds: activity.MovingTime,
		PacePerMile:     &pacePerMile,
		AvgHeartrate:    activity.AverageHeartrate,
		AchievedAt:      activity.StartDate,
	}

	existingElev, _ := s.store.GetPersonalRecordByCategory("highest_elevation")
	if existingElev == nil || activity.TotalElevationGain > existingElev.DistanceMeters {
		if _, err := s.store.Exec(`
			INSERT INTO personal_records (
				category, activity_id, distance_meters, duration_seconds,
				pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(category) DO UPDATE SET
				activity_id = excluded.activity_id,
				distance_meters = excluded.distance_meters,
				duration_seconds = excluded.duration_seconds,
				pace_per_mile = excluded.pace_per_mile,
				avg_heartrate = excluded.avg_heartrate,
				achieved_at = excluded.achieved_at
			WHERE excluded.distance_meters > personal_records.distance_meters
		`, elevPR.Category, elevPR.ActivityID, elevPR.DistanceMeters, elevPR.DurationSeconds,
			elevPR.PacePerMile, elevPR.AvgHeartrate, elevPR.AchievedAt.Format(time.RFC3339), nil, nil,
		); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("saving highest elevation PR: %w", err))
		}
	}

	// Fastest average pace (for runs > 1 mile)
	if activity.Distance >= analysis.Distance1Mile {
		existingPace, _ := s.store.GetPersonalRecordByCategory("fastest_pace")
		if existingPace == nil || (existingPace.PacePerMile != nil && pacePerMile < *existingPace.PacePerMile) {
			pacePR := &store.PersonalRecord{
				Category:        "fastest_pace",
				ActivityID:      activity.ID,
				DistanceMeters:  activity.Distance,
				DurationSeconds: activity.MovingTime,
				PacePerMile:     &pacePerMile,
				AvgHeartrate:    activity.AverageHeartrate,
				AchievedAt:      activity.StartDate,
			}
			if _, err := s.store.Exec(`
				INSERT INTO personal_records (
					category, activity_id, distance_meters, duration_seconds,
					pace_per_mile, avg_heartrate, achieved_at, start_offset, end_offset
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(category) DO UPDATE SET
					activity_id = excluded.activity_id,
					distance_meters = excluded.distance_meters,
					duration_seconds = excluded.duration_seconds,
					pace_per_mile = excluded.pace_per_mile,
					avg_heartrate = excluded.avg_heartrate,
					achieved_at = excluded.achieved_at
				WHERE excluded.pace_per_mile < personal_records.pace_per_mile
			`, pacePR.Category, pacePR.ActivityID, pacePR.DistanceMeters, pacePR.DurationSeconds,
				pacePR.PacePerMile, pacePR.AvgHeartrate, pacePR.AchievedAt.Format(time.RFC3339), nil, nil,
			); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("saving fastest pace PR: %w", err))
			}
		}
	}
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
