package store

//go:generate sqlc generate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"runner/internal/store/sqlc"
)

// Store wraps sqlc.Queries and provides the application's data access layer.
// It maintains backward compatibility with the existing DB methods.
type Store struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// newStore creates a Store from a database connection.
func newStore(db *sql.DB) *Store {
	return &Store{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced operations.
func (s *Store) DB() *sql.DB {
	return s.db
}

// --- Auth Methods ---

// GetAuth retrieves the stored authentication tokens.
func (s *Store) GetAuth() (*Auth, error) {
	row, err := s.queries.GetAuth(context.Background())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoAuth
	}
	if err != nil {
		return nil, err
	}
	return &Auth{
		AthleteID:    row.AthleteID,
		AccessToken:  row.AccessToken,
		RefreshToken: row.RefreshToken,
		ExpiresAt:    time.Unix(row.ExpiresAt, 0),
	}, nil
}

// SaveAuth stores or updates the authentication tokens.
func (s *Store) SaveAuth(auth *Auth) error {
	return s.queries.SaveAuth(context.Background(), sqlc.SaveAuthParams{
		AthleteID:    auth.AthleteID,
		AccessToken:  auth.AccessToken,
		RefreshToken: auth.RefreshToken,
		ExpiresAt:    auth.ExpiresAt.Unix(),
	})
}

// UpdateTokens updates just the access and refresh tokens.
func (s *Store) UpdateTokens(accessToken, refreshToken string, expiresAt time.Time) error {
	result, err := s.queries.UpdateTokens(context.Background(), sqlc.UpdateTokensParams{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt.Unix(),
	})
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoAuth
	}
	return nil
}

// --- Sync State Methods ---

// GetSyncState retrieves a sync state value by key.
// Returns empty string if key doesn't exist.
func (s *Store) GetSyncState(key string) (string, error) {
	value, err := s.queries.GetSyncState(context.Background(), key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// SetSyncState sets a sync state value.
func (s *Store) SetSyncState(key, value string) error {
	return s.queries.SetSyncState(context.Background(), sqlc.SetSyncStateParams{
		Key:   key,
		Value: value,
	})
}

// --- Activity Methods ---

// UpsertActivity inserts or updates an activity.
func (s *Store) UpsertActivity(a *Activity) error {
	return s.queries.UpsertActivity(context.Background(), sqlc.UpsertActivityParams{
		ID:                 a.ID,
		AthleteID:          a.AthleteID,
		Name:               a.Name,
		Type:               a.Type,
		StartDate:          a.StartDate.Format(time.RFC3339),
		StartDateLocal:     a.StartDateLocal.Format(time.RFC3339),
		Timezone:           toNullString(a.Timezone),
		Distance:           a.Distance,
		MovingTime:         int64(a.MovingTime),
		ElapsedTime:        int64(a.ElapsedTime),
		TotalElevationGain: toNullFloat64(a.TotalElevationGain),
		AverageSpeed:       toNullFloat64(a.AverageSpeed),
		MaxSpeed:           toNullFloat64(a.MaxSpeed),
		AverageHeartrate:   ptrToNullFloat64(a.AverageHeartrate),
		MaxHeartrate:       ptrToNullFloat64(a.MaxHeartrate),
		AverageCadence:     ptrToNullFloat64(a.AverageCadence),
		SufferScore:        ptrIntToNullInt64(a.SufferScore),
		HasHeartrate:       boolToInt64(a.HasHeartrate),
		StreamsSynced:      boolToInt64(a.StreamsSynced),
	})
}

// GetActivity retrieves an activity by ID.
func (s *Store) GetActivity(id int64) (*Activity, error) {
	row, err := s.queries.GetActivity(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrActivityNotFound
	}
	if err != nil {
		return nil, err
	}
	return activityRowToActivity(row)
}

// ListActivities returns activities ordered by start date descending.
func (s *Store) ListActivities(limit, offset int) ([]Activity, error) {
	rows, err := s.queries.ListActivities(context.Background(), sqlc.ListActivitiesParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, 0, len(rows))
	for _, row := range rows {
		a, err := listActivityRowToActivity(row)
		if err != nil {
			return nil, err
		}
		activities = append(activities, *a)
	}
	return activities, nil
}

// GetActivitiesNeedingStreams returns activities that haven't had their streams synced.
func (s *Store) GetActivitiesNeedingStreams(limit int) ([]Activity, error) {
	rows, err := s.queries.GetActivitiesNeedingStreams(context.Background(), int64(limit))
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, 0, len(rows))
	for _, row := range rows {
		a, err := needingStreamsRowToActivity(row)
		if err != nil {
			return nil, err
		}
		activities = append(activities, *a)
	}
	return activities, nil
}

// GetActivitiesNeedingMetrics returns activities that have streams but no computed metrics.
func (s *Store) GetActivitiesNeedingMetrics() ([]Activity, error) {
	rows, err := s.queries.GetActivitiesNeedingMetrics(context.Background())
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, 0, len(rows))
	for _, row := range rows {
		a, err := needingMetricsRowToActivity(row)
		if err != nil {
			return nil, err
		}
		activities = append(activities, *a)
	}
	return activities, nil
}

