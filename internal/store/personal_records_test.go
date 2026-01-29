package store

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	db := &DB{sqlDB}

	// Run migrations
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Insert a test activity for foreign key constraints
	_, err = sqlDB.Exec(`
		INSERT INTO activities (id, athlete_id, name, type, start_date, start_date_local,
			distance, moving_time, elapsed_time, has_heartrate, streams_synced)
		VALUES (1, 123, 'Test Run', 'Run', '2024-01-15T10:00:00Z', '2024-01-15T10:00:00Z',
			5000, 1500, 1600, 1, 1)
	`)
	if err != nil {
		sqlDB.Close()
		t.Fatalf("Failed to insert test activity: %v", err)
	}

	// Insert a second test activity
	_, err = sqlDB.Exec(`
		INSERT INTO activities (id, athlete_id, name, type, start_date, start_date_local,
			distance, moving_time, elapsed_time, has_heartrate, streams_synced)
		VALUES (2, 123, 'Another Run', 'Run', '2024-01-20T10:00:00Z', '2024-01-20T10:00:00Z',
			10000, 3000, 3100, 1, 1)
	`)
	if err != nil {
		sqlDB.Close()
		t.Fatalf("Failed to insert second test activity: %v", err)
	}

	t.Cleanup(func() {
		sqlDB.Close()
	})

	return db
}

