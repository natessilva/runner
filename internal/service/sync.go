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
	store   *store.Store
	hrZones analysis.HRZones
}

// NewSyncService creates a new sync service with athlete config for HR calculations
func NewSyncService(client *strava.Client, store *store.Store, athleteCfg config.AthleteConfig) *SyncService {
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

// reportError sends an error to the progress channel if available
func reportError(progress chan<- SyncProgress, phase string, err error) {
	if progress != nil {
		progress <- SyncProgress{
			Phase: phase,
			Error: err,
		}
	}
}

// SyncResult contains the results of a sync operation
type SyncResult struct {
	ActivitiesFetched    int
	ActivitiesStored     int
	StreamsFetched       int
	MetricsComputed      int
	PRsComputed          int
	PredictionsComputed  int
	RunsWithHR           int
	Errors               []error
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

	// Phase 5: Compute race predictions
	if err := s.computeRacePredictions(ctx, progress, result); err != nil {
		return result, fmt.Errorf("computing predictions: %w", err)
	}

	return result, nil
}

// syncActivities fetches all activities from Strava and stores them
func (s *SyncService) syncActivities(ctx context.Context, progress chan<- SyncProgress, result *SyncResult) error {
	// Get last sync time
	lastSyncStr, _ := s.store.GetSyncState("last_activity_sync")
	var after time.Time
	if lastSyncStr != "" {
		var parseErr error
		after, parseErr = time.Parse(time.RFC3339, lastSyncStr)
		if parseErr != nil {
			// Corrupted sync state - log error and sync from beginning
			syncErr := fmt.Errorf("parsing last sync time %q, will sync from beginning: %w", lastSyncStr, parseErr)
			result.Errors = append(result.Errors, syncErr)
			reportError(progress, "activities", syncErr)
			after = time.Time{} // Reset to zero time
		}
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
					storeErr := fmt.Errorf("storing activity %d: %w", a.ID, err)
					result.Errors = append(result.Errors, storeErr)
					reportError(progress, "activities", storeErr)
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
			streamErr := fmt.Errorf("activity %d (%s): %w", activity.ID, activity.Name, err)
			result.Errors = append(result.Errors, streamErr)
			reportError(progress, "streams", streamErr)
			continue
		}

		// Convert and store streams
		points := convertStreams(activity.ID, streams)
		if len(points) > 0 {
			if err := s.store.SaveStreams(activity.ID, points); err != nil {
				saveErr := fmt.Errorf("saving streams for %d: %w", activity.ID, err)
				result.Errors = append(result.Errors, saveErr)
				reportError(progress, "streams", saveErr)
				continue
			}
		}

		// Mark activity as having streams synced
		if err := s.store.MarkStreamsSynced(activity.ID); err != nil {
			markErr := fmt.Errorf("marking synced for %d: %w", activity.ID, err)
			result.Errors = append(result.Errors, markErr)
			reportError(progress, "streams", markErr)
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
			getErr := fmt.Errorf("getting streams for %d: %w", activity.ID, err)
			result.Errors = append(result.Errors, getErr)
			reportError(progress, "metrics", getErr)
			continue
		}

		if len(streams) == 0 {
			continue
		}

		// Compute metrics
		metrics := analysis.ComputeActivityMetrics(activity, streams, zones)

		// Save metrics
		if err := s.store.SaveActivityMetrics(&metrics); err != nil {
			saveErr := fmt.Errorf("saving metrics for %d: %w", activity.ID, err)
			result.Errors = append(result.Errors, saveErr)
			reportError(progress, "metrics", saveErr)
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
				prErr := fmt.Errorf("saving distance PR for %d: %w", activity.ID, err)
				result.Errors = append(result.Errors, prErr)
				reportError(progress, "personal_records", prErr)
			} else if updated {
				result.PRsComputed++
			}
		}

		// Check other achievements: longest run, highest elevation, fastest avg pace
		s.checkOtherAchievements(&activity, result, progress)

		// Get streams for best effort analysis
		streams, err := s.store.GetStreams(activity.ID)
		if err != nil {
			getErr := fmt.Errorf("getting streams for PR analysis %d: %w", activity.ID, err)
			result.Errors = append(result.Errors, getErr)
			reportError(progress, "personal_records", getErr)
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
				effortErr := fmt.Errorf("saving effort PR for %d: %w", activity.ID, err)
				result.Errors = append(result.Errors, effortErr)
				reportError(progress, "personal_records", effortErr)
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
func (s *SyncService) checkOtherAchievements(activity *store.Activity, result *SyncResult, progress chan<- SyncProgress) {
	pacePerMile := analysis.CalculatePacePerMile(activity.Distance, activity.MovingTime)

	// Longest run - compare by distance
	s.upsertAchievement("longest_run", activity, activity.Distance, pacePerMile, store.CompareDistance, result, progress)

	// Highest elevation - compare by elevation (stored in distance field)
	s.upsertAchievement("highest_elevation", activity, activity.TotalElevationGain, pacePerMile, store.CompareDistance, result, progress)

	// Fastest pace - compare by pace (only for runs > 1 mile)
	if activity.Distance >= analysis.Distance1Mile {
		s.upsertAchievement("fastest_pace", activity, activity.Distance, pacePerMile, store.ComparePace, result, progress)
	}
}

// upsertAchievement creates or updates a PR using the specified comparison mode
func (s *SyncService) upsertAchievement(category string, activity *store.Activity, distance, pace float64, mode store.CompareMode, result *SyncResult, progress chan<- SyncProgress) {
	pr := &store.PersonalRecord{
		Category:        category,
		ActivityID:      activity.ID,
		DistanceMeters:  distance,
		DurationSeconds: activity.MovingTime,
		PacePerMile:     &pace,
		AvgHeartrate:    activity.AverageHeartrate,
		AchievedAt:      activity.StartDate,
	}
	if updated, err := s.store.UpsertPersonalRecordWithMode(pr, mode); err != nil {
		upsertErr := fmt.Errorf("saving %s PR: %w", category, err)
		result.Errors = append(result.Errors, upsertErr)
		reportError(progress, "personal_records", upsertErr)
	} else if updated {
		result.PRsComputed++
	}
}

// computeRacePredictions generates race time predictions based on PRs
func (s *SyncService) computeRacePredictions(ctx context.Context, progress chan<- SyncProgress, result *SyncResult) error {
	if progress != nil {
		progress <- SyncProgress{Phase: "predictions", Total: 1, Completed: 0}
	}

	// Get all personal records
	prs, err := s.store.GetAllPersonalRecords()
	if err != nil {
		return fmt.Errorf("getting personal records: %w", err)
	}

	if len(prs) == 0 {
		// No PRs yet, nothing to predict
		return nil
	}

	// Select the best source PR for predictions
	sourcePR := analysis.SelectBestSourcePR(prs)
	if sourcePR == nil {
		// No suitable PR found (all too old or wrong category)
		return nil
	}

	// Generate predictions
	predictions := analysis.GeneratePredictions(sourcePR, nil)
	if len(predictions) == 0 {
		return nil
	}

	// Clear old predictions and insert new ones
	if err := s.store.DeleteAllRacePredictions(); err != nil {
		return fmt.Errorf("clearing old predictions: %w", err)
	}

	now := ctx.Value("now")
	var computedAt time.Time
	if t, ok := now.(time.Time); ok {
		computedAt = t
	} else {
		computedAt = time.Now()
	}

	for _, pred := range predictions {
		storePred := &store.RacePrediction{
			TargetDistance:   pred.TargetName,
			TargetMeters:     pred.TargetMeters,
			PredictedSeconds: pred.PredictedSeconds,
			PredictedPace:    pred.PredictedPace,
			VDOT:             pred.VDOT,
			SourceCategory:   sourcePR.Category,
			SourceActivityID: sourcePR.ActivityID,
			Confidence:       pred.Confidence,
			ConfidenceScore:  pred.ConfidenceScore,
			ComputedAt:       computedAt,
		}

		if err := s.store.UpsertRacePrediction(storePred); err != nil {
			predErr := fmt.Errorf("saving prediction for %s: %w", pred.TargetName, err)
			result.Errors = append(result.Errors, predErr)
			reportError(progress, "predictions", predErr)
			continue
		}
		result.PredictionsComputed++
	}

	if progress != nil {
		progress <- SyncProgress{Phase: "predictions", Total: 1, Completed: 1}
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