// MarkStreamsSynced marks an activity's streams as synced.
func (s *Store) MarkStreamsSynced(id int64) error {
	result, err := s.queries.MarkStreamsSynced(context.Background(), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrActivityNotFound
	}
	return nil
}

// CountActivities returns the total number of activities.
func (s *Store) CountActivities() (int, error) {
	count, err := s.queries.CountActivities(context.Background())
	return int(count), err
}

// --- Stream Methods ---

// GetStreams retrieves all stream points for an activity.
func (s *Store) GetStreams(activityID int64) ([]StreamPoint, error) {
	rows, err := s.queries.GetStreams(context.Background(), activityID)
	if err != nil {
		return nil, err
	}
	points := make([]StreamPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, streamToStreamPoint(row))
	}
	return points, nil
}

// GetStreamCount returns the number of stream points for an activity.
func (s *Store) GetStreamCount(activityID int64) (int, error) {
	count, err := s.queries.GetStreamCount(context.Background(), activityID)
	return int(count), err
}

// HasStreams checks if an activity has stream data.
func (s *Store) HasStreams(activityID int64) (bool, error) {
	_, err := s.queries.HasStreams(context.Background(), activityID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// DeleteStreams removes all stream data for an activity.
func (s *Store) DeleteStreams(activityID int64) error {
	return s.queries.DeleteStreams(context.Background(), activityID)
}

// --- Metrics Methods ---

// SaveActivityMetrics stores computed metrics for an activity.
func (s *Store) SaveActivityMetrics(m *ActivityMetrics) error {
	return s.queries.SaveActivityMetrics(context.Background(), sqlc.SaveActivityMetricsParams{
		ActivityID:        m.ActivityID,
		EfficiencyFactor:  ptrToNullFloat64(m.EfficiencyFactor),
		AerobicDecoupling: ptrToNullFloat64(m.AerobicDecoupling),
		CardiacDrift:      ptrToNullFloat64(m.CardiacDrift),
		PaceAtZ1:          ptrToNullFloat64(m.PaceAtZ1),
		PaceAtZ2:          ptrToNullFloat64(m.PaceAtZ2),
		PaceAtZ3:          ptrToNullFloat64(m.PaceAtZ3),
		Trimp:             ptrToNullFloat64(m.TRIMP),
		Hrss:              ptrToNullFloat64(m.HRSS),
		DataQualityScore:  ptrToNullFloat64(m.DataQualityScore),
		SteadyStatePct:    ptrToNullFloat64(m.SteadyStatePct),
	})
}

// GetActivityMetrics retrieves computed metrics for an activity.
func (s *Store) GetActivityMetrics(activityID int64) (*ActivityMetrics, error) {
	row, err := s.queries.GetActivityMetrics(context.Background(), activityID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ActivityMetrics{
		ActivityID:        row.ActivityID,
		EfficiencyFactor:  nullFloat64ToPtr(row.EfficiencyFactor),
		AerobicDecoupling: nullFloat64ToPtr(row.AerobicDecoupling),
		CardiacDrift:      nullFloat64ToPtr(row.CardiacDrift),
		PaceAtZ1:          nullFloat64ToPtr(row.PaceAtZ1),
		PaceAtZ2:          nullFloat64ToPtr(row.PaceAtZ2),
		PaceAtZ3:          nullFloat64ToPtr(row.PaceAtZ3),
		TRIMP:             nullFloat64ToPtr(row.Trimp),
		HRSS:              nullFloat64ToPtr(row.Hrss),
		DataQualityScore:  nullFloat64ToPtr(row.DataQualityScore),
		SteadyStatePct:    nullFloat64ToPtr(row.SteadyStatePct),
	}, nil
}

// HasMetrics checks if an activity has computed metrics.
func (s *Store) HasMetrics(activityID int64) (bool, error) {
	_, err := s.queries.HasMetrics(context.Background(), activityID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetAllMetrics retrieves metrics for all activities, ordered by date.
func (s *Store) GetAllMetrics() ([]ActivityMetrics, error) {
	rows, err := s.queries.GetAllMetrics(context.Background())
	if err != nil {
		return nil, err
	}
	metrics := make([]ActivityMetrics, 0, len(rows))
	for _, row := range rows {
		metrics = append(metrics, ActivityMetrics{
			ActivityID:        row.ActivityID,
			EfficiencyFactor:  nullFloat64ToPtr(row.EfficiencyFactor),
			AerobicDecoupling: nullFloat64ToPtr(row.AerobicDecoupling),
			CardiacDrift:      nullFloat64ToPtr(row.CardiacDrift),
			PaceAtZ1:          nullFloat64ToPtr(row.PaceAtZ1),
			PaceAtZ2:          nullFloat64ToPtr(row.PaceAtZ2),
			PaceAtZ3:          nullFloat64ToPtr(row.PaceAtZ3),
			TRIMP:             nullFloat64ToPtr(row.Trimp),
			HRSS:              nullFloat64ToPtr(row.Hrss),
			DataQualityScore:  nullFloat64ToPtr(row.DataQualityScore),
			SteadyStatePct:    nullFloat64ToPtr(row.SteadyStatePct),
		})
	}
	return metrics, nil
}

// CountMetrics returns the number of activities with computed metrics.
func (s *Store) CountMetrics() (int, error) {
	count, err := s.queries.CountMetrics(context.Background())
	return int(count), err
}

// GetActivitiesWithMetrics retrieves activities that have computed metrics.
func (s *Store) GetActivitiesWithMetrics(limit, offset int) ([]Activity, []ActivityMetrics, error) {
	rows, err := s.queries.GetActivitiesWithMetricsRaw(context.Background(), sqlc.GetActivitiesWithMetricsRawParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, nil, err
	}

	activities := make([]Activity, 0, len(rows))
	metrics := make([]ActivityMetrics, 0, len(rows))

	for _, row := range rows {
		startDate, err := time.Parse(time.RFC3339, row.StartDate)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing start_date %q: %w", row.StartDate, err)
		}
		startDateLocal, err := time.Parse(time.RFC3339, row.StartDateLocal)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing start_date_local %q: %w", row.StartDateLocal, err)
		}

		activities = append(activities, Activity{
			ID:                 row.ID,
			AthleteID:          row.AthleteID,
			Name:               row.Name,
			Type:               row.Type,
			StartDate:          startDate,
			StartDateLocal:     startDateLocal,
			Timezone:           row.Timezone.String,
			Distance:           row.Distance,
			MovingTime:         int(row.MovingTime),
			ElapsedTime:        int(row.ElapsedTime),
			TotalElevationGain: row.TotalElevationGain.Float64,
			AverageSpeed:       row.AverageSpeed.Float64,
			MaxSpeed:           row.MaxSpeed.Float64,
			AverageHeartrate:   nullFloat64ToPtr(row.AverageHeartrate),
			MaxHeartrate:       nullFloat64ToPtr(row.MaxHeartrate),
			AverageCadence:     nullFloat64ToPtr(row.AverageCadence),
			SufferScore:        nullInt64ToIntPtr(row.SufferScore),
			HasHeartrate:       row.HasHeartrate == 1,
			StreamsSynced:      row.StreamsSynced == 1,
		})

		metrics = append(metrics, ActivityMetrics{
			ActivityID:        row.ID,
			EfficiencyFactor:  nullFloat64ToPtr(row.EfficiencyFactor),
			AerobicDecoupling: nullFloat64ToPtr(row.AerobicDecoupling),
			CardiacDrift:      nullFloat64ToPtr(row.CardiacDrift),
			PaceAtZ1:          nullFloat64ToPtr(row.PaceAtZ1),
			PaceAtZ2:          nullFloat64ToPtr(row.PaceAtZ2),
			PaceAtZ3:          nullFloat64ToPtr(row.PaceAtZ3),
			TRIMP:             nullFloat64ToPtr(row.Trimp),
			HRSS:              nullFloat64ToPtr(row.Hrss),
			DataQualityScore:  nullFloat64ToPtr(row.DataQualityScore),
			SteadyStatePct:    nullFloat64ToPtr(row.SteadyStatePct),
		})
	}

	return activities, metrics, nil
}

// --- Personal Records Methods ---

// GetPersonalRecordByCategory retrieves a personal record by category.
func (s *Store) GetPersonalRecordByCategory(category string) (*PersonalRecord, error) {
	row, err := s.queries.GetPersonalRecordByCategory(context.Background(), category)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPersonalRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	return personalRecordRowToPersonalRecord(row)
}

// GetAllPersonalRecords retrieves all personal records.
func (s *Store) GetAllPersonalRecords() ([]PersonalRecord, error) {
	rows, err := s.queries.GetAllPersonalRecords(context.Background())
	if err != nil {
		return nil, err
	}
	records := make([]PersonalRecord, 0, len(rows))
	for _, row := range rows {
		pr, err := personalRecordRowToPersonalRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, *pr)
	}
	return records, nil
}

// GetPersonalRecordsForActivity retrieves all personal records achieved during a specific activity.
func (s *Store) GetPersonalRecordsForActivity(activityID int64) ([]PersonalRecord, error) {
	rows, err := s.queries.GetPersonalRecordsForActivity(context.Background(), activityID)
	if err != nil {
		return nil, err
	}
	records := make([]PersonalRecord, 0, len(rows))
	for _, row := range rows {
		pr, err := personalRecordRowToPersonalRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, *pr)
	}
	return records, nil
}

// DeletePersonalRecordsForActivity removes all PRs associated with an activity.
func (s *Store) DeletePersonalRecordsForActivity(activityID int64) error {
	return s.queries.DeletePersonalRecordsForActivity(context.Background(), activityID)
}

// UpsertPersonalRecord inserts or updates a personal record.
// Only updates if the new record is faster (lower duration for same distance category).
func (s *Store) UpsertPersonalRecord(pr *PersonalRecord) (updated bool, err error) {
	return s.UpsertPersonalRecordWithMode(pr, CompareDuration)
}

// UpsertPersonalRecordWithMode inserts or updates a personal record with the specified comparison mode.
func (s *Store) UpsertPersonalRecordWithMode(pr *PersonalRecord, mode CompareMode) (updated bool, err error) {
	existing, err := s.GetPersonalRecordByCategory(pr.Category)
	if err != nil && !errors.Is(err, ErrPersonalRecordNotFound) {
		return false, err
	}

	if existing != nil {
		switch mode {
		case CompareDuration:
			if existing.DurationSeconds <= pr.DurationSeconds {
				return false, nil
			}
		case CompareDistance:
			if existing.DistanceMeters >= pr.DistanceMeters {
				return false, nil
			}
		case ComparePace:
			if existing.PacePerMile != nil && pr.PacePerMile != nil && *existing.PacePerMile <= *pr.PacePerMile {
				return false, nil
			}
		}
	}

	err = s.queries.InsertPersonalRecord(context.Background(), sqlc.InsertPersonalRecordParams{
		Category:        pr.Category,
		ActivityID:      pr.ActivityID,
		DistanceMeters:  pr.DistanceMeters,
		DurationSeconds: int64(pr.DurationSeconds),
		PacePerMile:     ptrToNullFloat64(pr.PacePerMile),
		AvgHeartrate:    ptrToNullFloat64(pr.AvgHeartrate),
		AchievedAt:      pr.AchievedAt.Format(time.RFC3339),
		StartOffset:     ptrIntToNullInt64(pr.StartOffset),
		EndOffset:       ptrIntToNullInt64(pr.EndOffset),
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetPreviousRecord retrieves the previous record for a category before a given activity.
func (s *Store) GetPreviousRecord(category string, currentActivityID int64) (*PersonalRecord, error) {
	// Since we only keep the best record, we can't get the previous one.
	return nil, nil
}

// --- Race Predictions Methods ---

// UpsertRacePrediction inserts or updates a race prediction.
func (s *Store) UpsertRacePrediction(p *RacePrediction) error {
	return s.queries.UpsertRacePrediction(context.Background(), sqlc.UpsertRacePredictionParams{
		TargetDistance:   p.TargetDistance,
		TargetMeters:     p.TargetMeters,
		PredictedSeconds: int64(p.PredictedSeconds),
		PredictedPace:    p.PredictedPace,
		Vdot:             p.VDOT,
		SourceCategory:   p.SourceCategory,
		SourceActivityID: p.SourceActivityID,
		Confidence:       p.Confidence,
		ConfidenceScore:  p.ConfidenceScore,
		ComputedAt:       p.ComputedAt.Format(time.RFC3339),
	})
}

// GetAllRacePredictions retrieves all race predictions ordered by distance.
func (s *Store) GetAllRacePredictions() ([]RacePrediction, error) {
	rows, err := s.queries.GetAllRacePredictions(context.Background())
	if err != nil {
		return nil, err
	}
	predictions := make([]RacePrediction, 0, len(rows))
	for _, row := range rows {
		computedAt, err := time.Parse(time.RFC3339, row.ComputedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing computed_at %q: %w", row.ComputedAt, err)
		}
		predictions = append(predictions, RacePrediction{
			ID:               row.ID,
			TargetDistance:   row.TargetDistance,
			TargetMeters:     row.TargetMeters,
			PredictedSeconds: int(row.PredictedSeconds),
			PredictedPace:    row.PredictedPace,
			VDOT:             row.Vdot,
			SourceCategory:   row.SourceCategory,
			SourceActivityID: row.SourceActivityID,
			Confidence:       row.Confidence,
			ConfidenceScore:  row.ConfidenceScore,
			ComputedAt:       computedAt,
		})
	}
	return predictions, nil
}

// GetRacePrediction retrieves a single prediction by target distance.
func (s *Store) GetRacePrediction(targetDistance string) (*RacePrediction, error) {
	row, err := s.queries.GetRacePrediction(context.Background(), targetDistance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPredictionNotFound
	}
	if err != nil {
		return nil, err
	}
	computedAt, err := time.Parse(time.RFC3339, row.ComputedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing computed_at %q: %w", row.ComputedAt, err)
	}
	return &RacePrediction{
		ID:               row.ID,
		TargetDistance:   row.TargetDistance,
		TargetMeters:     row.TargetMeters,
		PredictedSeconds: int(row.PredictedSeconds),
		PredictedPace:    row.PredictedPace,
		VDOT:             row.Vdot,
		SourceCategory:   row.SourceCategory,
		SourceActivityID: row.SourceActivityID,
		Confidence:       row.Confidence,
		ConfidenceScore:  row.ConfidenceScore,
		ComputedAt:       computedAt,
	}, nil
}

// DeleteAllRacePredictions removes all predictions.
func (s *Store) DeleteAllRacePredictions() error {
	return s.queries.DeleteAllRacePredictions(context.Background())
}

// --- Conversion Helpers ---

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func toNullFloat64(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: true}
}

func ptrToNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func ptrIntToNullInt64(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func nullFloat64ToPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	return &n.Float64
}

func nullInt64ToIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// activityRowToActivity converts a GetActivityRow to an Activity.
func activityRowToActivity(row sqlc.GetActivityRow) (*Activity, error) {
	startDate, err := time.Parse(time.RFC3339, row.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date %q: %w", row.StartDate, err)
	}
	startDateLocal, err := time.Parse(time.RFC3339, row.StartDateLocal)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date_local %q: %w", row.StartDateLocal, err)
	}
	return &Activity{
		ID:                 row.ID,
		AthleteID:          row.AthleteID,
		Name:               row.Name,
		Type:               row.Type,
		StartDate:          startDate,
		StartDateLocal:     startDateLocal,
		Timezone:           row.Timezone.String,
		Distance:           row.Distance,
		MovingTime:         int(row.MovingTime),
		ElapsedTime:        int(row.ElapsedTime),
		TotalElevationGain: row.TotalElevationGain.Float64,
		AverageSpeed:       row.AverageSpeed.Float64,
		MaxSpeed:           row.MaxSpeed.Float64,
		AverageHeartrate:   nullFloat64ToPtr(row.AverageHeartrate),
		MaxHeartrate:       nullFloat64ToPtr(row.MaxHeartrate),
		AverageCadence:     nullFloat64ToPtr(row.AverageCadence),
		SufferScore:        nullInt64ToIntPtr(row.SufferScore),
		HasHeartrate:       row.HasHeartrate == 1,
		StreamsSynced:      row.StreamsSynced == 1,
	}, nil
}

func listActivityRowToActivity(row sqlc.ListActivitiesRow) (*Activity, error) {
	startDate, err := time.Parse(time.RFC3339, row.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date %q: %w", row.StartDate, err)
	}
	startDateLocal, err := time.Parse(time.RFC3339, row.StartDateLocal)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date_local %q: %w", row.StartDateLocal, err)
	}
	return &Activity{
		ID:                 row.ID,
		AthleteID:          row.AthleteID,
		Name:               row.Name,
		Type:               row.Type,
		StartDate:          startDate,
		StartDateLocal:     startDateLocal,
		Timezone:           row.Timezone.String,
		Distance:           row.Distance,
		MovingTime:         int(row.MovingTime),
		ElapsedTime:        int(row.ElapsedTime),
		TotalElevationGain: row.TotalElevationGain.Float64,
		AverageSpeed:       row.AverageSpeed.Float64,
		MaxSpeed:           row.MaxSpeed.Float64,
		AverageHeartrate:   nullFloat64ToPtr(row.AverageHeartrate),
		MaxHeartrate:       nullFloat64ToPtr(row.MaxHeartrate),
		AverageCadence:     nullFloat64ToPtr(row.AverageCadence),
		SufferScore:        nullInt64ToIntPtr(row.SufferScore),
		HasHeartrate:       row.HasHeartrate == 1,
		StreamsSynced:      row.StreamsSynced == 1,
	}, nil
}

func needingStreamsRowToActivity(row sqlc.GetActivitiesNeedingStreamsRow) (*Activity, error) {
	startDate, err := time.Parse(time.RFC3339, row.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date %q: %w", row.StartDate, err)
	}
	startDateLocal, err := time.Parse(time.RFC3339, row.StartDateLocal)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date_local %q: %w", row.StartDateLocal, err)
	}
	return &Activity{
		ID:                 row.ID,
		AthleteID:          row.AthleteID,
		Name:               row.Name,
		Type:               row.Type,
		StartDate:          startDate,
		StartDateLocal:     startDateLocal,
		Timezone:           row.Timezone.String,
		Distance:           row.Distance,
		MovingTime:         int(row.MovingTime),
		ElapsedTime:        int(row.ElapsedTime),
		TotalElevationGain: row.TotalElevationGain.Float64,
		AverageSpeed:       row.AverageSpeed.Float64,
		MaxSpeed:           row.MaxSpeed.Float64,
		AverageHeartrate:   nullFloat64ToPtr(row.AverageHeartrate),
		MaxHeartrate:       nullFloat64ToPtr(row.MaxHeartrate),
		AverageCadence:     nullFloat64ToPtr(row.AverageCadence),
		SufferScore:        nullInt64ToIntPtr(row.SufferScore),
		HasHeartrate:       row.HasHeartrate == 1,
		StreamsSynced:      row.StreamsSynced == 1,
	}, nil
}