func TestUpsertPersonalRecord_CreateNew(t *testing.T) {
	db := setupTestDB(t)

	pacePerMile := 360.0
	avgHR := 155.0
	pr := &PersonalRecord{
		Category:        "distance_5k",
		ActivityID:      1,
		DistanceMeters:  5000,
		DurationSeconds: 1500,
		PacePerMile:     &pacePerMile,
		AvgHeartrate:    &avgHR,
		AchievedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	updated, err := db.UpsertPersonalRecord(pr)
	if err != nil {
		t.Fatalf("UpsertPersonalRecord failed: %v", err)
	}

	if !updated {
		t.Error("Expected updated=true for new record")
	}

	// Verify the record was created
	fetched, err := db.GetPersonalRecordByCategory("distance_5k")
	if err != nil {
		t.Fatalf("GetPersonalRecordByCategory failed: %v", err)
	}

	if fetched.DurationSeconds != 1500 {
		t.Errorf("Expected duration 1500, got %d", fetched.DurationSeconds)
	}
}

func TestUpsertPersonalRecord_UpdateOnlyIfFaster(t *testing.T) {
	db := setupTestDB(t)

	// Insert initial record
	pr1 := &PersonalRecord{
		Category:        "distance_5k",
		ActivityID:      1,
		DistanceMeters:  5000,
		DurationSeconds: 1500, // 25:00
		AchievedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	db.UpsertPersonalRecord(pr1)

	// Try to update with slower time
	pr2 := &PersonalRecord{
		Category:        "distance_5k",
		ActivityID:      2,
		DistanceMeters:  5000,
		DurationSeconds: 1600, // 26:40 - slower
		AchievedAt:      time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
	}

	updated, err := db.UpsertPersonalRecord(pr2)
	if err != nil {
		t.Fatalf("UpsertPersonalRecord failed: %v", err)
	}

	if updated {
		t.Error("Expected updated=false for slower time")
	}

	// Verify original record is still there
	fetched, _ := db.GetPersonalRecordByCategory("distance_5k")
	if fetched.DurationSeconds != 1500 {
		t.Errorf("Expected original duration 1500, got %d", fetched.DurationSeconds)
	}
	if fetched.ActivityID != 1 {
		t.Errorf("Expected original activity ID 1, got %d", fetched.ActivityID)
	}
}

func TestUpsertPersonalRecord_UpdateWhenFaster(t *testing.T) {
	db := setupTestDB(t)

	// Insert initial record
	pr1 := &PersonalRecord{
		Category:        "distance_5k",
		ActivityID:      1,
		DistanceMeters:  5000,
		DurationSeconds: 1500, // 25:00
		AchievedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	db.UpsertPersonalRecord(pr1)

	// Update with faster time
	pr2 := &PersonalRecord{
		Category:        "distance_5k",
		ActivityID:      2,
		DistanceMeters:  5000,
		DurationSeconds: 1400, // 23:20 - faster
		AchievedAt:      time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
	}

	updated, err := db.UpsertPersonalRecord(pr2)
	if err != nil {
		t.Fatalf("UpsertPersonalRecord failed: %v", err)
	}

	if !updated {
		t.Error("Expected updated=true for faster time")
	}

	// Verify new record replaced the old one
	fetched, _ := db.GetPersonalRecordByCategory("distance_5k")
	if fetched.DurationSeconds != 1400 {
		t.Errorf("Expected new duration 1400, got %d", fetched.DurationSeconds)
	}
	if fetched.ActivityID != 2 {
		t.Errorf("Expected new activity ID 2, got %d", fetched.ActivityID)
	}
}

func TestGetAllPersonalRecords(t *testing.T) {
	db := setupTestDB(t)

	// Insert multiple records
	records := []*PersonalRecord{
		{Category: "distance_5k", ActivityID: 1, DistanceMeters: 5000, DurationSeconds: 1500, AchievedAt: time.Now()},
		{Category: "effort_1mi", ActivityID: 1, DistanceMeters: 1609, DurationSeconds: 360, AchievedAt: time.Now()},
		{Category: "longest_run", ActivityID: 2, DistanceMeters: 10000, DurationSeconds: 3600, AchievedAt: time.Now()},
	}

	for _, pr := range records {
		db.UpsertPersonalRecord(pr)
	}

	all, err := db.GetAllPersonalRecords()
	if err != nil {
		t.Fatalf("GetAllPersonalRecords failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("Expected 3 records, got %d", len(all))
	}
}

func TestGetPersonalRecordsForActivity(t *testing.T) {
	db := setupTestDB(t)

	// Insert records for different activities
	db.UpsertPersonalRecord(&PersonalRecord{
		Category: "distance_5k", ActivityID: 1, DistanceMeters: 5000, DurationSeconds: 1500, AchievedAt: time.Now(),
	})
	db.UpsertPersonalRecord(&PersonalRecord{
		Category: "effort_1mi", ActivityID: 1, DistanceMeters: 1609, DurationSeconds: 360, AchievedAt: time.Now(),
	})
	db.UpsertPersonalRecord(&PersonalRecord{
		Category: "longest_run", ActivityID: 2, DistanceMeters: 10000, DurationSeconds: 3600, AchievedAt: time.Now(),
	})

	// Get records for activity 1
	records, err := db.GetPersonalRecordsForActivity(1)
	if err != nil {
		t.Fatalf("GetPersonalRecordsForActivity failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records for activity 1, got %d", len(records))
	}

	// Get records for activity 2
	records, err = db.GetPersonalRecordsForActivity(2)
	if err != nil {
		t.Fatalf("GetPersonalRecordsForActivity failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record for activity 2, got %d", len(records))
	}
}

func TestGetPersonalRecordByCategory_NotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.GetPersonalRecordByCategory("nonexistent")
	if err != ErrPersonalRecordNotFound {
		t.Errorf("Expected ErrPersonalRecordNotFound, got %v", err)
	}
}

func TestDeletePersonalRecordsForActivity(t *testing.T) {
	db := setupTestDB(t)

	// Insert records
	db.UpsertPersonalRecord(&PersonalRecord{
		Category: "distance_5k", ActivityID: 1, DistanceMeters: 5000, DurationSeconds: 1500, AchievedAt: time.Now(),
	})
	db.UpsertPersonalRecord(&PersonalRecord{
		Category: "effort_1mi", ActivityID: 1, DistanceMeters: 1609, DurationSeconds: 360, AchievedAt: time.Now(),
	})
	db.UpsertPersonalRecord(&PersonalRecord{
		Category: "longest_run", ActivityID: 2, DistanceMeters: 10000, DurationSeconds: 3600, AchievedAt: time.Now(),
	})

	// Delete records for activity 1
	err := db.DeletePersonalRecordsForActivity(1)
	if err != nil {
		t.Fatalf("DeletePersonalRecordsForActivity failed: %v", err)
	}

	// Verify only activity 2's record remains
	all, _ := db.GetAllPersonalRecords()
	if len(all) != 1 {
		t.Errorf("Expected 1 record remaining, got %d", len(all))
	}
	if all[0].ActivityID != 2 {
		t.Errorf("Expected remaining record to be for activity 2")
	}
}

func TestPersonalRecord_WithOffsets(t *testing.T) {
	db := setupTestDB(t)

	startOffset := 60
	endOffset := 120
	pr := &PersonalRecord{
		Category:        "effort_400m",
		ActivityID:      1,
		DistanceMeters:  400,
		DurationSeconds: 60,
		AchievedAt:      time.Now(),
		StartOffset:     &startOffset,
		EndOffset:       &endOffset,
	}

	db.UpsertPersonalRecord(pr)

	fetched, err := db.GetPersonalRecordByCategory("effort_400m")
	if err != nil {
		t.Fatalf("GetPersonalRecordByCategory failed: %v", err)
	}

	if fetched.StartOffset == nil || *fetched.StartOffset != 60 {
		t.Error("StartOffset not saved correctly")
	}
	if fetched.EndOffset == nil || *fetched.EndOffset != 120 {
		t.Error("EndOffset not saved correctly")
	}
}