func needingMetricsRowToActivity(row sqlc.GetActivitiesNeedingMetricsRow) (*Activity, error) {
	startDate, err := time.Parse(time.RFC3339, row.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date %q: %w", row.StartDate, err)
	}
	startDateLocal, err := time.Parse(time.RFC3339, row.StartDateLocal)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date_local %q: %w", row.StartDateLocal, err)
	}
	return &Activity{
		ID:                 row.ID,
		AthleteID:          row.AthleteID,
		Name:               row.Name,
		Type:               row.Type,
		StartDate:          startDate,
		StartDateLocal:     startDateLocal,
		Timezone:           row.Timezone.String,
		Distance:           row.Distance,
		MovingTime:         int(row.MovingTime),
		ElapsedTime:        int(row.ElapsedTime),
		TotalElevationGain: row.TotalElevationGain.Float64,
		AverageSpeed:       row.AverageSpeed.Float64,
		MaxSpeed:           row.MaxSpeed.Float64,
		AverageHeartrate:   nullFloat64ToPtr(row.AverageHeartrate),
		MaxHeartrate:       nullFloat64ToPtr(row.MaxHeartrate),
		AverageCadence:     nullFloat64ToPtr(row.AverageCadence),
		SufferScore:        nullInt64ToIntPtr(row.SufferScore),
		HasHeartrate:       row.HasHeartrate == 1,
		StreamsSynced:      row.StreamsSynced == 1,
	}, nil
}

func streamToStreamPoint(row sqlc.Stream) StreamPoint {
	return StreamPoint{
		ActivityID:     row.ActivityID,
		TimeOffset:     int(row.TimeOffset),
		Lat:            nullFloat64ToPtr(row.LatlngLat),
		Lng:            nullFloat64ToPtr(row.LatlngLng),
		Altitude:       nullFloat64ToPtr(row.Altitude),
		VelocitySmooth: nullFloat64ToPtr(row.VelocitySmooth),
		Heartrate:      nullInt64ToIntPtr(row.Heartrate),
		Cadence:        nullInt64ToIntPtr(row.Cadence),
		GradeSmooth:    nullFloat64ToPtr(row.GradeSmooth),
		Distance:       nullFloat64ToPtr(row.Distance),
	}
}

func personalRecordRowToPersonalRecord(row sqlc.PersonalRecord) (*PersonalRecord, error) {
	achievedAt, err := time.Parse(time.RFC3339, row.AchievedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing achieved_at %q: %w", row.AchievedAt, err)
	}
	return &PersonalRecord{
		ID:              row.ID,
		Category:        row.Category,
		ActivityID:      row.ActivityID,
		DistanceMeters:  row.DistanceMeters,
		DurationSeconds: int(row.DurationSeconds),
		PacePerMile:     nullFloat64ToPtr(row.PacePerMile),
		AvgHeartrate:    nullFloat64ToPtr(row.AvgHeartrate),
		AchievedAt:      achievedAt,
		StartOffset:     nullInt64ToIntPtr(row.StartOffset),
		EndOffset:       nullInt64ToIntPtr(row.EndOffset),
	}, nil
}
